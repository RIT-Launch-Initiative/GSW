package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger = zap.Must(zap.NewDevelopment()).WithOptions(zap.AddCallerSkip(1)).Named("gsw")

// InitLogger Initializes the logger
// Configured using the logger.yaml file in the data/config directory
// If the file is not found, the logger will default to a development logger
func InitLogger() {
	viperConfig, err := loadLoggerConfig()
	if err != nil {
		logger.Warn("couldn't load logging config, using default logger", zap.Error(err))
		return
	}

	outputPaths, err := resolveOutputPaths(viperConfig.GetStringSlice("OutputPaths"))
	if err != nil {
		logger.Warn("failed to resolve output paths, using default logger", zap.Error(err))
		return
	}

	level, err := zap.ParseAtomicLevel(viperConfig.GetString("level"))
	if err != nil {
		logger.Warn("failed to parse log level, using INFO level", zap.Error(err))
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	encoderConfig, err := buildEncoderConfig(viperConfig)
	if err != nil {
		logger.Warn("failed to build encoder config, using default logger", zap.Error(err))
		return
	}

	loggerConfig := zap.Config{
		Level:            level,
		Development:      false,
		Encoding:         viperConfig.GetString("encoding"),
		OutputPaths:      outputPaths,
		ErrorOutputPaths: outputPaths,
		EncoderConfig:    encoderConfig,
	}

	logger = zap.Must(loggerConfig.Build(zap.AddCaller(), zap.AddCallerSkip(1)))
}

// loadLoggerConfig loads the logger configuration from a YAML file
func loadLoggerConfig() (*viper.Viper, error) {
	cfg := viper.New()
	cfg.SetConfigType("yaml")
	cfg.SetConfigName("logger")
	cfg.AddConfigPath("data/config")
	cfg.SetEnvPrefix("GSW_LOGGER")
	cfg.AutomaticEnv()
	if err := cfg.BindEnv("OutputPaths", "GSW_LOGGER_OUTPUT_PATHS"); err != nil {
		return nil, err
	}

	if err := cfg.ReadInConfig(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolveOutputPaths generates a log file name and creates the file for all GSW logs within a session
func resolveOutputPaths(paths []string) ([]string, error) {
	for i, path := range paths {
		if path == "stdout" || path == "stderr" {
			continue
		}

		logFileName := fmt.Sprintf("gsw_service_log-%s.log", time.Now().Format("2006-01-02 15-04-05"))
		baseName := filepath.Join(path, logFileName)
		totalPath := baseName
		num := 1

		for {
			if _, err := os.Stat(totalPath); os.IsNotExist(err) {
				break
			}
			totalPath = fmt.Sprintf("%s.%d", baseName, num)
			num++
		}

		if _, err := os.Create(totalPath); err != nil {
			if err := os.MkdirAll(path, 0755); err != nil {
				logger.Warn("Failed to create log directory", zap.Error(err))
				return nil, err
			}

			if _, err := os.Create(totalPath); err != nil {
				logger.Warn("Failed to create log file", zap.Error(err))
				return nil, err
			}
		}

		paths[i] = totalPath
	}
	return paths, nil
}

// buildEncoderConfig builds the Zap logger based on a Viper configuration
func buildEncoderConfig(cfg *viper.Viper) (zapcore.EncoderConfig, error) {
	encCfg := zapcore.EncoderConfig{
		MessageKey:    cfg.GetString("encoderConfig.messageKey"),
		LevelKey:      cfg.GetString("encoderConfig.levelKey"),
		TimeKey:       cfg.GetString("encoderConfig.timeKey"),
		NameKey:       cfg.GetString("encoderConfig.nameKey"),
		CallerKey:     cfg.GetString("encoderConfig.callerKey"),
		StacktraceKey: cfg.GetString("encoderConfig.stacktraceKey"),
		LineEnding:    zapcore.DefaultLineEnding,
	}

	var err error

	if encCfg.EncodeLevel, err = getLevelEncoder(cfg.GetString("encoderConfig.levelEncoder")); err != nil {
		logger.Warn("Invalid levelEncoder", zap.Error(err))
		return encCfg, err
	}

	if encCfg.EncodeTime, err = getTimeEncoder(cfg.GetString("encoderConfig.timeEncoder")); err != nil {
		logger.Warn("Invalid timeEncoder", zap.Error(err))
		return encCfg, err
	}

	if encCfg.EncodeDuration, err = getDurationEncoder(cfg.GetString("encoderConfig.durationEncoder")); err != nil {
		logger.Warn("Invalid durationEncoder", zap.Error(err))
		return encCfg, err
	}

	if encCfg.EncodeCaller, err = getCallerEncoder(cfg.GetString("encoderConfig.callerEncoder")); err != nil {
		logger.Warn("Invalid callerEncoder", zap.Error(err))
		return encCfg, err
	}

	return encCfg, nil
}

// getLevelEncoder determines which Zap level encoder should be used based on a string
func getLevelEncoder(name string) (zapcore.LevelEncoder, error) {
	switch name {
	case "capital":
		return zapcore.CapitalLevelEncoder, nil
	case "capitalColor":
		return zapcore.CapitalColorLevelEncoder, nil
	case "lowercase":
		return zapcore.LowercaseLevelEncoder, nil
	case "lowercaseColor":
		return zapcore.LowercaseColorLevelEncoder, nil
	default:
		return nil, fmt.Errorf("unsupported levelEncoder: %s", name)
	}
}

// getTimeEncoder determines which Zap time encoder should be used based on a string
func getTimeEncoder(name string) (zapcore.TimeEncoder, error) {
	switch name {
	case "iso8601":
		return zapcore.ISO8601TimeEncoder, nil
	case "millis":
		return zapcore.EpochMillisTimeEncoder, nil
	case "nanos":
		return zapcore.EpochNanosTimeEncoder, nil
	case "epoch":
		return zapcore.EpochTimeEncoder, nil
	default:
		return nil, fmt.Errorf("unsupported timeEncoder: %s", name)
	}
}

// getDurationEncoder determines which Zap duration encoder should be used based on a string
func getDurationEncoder(name string) (zapcore.DurationEncoder, error) {
	switch name {
	case "seconds":
		return zapcore.SecondsDurationEncoder, nil
	case "nanos":
		return zapcore.NanosDurationEncoder, nil
	case "string":
		return zapcore.StringDurationEncoder, nil
	default:
		return nil, fmt.Errorf("unsupported durationEncoder: %s", name)
	}
}

// getCallerEncoder determines which Zap caller encoder should be used based on a string
func getCallerEncoder(name string) (zapcore.CallerEncoder, error) {
	switch name {
	case "short":
		return zapcore.ShortCallerEncoder, nil
	case "full":
		return zapcore.FullCallerEncoder, nil
	default:
		return nil, fmt.Errorf("unsupported callerEncoder: %s", name)
	}
}

// Info logs an info message
func Info(message string, fields ...zap.Field) {
	logger.Info(message, fields...)
}

// Warn logs a warning message
func Warn(message string, fields ...zap.Field) {
	logger.Warn(message, fields...)
}

// Debug logs a debug message
func Debug(message string, fields ...zap.Field) {
	logger.Debug(message, fields...)
}

// Fatal logs a fatal message
func Fatal(message string, fields ...zap.Field) {
	logger.Fatal(message, fields...)
}

// Error logs an error message
func Error(message string, fields ...zap.Field) {
	logger.Error(message, fields...)
}

// Panic logs a panic message
func Panic(message string, fields ...zap.Field) {
	logger.Panic(message, fields...)
}

// Log retrieves the underlying zap logger
func Log() *zap.Logger {
	return logger.WithOptions(zap.AddCallerSkip(-1))
}
