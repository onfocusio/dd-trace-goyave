package goyavetrace

import (
	"context"

	stdslog "log/slog"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"goyave.dev/goyave/v5/slog"
)

// AdaptLoggerFn logger adapter function to unify the goyave slogger with datadog's logger.
func AdaptLoggerFn(logger *slog.Logger) func(lvl tracer.LogLevel, msg string, a ...any) {
	return func(lvl tracer.LogLevel, msg string, a ...any) {
		level := stdslog.LevelInfo
		err := level.UnmarshalText([]byte(lvl.String()))
		if err != nil {
			level = stdslog.LevelInfo
		}
		logger.Log(context.Background(), level, msg, a...)
	}
}
