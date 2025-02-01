package logger

import(
	"fmt"
	"time"
	"os"

	"go.uber.org/zap"
	"github.com/spf13/viper"
)

var logger *zap.Logger

func init(){
	//Default Logger
	logger := zap.Must(zap.NewDevelopment())

	// Viper config parsing 	
	viperConfig := viper.New()
	viperConfig.SetConfigType("yaml")
	viperConfig.SetConfigName("logger")
	viperConfig.AddConfigPath("data/config")
	
	if err := viperConfig.ReadInConfig(); err != nil {
		logger.Warn(fmt.Sprint(err))	
		return 
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

	}
	loggerConfig.Level = level 

	// Setting Encoding Type
	loggerConfig.Encoding = viperConfig.GetString("encoding")
	
	loggerConfig.EncoderConfig = zap.NewDevelopmentConfig().EncoderConfig

	logger = zap.Must(loggerConfig.Build())
}

func Info(message string, fields... zap.Field){
	logger.Info(message, fields...)
}

func Warn(message string, fields... zap.Field){
	logger.Warn(message, fields...)
}

func 	Debug(message string, fields... zap.Field){
	logger.Debug(message, fields...)
}

func Fatal(message string, fields... zap.Field){
	logger.Fatal(message, fields...)
}

func Error(message string, fields... zap.Field){
	logger.Error(message, fields...)
}

