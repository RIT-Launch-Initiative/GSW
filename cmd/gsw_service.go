package main

import (
	"context"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"os"
	"os/signal"
	"syscall"

	"github.com/AarC10/GSW-V2/proc"
)

func printTelemetryPackets() {
	fmt.Println("Telemetry Packets:")
	for _, packet := range proc.GswConfig.TelemetryPackets {
		fmt.Printf("\tName: %s\n\tPort: %d\n", packet.Name, packet.Port)
		if len(packet.Measurements) > 0 {
			fmt.Println("\tMeasurements:")
			for _, measurementName := range packet.Measurements {
				measurement, ok := proc.GswConfig.Measurements[measurementName]
				if !ok {
					fmt.Printf("\t\tMeasurement '%s' not found\n", measurementName)
					continue
				}
				fmt.Printf("\t\t%s\n", measurement.String())
			}
		} else {
			fmt.Println("\t\tNo measurements defined.")
		}
	}
}

func vcmInitialize() error {
	// TODO: Need to set up configuration stuff
	_, err := proc.ParseConfig("data/config/backplane.yaml")
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return err
	}

	printTelemetryPackets()
	return nil
}

func decomInitialize(ctx context.Context) map[int]chan []byte {
	channelMap := make(map[int]chan []byte)

	for _, packet := range proc.GswConfig.TelemetryPackets {
		finalOutputChannel := make(chan []byte)
		channelMap[packet.Port] = finalOutputChannel

		go func(packet tlm.TelemetryPacket, ch chan []byte) {
			proc.TelemetryPacketWriter(packet, finalOutputChannel)
			<-ctx.Done()
			close(ch)
		}(packet, finalOutputChannel)
	}

	return channelMap
}

func dbInitialize(ctx context.Context, channelMap map[int]chan []byte) {
	dbHandler := db.InfluxDBV1Handler{}
	err := dbHandler.Initialize()
	if err != nil {
		fmt.Println("Warning. Telemetry packets will not be published to database")
		return
	}

	for _, packet := range proc.GswConfig.TelemetryPackets {
		go func(dbHandler db.Handler, packet tlm.TelemetryPacket, ch chan []byte) {
			proc.DatabaseWriter(dbHandler, packet, ch)
			<-ctx.Done()
			close(ch)
		}(&dbHandler, packet, channelMap[packet.Port])
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigs
		fmt.Printf("Received signal: %s\n", sig)
		cancel()
	}()

	if vcmInitialize() != nil {
		fmt.Println("Exiting GSW")
	}

	decomInitialize(ctx)
	//dbInitialize(ctx, channelMap)

	// Wait for context cancellation or signal handling
	<-ctx.Done()
	fmt.Println("Shutting down...")
}
