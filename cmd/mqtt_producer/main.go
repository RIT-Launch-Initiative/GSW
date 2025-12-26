package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	shmDir      = flag.String("shm", "/dev/shm", "directory to use for shared memory")
	brokerUrl   = flag.String("broker", "tcp://127.0.0.1:1883", "mqtt broker url")
	topicPrefix = flag.String("topic_prefix", "gsw", "mqtt topic prefix")
)

func main() {
	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		log.Fatal(err)
	}
	_, err = proc.ParseConfigBytes(configData)
	if err != nil {
		log.Fatal(err)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*brokerUrl)
	opts.SetClientID("gsw-mqtt-app")
	opts.SetCleanSession(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	for _, packet := range proc.GswConfig.TelemetryPackets {
		wg.Add(1)
		go func(packet tlm.TelemetryPacket) {
			defer wg.Done()
			err := packetWriter(ctx, packet, client)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("error in writer: %v", err)
			}
		}(packet)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("shutting down")
		cancel()
	}()

	wg.Wait()
}

func packetWriter(ctx context.Context, packet tlm.TelemetryPacket, client mqtt.Client) error {
	pLog := log.New(os.Stderr, fmt.Sprintf("[%s] ", packet.Name), log.LstdFlags|log.Lmsgprefix)
	pLog.Println("starting streaming")

	reader, err := proc.NewIpcShmReaderForPacket(packet, *shmDir)
	if err != nil {
		pLog.Fatal(fmt.Errorf("couldn't create reader: %w", err))
	}
	defer reader.Cleanup()

	for {
		p, err := reader.Read(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			pLog.Printf("error reading packet: %v\n", err)
			continue
		}
		data := p.Data()
		offset := 0
		for _, name := range packet.Measurements {
			meas, ok := proc.GswConfig.Measurements[name]
			if !ok || offset+meas.Size > len(data) {
				continue
			}
			val, err := tlm.InterpretMeasurementValue(meas, data[offset:offset+meas.Size])
			if err != nil {
				pLog.Printf("error interpreting measurement: %v\n", err)
				continue
			}
			jsonStr, err := json.Marshal(val)
			if err != nil {
				pLog.Printf("error marshaling measurement: %v\n", err)
				continue
			}
			client.Publish(fmt.Sprintf("%s/%s/%s", *topicPrefix, packet.Name, name), byte(0), false, jsonStr)
			offset += meas.Size
		}
	}
}
