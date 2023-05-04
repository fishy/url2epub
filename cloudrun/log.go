package main

import (
	"context"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"golang.org/x/exp/slog"

	"go.yhsif.com/url2epub/logger"
)

func initLogger() {
	logger := slog.New(slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				switch a.Key {
				case slog.MessageKey:
					a.Key = "message"
				case slog.LevelKey:
					a.Key = "severity"
				}
			}
			switch a.Value.Kind() {
			case slog.KindDuration:
				a.Value = slog.StringValue(a.Value.Duration().String())
			}
			return a
		},
	}.NewJSONHandler(os.Stderr))
	if v, ok := os.LookupEnv("VERSION_TAG"); ok {
		logger = logger.With(slog.String("v", v))
	}
	slog.SetDefault(logger)
}

func realIP(r *http.Request) netip.Addr {
	// First, try X-Forwarded-For header
	// Note that cloud run appends the real ip to the end
	xForwardedFor := r.Header.Get("x-forwarded-for")
	split := strings.Split(xForwardedFor, ",")
	for i := len(split) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(split[i])
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			slog.Default().Debug(
				"Wrong forwarded ip",
				"x-forwarded-for", xForwardedFor,
				"ip", ip,
			)
			continue
		}
		if addr.IsPrivate() || addr.IsLoopback() {
			continue
		}
		return addr
	}

	// Next, use the one from r.RemoteIP
	if parsed, err := netip.ParseAddrPort(r.RemoteAddr); err != nil {
		slog.Default().Debug(
			"Cannot parse RemoteAddr",
			"remoteAddr", r.RemoteAddr,
		)
		return netip.Addr{}
	} else {
		return parsed.Addr()
	}
}

func logContext(r *http.Request) context.Context {
	return logger.SetContext(r.Context(), slog.Default().With(
		slog.Group(
			"httpRequest",
			slog.String("method", r.Method),
			slog.String("userAgent", r.UserAgent()),
			slog.String("ip", realIP(r).String()),
			slog.String("referer", r.Referer()),
			slog.String("protocol", r.Proto),
			slog.String("requestUrl", r.URL.String()),
		),
	))
}
