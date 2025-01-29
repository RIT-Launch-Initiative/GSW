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
					zap.L().Warn(fmt.Sprint("Measurement '",measurementName,"' not found"))
					continue
				}
				fmt.Printf("\t\t%s\n", measurement.String())
			}
		} else {
			zap.L().Warn("No measurement defined.")
		}
	}
}

func vcmInitialize(config *viper.Viper) (*ipc.IpcShmHandler, error) {
	if !config.IsSet("telemetry_config") {
		err := errors.New("Error: Telemetry config filepath is not set in GSW config.")
		zap.L().Error(fmt.Sprint(err))
	}
	data, err := os.ReadFile(config.GetString("telemetry_config"))
	if err != nil {
		zap.L().Error(fmt.Sprint("Error reading YAML file: ", err))
	}
	_, err = proc.ParseConfigBytes(data)
	if err != nil {
		zap.L().Error(fmt.Sprint("Error parsing YAML: ", err))
		return nil, err
	}
	configWriter, err := ipc.CreateIpcShmHandler("telemetry-config", len(data), true)
	if err != nil {
		zap.L().Error(fmt.Sprint("Error creating shared memory handler: ", err))
		return nil, err
	}
	if configWriter.Write(data) != nil {
		configWriter.Cleanup()
		zap.L().Error(fmt.Sprint("Error writing telemetry config to shared memory: ", err))
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
		zap.L().Warn("Warning. Telemetry packets will not be published to database")
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
		zap.L().Error(fmt.Sprint("Error reading GSW config: %w", err))
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
	totalLogPath := fmt.Sprint("data/logs/",logFileName)

	//check if log folder exists
	_, err := os.Stat("data/logs/")

	// Make unique file name
	numIncrease := 0
	for {
		if _ ,err := os.Stat(totalLogPath); err != nil{
			break	
		}
		totalLogPath = fmt.Sprint(totalLogPath, ".", numIncrease)
		numIncrease++
	}
	_, noPath :=	os.Create(totalLogPath)

	if (noPath != nil){
		os.Mkdir("data/logs", 0755)
	}
	
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
	defer configWriter.Cleanup()
	if err != nil {
		zap.L().Info("Exiting GSW...")
		return
	}

	channelMap := decomInitialize(ctx)
	dbInitialize(ctx, channelMap)

	// Wait for context cancellation or signal handling
	<-ctx.Done()
	zap.L().Info("Shutting down GSW...")
}
