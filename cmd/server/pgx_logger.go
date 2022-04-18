package main

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.uber.org/zap"
)

type pgxLogger struct {
	log *zap.Logger
}

func (l *pgxLogger) Log(_ context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	logFunc := l.log.Info
	switch level {
	case pgx.LogLevelTrace, pgx.LogLevelDebug:
		logFunc = l.log.Debug
	case pgx.LogLevelInfo:
		logFunc = l.log.Info
	case pgx.LogLevelWarn:
		logFunc = l.log.Warn
	case pgx.LogLevelError:
		logFunc = l.log.Error
	case pgx.LogLevelNone:
		return
	}
	var fields []zap.Field
	for key, value := range data {
		fields = append(fields, zap.Any(key, value))
	}
	logFunc(msg, fields...)
}
