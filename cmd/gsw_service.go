package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AarC10/GSW-V2/lib/db"
	"github.com/AarC10/GSW-V2/lib/ipc"
	"github.com/AarC10/GSW-V2/lib/tlm"
	"github.com/spf13/viper"
	"go.uber.org/zap"

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

func vcmInitialize(config *viper.Viper) (*ipc.IpcShmHandler, error) {
	if !config.IsSet("telemetry_config") {
		err := errors.New("Error: Telemetry config filepath is not set in GSW config.")
		fmt.Printf("%v\n", err)
		return nil, err
	}
	data, err := os.ReadFile(config.GetString("telemetry_config"))
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		return nil, err
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return nil, err
	}
	configWriter, err := ipc.CreateIpcShmHandler("telemetry-config", len(data), true)
	if err != nil {
		fmt.Printf("Error creating shared memory handler: %v\n", err)
		return nil, err
	}
	if configWriter.Write(data) != nil {
		fmt.Printf("Error writing telemetry config to shared memory: %v\n", err)
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

func readConfig() *viper.Viper {
	config := viper.New()
	configFilepath := flag.String("c", "gsw_service", "name of config file")
	flag.Parse()
	config.SetConfigName(*configFilepath)
	config.SetConfigType("yaml")
	config.AddConfigPath("data/config/")
	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Error reading GSW config: %w", err))
	}
	return config
}


func readLogConfig() *zap.Logger {
	// Viper config parsing 	
	viperConfig := viper.New()
	viperConfig.SetConfigType("yaml")
	viperConfig.SetConfigName("logger")
	viperConfig.AddConfigPath("data/config")
	
	if err := viperConfig.ReadInConfig(); err != nil {
		zap.L().Warn(fmt.Sprint(err))	
		return nil
	}

	// Create zap config
	var loggerConfig zap.Config
	
	outputPaths := viperConfig.GetStringSlice("OutputPaths")
	errorOutputPaths := viperConfig.GetStringSlice("errorOutputPaths")
	
	// Create log file
	logFileName := fmt.Sprint("gsw_service_log-", time.Now().Format("2006-01-02 15:04:05"),".log")

	// TODO find out how to write to /run/log/journal
	totalLogPath := fmt.Sprint("../logs/",logFileName)

	// Make unique file name
	numIncrease := 0
	for {
		if _ ,err := os.Stat(totalLogPath); err != nil{
			break	
		}
		totalLogPath = fmt.Sprint(totalLogPath, ".", numIncrease)
		numIncrease++
	}
	_, err :=	os.Create(totalLogPath)

	// Setting Logger Paths
	loggerConfig.OutputPaths = append(outputPaths, totalLogPath) 
	loggerConfig.ErrorOutputPaths = append(errorOutputPaths, totalLogPath) 

	// Setting Logger Level
	level, err := zap.ParseAtomicLevel(viperConfig.GetString("level"));
	if  err != nil{
		zap.L().Warn(fmt.Sprint(err))	
		return nil	
	}
	loggerConfig.Level = level 

	// Setting Encoding Type
	loggerConfig.Encoding = viperConfig.GetString("encoding")
	
	loggerConfig.EncoderConfig = zap.NewDevelopmentConfig().EncoderConfig

	return zap.Must(loggerConfig.Build())
}

func init(){
	logger := readLogConfig()
	if logger == nil {
		zap.ReplaceGlobals(zap.Must(zap.NewDevelopment()))
	} else {
		zap.ReplaceGlobals(logger)
	}

	// Test things
	zap.L().Warn("This is a warn")
	zap.L().Info("This is info")
	zap.L().Debug("This is a debug")
	zap.L().Error("This is an error")
}

func main() {
	// Read gsw_service config
	config := readConfig()

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

	configWriter, err := vcmInitialize(config)
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
