package main

import (
	"context"
	"net/http"
	"os"

	"go.yhsif.com/ctxslog"
	"golang.org/x/exp/slog"
)

func initLogger() {
	logger := slog.New(ctxslog.ContextHandler(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: ctxslog.ChainReplaceAttr(
			ctxslog.GCPKeys,
			ctxslog.StringDuration,
		),
	})))
	if v, ok := os.LookupEnv("VERSION_TAG"); ok {
		logger = logger.With(slog.String("v", v))
	}
	slog.SetDefault(logger)
}

func logContext(r *http.Request) context.Context {
	return ctxslog.Attach(
		r.Context(),
		"httpRequest", ctxslog.HTTPRequest(r, ctxslog.GCPRealIP),
	)
}
