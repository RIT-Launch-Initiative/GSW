package logger

import (
	"crypto/internal/edwards25519/field"
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init(){
	//Default Logger
	defaultLogger := zap.Must(zap.NewDevelopment())

	// Viper config parsing 	
	viperConfig := viper.New()
	viperConfig.SetConfigType("yaml")
	viperConfig.SetConfigName("logger")
	viperConfig.AddConfigPath("data/config")
	
	if err := viperConfig.ReadInConfig(); err != nil {
		defaultLogger.Warn(fmt.Sprint(err))	
		logger = defaultLogger
		return 
	}
	// Create zap config
	var loggerConfig zap.Config
	
	outputPaths := viperConfig.GetStringSlice("OutputPaths")

	// Make and populate the output paths 
	for index, path := range outputPaths {
		if path == "stdout" || path == "stderr"{ 
			outputPaths[index] = path
			continue
		}
		// Create log file
		logFileName := fmt.Sprint("gsw_service_log-", time.Now().Format("2006-01-02 15:04:05"),".log")
		totalLogPath := fmt.Sprint(path,logFileName)

		// Ensures unique file name
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
			os.Mkdir(path, 0755)
			os.Create(totalLogPath)
		}
		outputPaths[index] = totalLogPath
	}
	// Setting Logger Paths
	loggerConfig.OutputPaths = outputPaths 
	loggerConfig.ErrorOutputPaths = outputPaths 

	// Setting Logger Level
	level, err := zap.ParseAtomicLevel(viperConfig.GetString("level"));

	if  err != nil{
		defaultLogger.Warn(fmt.Sprint(err))	
	}
	loggerConfig.Level = level 

	// Setting Encoding Type
	var levelEncoder zapcore.LevelEncoder
	var timeEncoder zapcore.TimeEncoder
	var durationEncoder zapcore.DurationEncoder
	var callerEncoder zapcore.CallerEncoder

	levelEncoder.UnmarshalText([]byte(viperConfig.GetString("encoderConfig.levelEncoder")))
	timeEncoder.UnmarshalText([]byte(viperConfig.GetString("encoderConfig.timeEncoder")))
	durationEncoder.UnmarshalText([]byte(viperConfig.GetString("encoderConfig.durationEncoder")))
	callerEncoder.UnmarshalText([]byte(viperConfig.GetString("encoderConfig.callerEncoder")))

	loggerConfig.Encoding = viperConfig.GetString("encoding")
	loggerConfig.EncoderConfig = zapcore.EncoderConfig{
		MessageKey: viperConfig.GetString("encoderConfig.messageKey"),	
		LevelKey: viperConfig.GetString("encoderConfig.levelKey"),
		TimeKey: viperConfig.GetString("encoderConfig.timeKey"),
		NameKey: viperConfig.GetString("encoderConfig.nameKey"),
		CallerKey: viperConfig.GetString("encoderConfig.callerKey"),
		StacktraceKey: viperConfig.GetString("encoderConfig.stacktraceKey"),
		LineEnding: viperConfig.GetString("encoderConfig.LineEnding"),
		EncodeLevel: levelEncoder,
		EncodeTime: timeEncoder,
		EncodeDuration: durationEncoder,
		EncodeCaller: callerEncoder,
	}

	logger = zap.Must(loggerConfig.Build())
}

func Info(message string, fields... zap.Field){
	logger.Info(message, fields...)
}

func Warn(message string, fields... zap.Field){
	logger.Warn(message, fields...)
}

func Debug(message string, fields... zap.Field){
	logger.Debug(message, fields...)
}

func Fatal(message string, fields... zap.Field){
	logger.Fatal(message, fields...)
}

func Error(message string, fields... zap.Field){
	logger.Error(message, fields...)
}

func Panic(message string, fields... zap.Field){
	logger.Panic(message, fields...)
}

