package main

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"go.yhsif.com/ctxslog"
	"golang.org/x/exp/slog"
)

func initLogger() {
	logger := slog.New(ctxslog.ContextHandler(ctxslog.JSONCallstackHandler(
		slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
			ReplaceAttr: ctxslog.ChainReplaceAttr(
				ctxslog.GCPKeys,
				ctxslog.StringDuration,
				func(groups []string, a slog.Attr) slog.Attr {
					switch a.Value.Kind() {
					case slog.KindInt64:
						a.Value = slog.StringValue(strconv.FormatInt(a.Value.Int64(), 10))
					case slog.KindUint64:
						a.Value = slog.StringValue(strconv.FormatUint(a.Value.Uint64(), 10))
					}
					return a
				},
			),
		}),
		slog.LevelError,
	)))
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
