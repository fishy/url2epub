package main

import (
	"context"
	"net/http"
	"os"

	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger() {
	cfg := zapdriver.NewProductionConfig()
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	logger, err := cfg.Build(zapdriver.WrapCore())
	if err != nil {
		panic(err)
	}
	if v, ok := os.LookupEnv("VERSION_TAG"); ok {
		logger = logger.With(zap.String("v", v))
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
