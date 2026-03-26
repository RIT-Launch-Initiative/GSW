package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
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

type resolvedDBConfig struct {
	v1 *db.InfluxDBV1Config
	v2 *db.InfluxDBV2Config
}

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

func dbInitialize(ctx context.Context, channelMap map[int]chan []byte, cfg resolvedDBConfig, wg *sync.WaitGroup) error {
	var handler db.Handler

	if cfg.v2 != nil {
		h := &db.InfluxDBV2Handler{}
		if err := h.InitializeWithConfig(*cfg.v2); err != nil {
			return fmt.Errorf("initializing InfluxDB V2: %w", err)
		}
		handler = h
		logger.Info("Using InfluxDB V2 handler with batching",
			zap.Uint("batchSize", cfg.v2.BatchSize),
			zap.Uint("flushIntervalMs", cfg.v2.FlushInterval),
		)
	} else if cfg.v1 != nil {
		h := &db.InfluxDBV1Handler{}
		if err := h.InitializeWithConfig(*cfg.v1); err != nil {
			return fmt.Errorf("initializing InfluxDB V1: %w", err)
		}
		handler = h
		logger.Info("Using InfluxDB V1 handler (UDP)")
	} else {
		return nil
	}

	for _, packet := range proc.GswConfig.TelemetryPackets {
		wg.Add(1)
		go func(packet tlm.TelemetryPacket, ch chan []byte) {
			defer wg.Done()
			proc.DatabaseWriter(ctx, handler, packet, ch)
		}(packet, channelMap[packet.Port])
	}
	return nil
}

func resolveDBConfig(config *viper.Viper) (resolvedDBConfig, error) {
	v2Map := config.GetStringMap("database_v2")
	if len(v2Map) > 0 {
		precision, err := db.ParsePrecision(config.GetString("database_v2.precision"))
		if err != nil {
			return resolvedDBConfig{}, fmt.Errorf("invalid database_v2.precision: %w", err)
		}

		v2cfg := db.InfluxDBV2Config{
			URL:           config.GetString("database_v2.url"),
			Token:         config.GetString("database_v2.token"),
			Org:           config.GetString("database_v2.org"),
			Bucket:        config.GetString("database_v2.bucket"),
			BatchSize:     uint(config.GetInt("database_v2.batch_size")),
			FlushInterval: uint(config.GetInt("database_v2.flush_interval_ms")),
			Precision:     precision,
		}

		if v2cfg.URL == "" || v2cfg.Org == "" || v2cfg.Bucket == "" {
			return resolvedDBConfig{}, errors.New("database_v2.url, database_v2.org, and database_v2.bucket are required when database_v2 is set")
		}

		return resolvedDBConfig{v2: &v2cfg}, nil
	}

	if config.IsSet("database_v2_url") || config.IsSet("database_v2_org") || config.IsSet("database_v2_bucket") {
		logger.Warn("Legacy database_v2_* keys are deprecated; prefer nested database_v2.* config")

		precision, err := db.ParsePrecision(config.GetString("database_v2_precision"))
		if err != nil {
			return resolvedDBConfig{}, fmt.Errorf("invalid database_v2_precision: %w", err)
		}

		v2cfg := db.InfluxDBV2Config{
			URL:           config.GetString("database_v2_url"),
			Token:         config.GetString("database_v2_token"),
			Org:           config.GetString("database_v2_org"),
			Bucket:        config.GetString("database_v2_bucket"),
			BatchSize:     uint(config.GetInt("database_v2_batch_size")),
			FlushInterval: uint(config.GetInt("database_v2_flush_interval_ms")),
			Precision:     precision,
		}

		if v2cfg.URL == "" || v2cfg.Org == "" || v2cfg.Bucket == "" {
			return resolvedDBConfig{}, errors.New("database_v2_url, database_v2_org, and database_v2_bucket are required when legacy database_v2_* settings are used")
		}

		return resolvedDBConfig{v2: &v2cfg}, nil
	}

	hostSet := config.IsSet("database_host_name")
	portSet := config.IsSet("database_port_number")
	if hostSet || portSet {
		host := config.GetString("database_host_name")
		port := config.GetInt("database_port_number")
		if host == "" || port <= 0 {
			return resolvedDBConfig{}, errors.New("database_host_name and database_port_number must both be set for InfluxDB V1")
		}
		v1cfg := db.InfluxDBV1Config{Host: host, Port: port}
		return resolvedDBConfig{v1: &v1cfg}, nil
	}

	return resolvedDBConfig{}, nil
}

func readConfig() (*viper.Viper, int) {
	config := viper.New()
	config.SetConfigName(*configFilepath)
	config.SetConfigType("yaml")
	config.SetEnvPrefix("GSW")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
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

	resolvedDB, err := resolveDBConfig(config)
	if err != nil {
		logger.Warn("Database configuration is invalid; telemetry packets will not be published to the database", zap.Error(err))
	} else if resolvedDB.v1 != nil || resolvedDB.v2 != nil {
		if err = dbInitialize(ctx, channelMap, resolvedDB, &wg); err != nil {
			logger.Warn("DB initialization failed, telemetry packets will not be published to the database", zap.Error(err))
		}
	} else {
		logger.Info("No database configuration found; telemetry packets will not be published to the database")
	}

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down GSW...")
	wg.Wait()
	logger.Info("GSW stopped")
}
