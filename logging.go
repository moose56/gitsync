package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger A simple interface to abstract away any 3rd party logging module used. Only
// the required functionality is exposed.
type Logger interface {
	Debug(args ...interface{})
	Warn(args ...interface{})
	Info(args ...interface{})
	Panic(args ...interface{})
	Sync()
}

// simpleLogger wraps the 3rd party zap logger module
type simpleLogger struct {
	l *zap.SugaredLogger
}

func NewLogger(path string) Logger {
	loggerConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:       false,
		Encoding:          "console",
		DisableStacktrace: true,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr", path},
		ErrorOutputPaths: []string{"stderr", path},
	}
	logger, err := loggerConfig.Build()
	if err != nil {
		panic(err)
	}
	sugar := logger.Sugar()

	return &simpleLogger{l: sugar}
}

func (g simpleLogger) Warn(args ...interface{}) {
	g.l.Warn(args)
}

func (g simpleLogger) Info(args ...interface{}) {
	g.l.Info(args)
}

func (g simpleLogger) Debug(args ...interface{}) {
	g.l.Debug(args)
}

func (g simpleLogger) Panic(args ...interface{}) {
	g.l.Panic(args)
}

func (g simpleLogger) Sync() {
	err := g.l.Sync()
	if err != nil {
		return
	}
}
