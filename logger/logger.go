// Package logger provides wrappers around slog.
package logger // import "go.yhsif.com/url2epub/logger"

import (
	"context"

	"golang.org/x/exp/slog"
)

type logKeyType struct{}

var logKey logKeyType

func For(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(logKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

func SetContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, logKey, l)
}
