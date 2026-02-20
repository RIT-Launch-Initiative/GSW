// NOTE(mia): Currently this is a minimal best-effort bridge that will not
// scale with higher message rates, yet. Measurements that either the MQTT
// broker or this bridge can't process will be dropped.
// refer to https://github.com/RIT-Launch-Initiative/GSW/issues/59

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

var (
	shmDir      = flag.String("shm", "/dev/shm", "directory to use for shared memory")
	brokerURL   = flag.String("broker", "tcp://127.0.0.1:1883", "mqtt broker url")
	topicPrefix = flag.String("topic_prefix", "gsw", "mqtt topic prefix")
)

func main() {
	flag.Parse()

	configData, err := proc.ReadTelemetryConfigFromShm(*shmDir)
	if err != nil {
		logger.Fatal("error reading telemetry config from shm", zap.Error(err))
	}
	_, err = proc.ParseConfigBytes(configData)
	if err != nil {
		logger.Fatal("error parsing telemetry config from gsw", zap.Error(err))
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(*brokerURL)
	opts.SetClientID("gsw-mqtt-app")
	opts.SetCleanSession(true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Fatal("error connecting to mqtt and creating token", zap.Error(token.Error()))
	}

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	for _, packet := range proc.GswConfig.TelemetryPackets {
		wg.Add(1)
		go func(packet tlm.TelemetryPacket) {
			defer wg.Done()
			err := packetWriter(ctx, packet, client)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("error in writer", zap.Error(err))
			}
		}(packet)
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("shutting down")
		cancel()
	}()

	wg.Wait()
	client.Disconnect(250)
}

func packetWriter(ctx context.Context, packet tlm.TelemetryPacket, client mqtt.Client) error {
	pLog := logger.Log().With(zap.String("packet", packet.Name))
	pLog.Info("starting streaming")

	reader, err := proc.NewIpcShmReaderForPacket(packet, *shmDir)
	if err != nil {
		return fmt.Errorf("couldn't create reader: %w", err)
	}
	defer reader.Cleanup()

	for {
		p, err := reader.Read(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			pLog.Error("error reading packet", zap.Error(err))
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
				pLog.Error("error interpreting measurement", zap.Error(err))
				continue
			}
			jsonStr, err := json.Marshal(val)
			if err != nil {
				pLog.Error("error marshaling measurement", zap.Error(err))
				continue
			}
			// qos=0 delivery not guaranteed
			client.Publish(fmt.Sprintf("%s/%s/%s", *topicPrefix, packet.Name, name), 0, false, jsonStr)
			offset += meas.Size
		}
	}
}
