package main

import (
	"context"
	"net/http"

	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
)

func initLogger() {
	logger, err := zapdriver.NewProduction()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}

type zapKeyType struct{}

var zapKey zapKeyType

func l(ctx context.Context) *zap.SugaredLogger {
	if l, ok := ctx.Value(zapKey).(*zap.SugaredLogger); ok {
		return l
	}
	return zap.S()
}

func logContext(r *http.Request) context.Context {
	return context.WithValue(r.Context(), zapKey, zap.S().With(
		zapdriver.HTTP(&zapdriver.HTTPPayload{
			RequestMethod: r.Method,
			UserAgent:     r.UserAgent(),
			RemoteIP:      r.RemoteAddr,
			Referer:       r.Referer(),
			Protocol:      r.Proto,
			RequestURL:    r.URL.String(),
		}),
	))
}
