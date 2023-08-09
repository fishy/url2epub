package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"go.yhsif.com/ctxslog"
)

func initLogger() {
	logger := ctxslog.New(
		ctxslog.WithAddSource(true),
		ctxslog.WithLevel(slog.LevelDebug),
		ctxslog.WithCallstack(slog.LevelError),
		ctxslog.WithReplaceAttr(ctxslog.ChainReplaceAttr(
			ctxslog.GCPKeys,
			ctxslog.StringDuration,
			ctxslog.StringInt,
		)),
	)
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

func chatContext(ctx context.Context, id int64) context.Context {
	return ctxslog.Attach(ctx, slog.String("chatID", strconv.FormatInt(id, 10)))
}
