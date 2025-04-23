package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"net/http"
	_ "net/http/pprof"
)

// printTelemetryPackets prints the telemetry packets and their measurements it found in the configuration.
func printTelemetryPackets() {
	fmt.Println("Telemetry Packets:")
	for _, packet := range proc.GswConfig.TelemetryPackets {
		fmt.Printf("\tName: %s\n\tPort: %d\n", packet.Name, packet.Port)
		if len(packet.Measurements) > 0 {
			fmt.Println("\tMeasurements:")
			for _, measurementName := range packet.Measurements {
				measurement, ok := proc.GswConfig.Measurements[measurementName]
				if !ok {
					logger.Warn(fmt.Sprint("Measurement '", measurementName, "' not found"))
					continue
				}
				fmt.Printf("\t\t%s\n", measurement.String())
			}
		} else {
			logger.Warn("No measurement defined.")
		}
	}
}

// vcmInitialize initializes the Vehicle Config Manager
// It reads the telemetry config file and writes it into shared memory
func vcmInitialize(config *viper.Viper) (*ipc.ShmHandler, error) {
	if !config.IsSet("telemetry_config") {
		err := errors.New("telemetry config filepath is not set in GSW config")
		logger.Error(fmt.Sprint(err))
		return nil, err
	}
	data, err := os.ReadFile(config.GetString("telemetry_config"))
	if err != nil {
		logger.Error("Error reading YAML file: ", zap.Error(err))
		return nil, err
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {

		logger.Error("Error parsing YAML:", zap.Error(err))
		return nil, err
	}
	configWriter, err := ipc.CreateShmHandler("telemetry-config", len(data), true)
	if err != nil {
		logger.Error("Error creating shared memory handler: ", zap.Error(err))
		return nil, err
	}
	if configWriter.Write(data) != nil {
		configWriter.Cleanup()
		logger.Error("Error writing telemetry config to shared memory: ", zap.Error(err))
		return nil, err
	}

	printTelemetryPackets()
	return configWriter, nil
}

// decomInitialize starts decommutation goroutines for each telemetry packet
func decomInitialize(ctx context.Context) map[int]chan []byte {
	channelMap := make(map[int]chan []byte)

	for _, packet := range proc.GswConfig.TelemetryPackets {
		finalOutputChannel := make(chan []byte)
		channelMap[packet.Port] = finalOutputChannel

		go func(packet tlm.TelemetryPacket, ch chan []byte) {
			proc.TelemetryPacketWriter(packet, ch)
			<-ctx.Done()
			close(ch)
		}(packet, finalOutputChannel)
	}

	return channelMap
}

func dbInitialize(ctx context.Context, channelMap map[int]chan []byte, host string, port int) error {
	dbHandler := db.InfluxDBV1Handler{}
	err := dbHandler.Initialize(host, port)
	if err != nil {
		logger.Warn("Warning. Telemetry packets will not be published to database")
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

func readConfig() (*viper.Viper, int) {
	config := viper.New()
	configFilepath := flag.String("c", "gsw_service", "name of config file")
	doPprof := flag.Int("p", 0, "Port to run pprof server on. Leave empty or set to 0 to disable pprof server")
	flag.Parse()
	config.SetConfigName(*configFilepath)
	config.SetConfigType("yaml")
	config.AddConfigPath("data/config/")
	err := config.ReadInConfig()

	if err != nil {
		logger.Fatal("Error reading GSW config: %w", zap.Error(err))
	}
	if !config.IsSet("database_host_name") {
		logger.Panic("Error reading GSW config: database_host_name not set...")
	}
	if !config.IsSet("database_port_number") {
		logger.Panic("Error reading GSW config: database_port_number not set...")
	}

	return config, *doPprof
}

func main() {
	logger.InitLogger()

	// Read gsw_service config
	config, profilingPort := readConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if profilingPort != 0 {
		go func() {
			logger.Info(fmt.Sprintf("Running pprof server at localhost:%d", profilingPort))
			err := http.ListenAndServe(fmt.Sprintf("localhost:%d", profilingPort), nil)

			if err != nil {
				logger.Warn("Unable to listen and serve")	
			}


		}()
	}

	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigs
		logger.Debug("Received signal: ", zap.String("signal", sig.String()))
		cancel()
	}()

	configWriter, err := vcmInitialize(config)
	if err != nil {
		logger.Panic("Exiting GSW...")
		return
	}
	defer configWriter.Cleanup()

	channelMap := decomInitialize(ctx)
	err = dbInitialize(ctx, channelMap, config.GetString("database_host_name"), config.GetInt("database_port_number"))
	if err != nil {
		logger.Warn("DB Initialization failed", zap.Error(err))
	}

	// Wait for context cancellation or signal handling
	<-ctx.Done()
	logger.Info("Shutting down GSW...")
}
