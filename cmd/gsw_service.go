package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/logger"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/AarC10/GSW-V2/proc"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"net/http"
	_ "net/http/pprof"
)

var (
	shmDir         = flag.String("shm", "/dev/shm", "directory to use for shared memory")
	configFilepath = flag.String("c", "gsw_service", "name of config file")
	doPprof        = flag.Int("p", 0, "Port to run pprof server on. Leave empty or set to 0 to disable pprof server")
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

// telemetryConfigInitialize reads the telemetry config file and writes
// it into shared memory. Returns the cleanup function.
func telemetryConfigInitialize(config *viper.Viper) (func(), error) {
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

	cleanup, err := proc.WriteTelemetryConfigToShm(*shmDir, data)
	if err != nil {
		logger.Error("Error writing telemetry config to shared memory: ", zap.Error(err))
		return nil, err
	}

	printTelemetryPackets()
	return cleanup, nil
}

// decomInitialize starts decommutation goroutines for each telemetry packet
func decomInitialize(ctx context.Context, wg *sync.WaitGroup) map[int]chan []byte {
	channelMap := make(map[int]chan []byte)

	for _, packet := range proc.GswConfig.TelemetryPackets {
		finalOutputChannel := make(chan []byte)
		channelMap[packet.Port] = finalOutputChannel

		wg.Add(1)
		go func(packet tlm.TelemetryPacket, ch chan []byte) {
			defer wg.Done()
			err := proc.TelemetryPacketWriter(ctx, packet, finalOutputChannel, *shmDir)
			if err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("error initializing packet writer", zap.Error(err))
			}
			close(ch)
		}(packet, finalOutputChannel)
	}

	return channelMap
}

func dbInitialize(ctx context.Context, channelMap map[int]chan []byte, host string, port int, wg *sync.WaitGroup) error {
	dbHandler := db.InfluxDBV1Handler{}
	if err := dbHandler.Initialize(host, port); err != nil {
		return err
	}

	for _, packet := range proc.GswConfig.TelemetryPackets {
		wg.Add(1)
		go func(packet tlm.TelemetryPacket, ch chan []byte) {
			defer wg.Done()
			proc.DatabaseWriter(ctx, &dbHandler, packet, ch)
		}(packet, channelMap[packet.Port])
	}
	return nil
}

func readConfig() (*viper.Viper, int) {
	config := viper.New()
	config.SetConfigName(*configFilepath)
	config.SetConfigType("yaml")
	config.SetEnvPrefix("GSW")
	config.AutomaticEnv()
	config.AddConfigPath("data/config/")
	err := config.ReadInConfig()

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			logger.Fatal("Error reading GSW config", zap.Error(err))
		} else {
			logger.Warn("Config file not found, reading config from environment variables")
		}
	}

	return config, *doPprof
}

func initProfiling(pprofPort int) {
	go func() {
		logger.Info(fmt.Sprintf("Running pprof server at localhost:%d", pprofPort))
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", pprofPort), nil)
		if err != nil {
			logger.Error("Error starting pprof server: ", zap.Error(err))
		}
	}()
}

func main() {
	flag.Parse()
	logger.InitLogger()

	config, profilingPort := readConfig()

	if profilingPort != 0 {
		initProfiling(profilingPort)
	}

	telemetryConfigCleanup, err := telemetryConfigInitialize(config)
	if err != nil {
		logger.Fatal("Exiting GSW...")
		return
	}
	defer telemetryConfigCleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigs
		logger.Debug("Received signal", zap.String("signal", sig.String()))
		cancel()
	}()

	var wg sync.WaitGroup

	// Start decom writers
	channelMap := decomInitialize(ctx, &wg)

	// Start DB writers
	if config.IsSet("database_host_name") && config.IsSet("database_port_number") {
		if err = dbInitialize(ctx, channelMap, config.GetString("database_host_name"), config.GetInt("database_port_number"), &wg); err != nil {
			logger.Warn("DB Initialization failed, telemetry packets will not be published to the database", zap.Error(err))
		}
	} else {
		logger.Warn("database_host_name or database_port_number is not set, telemetry packets will not be published to the database")
	}

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down GSW...")
	wg.Wait()
	logger.Info("GSW stopped")
}
