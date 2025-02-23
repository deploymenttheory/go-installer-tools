package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var sugar *zap.SugaredLogger

// Init initializes the logger with the specified log level
func Init(level string) error {
	// Parse the log level
	var zapLevel zapcore.Level
	err := zapLevel.UnmarshalText([]byte(level))
	if err != nil {
		return err
	}

	// Create the encoder configuration
	encoderConfig := zapcore.EncoderConfig{
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // Use colors for levels
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create the core configuration
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      false,
		Sampling:         nil,
		Encoding:         "console", // Use console encoding instead of JSON
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Build the logger
	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	// Create the sugar logger
	sugar = logger.Sugar()
	return nil
}

// Debug logs a message at debug level
func Debug(msg string, args ...interface{}) {
	sugar.Debugw(msg, args...)
}

// Info logs a message at info level
func Info(msg string, args ...interface{}) {
	sugar.Infow(msg, args...)
}

// Warn logs a message at warn level
func Warn(msg string, args ...interface{}) {
	sugar.Warnw(msg, args...)
}

// Error logs a message at error level
func Error(msg string, args ...interface{}) {
	sugar.Errorw(msg, args...)
}

// Fatal logs a message at fatal level and exits
func Fatal(msg string, args ...interface{}) {
	sugar.Fatalw(msg, args...)
	os.Exit(1)
}

// Sync flushes any buffered log entries
func Sync() error {
	return sugar.Sync()
}
