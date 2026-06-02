package logging

import (
	"context"
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
)

type loggerKey struct{}

var defaultLogger = NewConsoleLogger(os.Stderr, zerolog.InfoLevel)

func DefaultLogger() zerolog.Logger {
	return defaultLogger
}

func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

func FromContext(ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return DefaultLogger()
	}
	if logger, ok := ctx.Value(loggerKey{}).(zerolog.Logger); ok {
		return logger
	}
	return DefaultLogger()
}

func NewConsoleLogger(out io.Writer, level zerolog.Level) zerolog.Logger {
	writer := zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = &lockedWriter{w: out}
	})
	return zerolog.New(writer).With().Timestamp().Logger().Level(level)
}

func NewDisabledLogger() zerolog.Logger {
	return zerolog.New(io.Discard).Level(zerolog.Disabled)
}

type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}
