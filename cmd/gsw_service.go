package main

import (
	"github.com/spf13/viper"
	"context"
	"fmt"
	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"os"
	"os/signal"
	"syscall"
	"errors"

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

func vcmInitialize() (*ipc.IpcShmHandler, error) {
	if !viper.IsSet("telemetry_config") {
		err := errors.New("Error: Telemetry config filepath is not set in GSW config.")
		fmt.Printf("%v\n", err)
		return nil, err
	}
	data, err := os.ReadFile(viper.GetString("telemetry_config"))
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		return nil, err
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return nil, err
	}
	configWriter, err := ipc.CreateIpcShmHandler("config", len(data), true)
	if err != nil {
		fmt.Printf("Error creating shared memory handler: %v\n", err)
		return nil, err
	}
	if configWriter.Write(data) != nil {
		fmt.Printf("Error writing config to shared memory: %v\n", err)
		configWriter.Cleanup()
		return nil, err
	}

	printTelemetryPackets()
	return configWriter, nil
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

func dbInitialize(ctx context.Context, channelMap map[int]chan []byte) error {
	dbHandler := db.InfluxDBV1Handler{}
	err := dbHandler.Initialize()
	if err != nil {
		fmt.Println("Warning. Telemetry packets will not be published to database")
		return err
	}

	for _, packet := range proc.GswConfig.TelemetryPackets {
		go func(dbHandler db.Handler, packet tlm.TelemetryPacket, ch chan []byte) {
			proc.DatabaseWriter(dbHandler, packet, ch)
			<-ctx.Done()
			close(ch)
		}(&dbHandler, packet, channelMap[packet.Port])
	}

	return nil
}

func readConfig() {
	viper.SetConfigName("gsw_service")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("data/config/")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Error reading gsw_config: %w", err))
	}
}

func main() {
	// Read gsw_service config
	readConfig()


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

	configWriter, err := vcmInitialize()
	if err != nil {
		fmt.Println("Exiting GSW")
		return
	}
	defer configWriter.Cleanup()

	channelMap := decomInitialize(ctx)
	dbInitialize(ctx, channelMap)

	// Wait for context cancellation or signal handling
	<-ctx.Done()
	fmt.Println("Shutting down...")
}
