package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	configData, err := proc.ReadTelemetryConfigFromShm("/dev/shm")
	if err != nil {
		log.Fatal(err)
	}
	_, err = proc.ParseConfigBytes(configData)
	if err != nil {
		log.Fatal(err)
	}

	opts := MQTT.NewClientOptions()
	opts.AddBroker("tcp://127.0.0.1:1883")
	opts.SetClientID("gsw-mqtt-app")
	opts.SetCleanSession(true)

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal(token.Error())
	}
	packet := proc.GswConfig.TelemetryPackets[0]
	log.Println("Starting streaming for packet " + packet.Name)

	reader, err := proc.NewIpcShmReaderForPacket(packet, "/dev/shm")
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Cleanup()

	for {
		p, err := reader.Read()
		if err != nil {
			fmt.Printf("error reading packet: %v\n", err)
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
				val = "err"
			}
			jsonStr, err := json.Marshal(val)
			if err != nil {
				log.Fatal(err)
			}
			token := client.Publish(fmt.Sprintf("gsw/%s/%s", packet.Name, name), byte(0), false, jsonStr)
			token.Wait()
			offset += meas.Size
		}
	}
}
