package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/exp/slog"

	"go.yhsif.com/url2epub/tgbot"
)

const (
	epubTimeout   = time.Second * 15
	uploadTimeout = time.Second * 15
)

const (
	webhookMaxConn = 5

	globalURLPrefix = `https://url2epub.fishy.me`
	webhookPrefix   = `/w/`
	epubEndpoint    = `/epub`

	rmDescription = `desktop-windows`

	startCommand = `/start`
	stopCommand  = `/stop`
	dirCommand   = `/dir`
	fontCommand  = `/font`
	epubCommand  = `/epub`

	unknownCallback = `🚫 Unknown callback`

	dirIDPrefix = `dir:`
	fontPrefix  = `font:`

	restDocURL = `https://github.com/fishy/url2epub/blob/main/REST.md`

	userAgentTemplate = "url2epub/%s"
)

var defaultUserAgent string

var dsClient *datastore.Client

func main() {
	initLogger()

	if bi, ok := debug.ReadBuildInfo(); ok {
		slog.Debug(
			"Read build info",
			"string", bi.String(),
			"json", bi,
		)
	} else {
		slog.Warn("Unable to read build info")
	}

	ctx := context.Background()
	if err := initDatastoreClient(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to get data store client",
			"err", err,
		)
		os.Exit(1)
	}
	initBot(ctx)
	initTwitter(ctx)

	defaultUserAgent = fmt.Sprintf(userAgentTemplate, os.Getenv("K_REVISION"))
	slog.InfoContext(
		ctx,
		"default user agent",
		"user-agent", defaultUserAgent,
	)

	http.HandleFunc("/", rootHandler)
	http.HandleFunc(webhookPrefix, webhookHandler)
	http.HandleFunc(epubEndpoint, restEpubHandler)
	http.HandleFunc("/_ah/health", healthCheckHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		slog.WarnContext(
			ctx,
			"Using default port",
			"port", port,
		)
	}
	slog.InfoContext(
		ctx,
		"Started listening",
		"port", port,
	)

	slog.ErrorContext(
		ctx,
		"HTTP server returned",
		"err", http.ListenAndServe(fmt.Sprintf(":%s", port), nil),
	)
}

func initDatastoreClient(ctx context.Context) error {
	var err error
	dsClient, err = datastore.NewClient(ctx, getProjectID())
	return err
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "healthy")
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := logContext(r)

	if !getBot().ValidateWebhookURL(r) {
		http.NotFound(w, r)
		return
	}

	if r.Body == nil {
		slog.ErrorContext(ctx, "Empty webhook request body")
		http.NotFound(w, r)
		return
	}

	var update tgbot.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		slog.ErrorContext(
			ctx,
			"Unable to decode json",
			"err", err,
		)
		http.NotFound(w, r)
		return
	}

	if callback := update.Callback; callback != nil {
		ctx := chatContext(ctx, callback.Message.Chat.ID)
		data := callback.Data
		switch {
		default:
			slog.ErrorContext(
				ctx,
				"Bad callback",
				"data", data,
				"callback", callback,
			)
			getBot().ReplyCallback(ctx, callback.ID, unknownCallback)
			reply200(w)

		case strings.HasPrefix(data, dirIDPrefix):
			dirCallbackHandler(ctx, w, data, callback)
		case strings.HasPrefix(data, fontPrefix):
			fontCallbackHandler(ctx, w, data, callback)
		}
		return
	}

	if update.Message == nil {
		slog.WarnContext(ctx, "Not a message nor callback, ignoring...", "update", update)
		reply200(w)
		return
	}
	ctx = chatContext(ctx, update.Message.Chat.ID)
	text := update.Message.Text
	switch {
	default:
		urlHandler(ctx, w, update.Message)
	case strings.HasPrefix(text, startCommand):
		startHandler(ctx, w, update.Message, text)
	case strings.HasPrefix(text, epubCommand):
		epubHandler(ctx, w, update.Message)
	case text == stopCommand:
		stopHandler(ctx, w, update.Message)
	case text == dirCommand:
		dirHandler(ctx, w, update.Message)
	case text == fontCommand:
		fontHandler(ctx, w, update.Message)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, restDocURL, http.StatusTemporaryRedirect)
}

var tokenValue atomic.Pointer[tgbot.Bot]

// initBot initializes botToken.
func initBot(ctx context.Context) {
	secret := os.Getenv("SECRET_TELEGRAM_TOKEN")
	tokenValue.Store(&tgbot.Bot{
		Token:           secret,
		GlobalURLPrefix: globalURLPrefix,
		WebhookPrefix:   webhookPrefix,
	})
	if _, err := getBot().SetWebhook(ctx, webhookMaxConn); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to set webhook",
			"err", err,
		)
		os.Exit(1)
	}
}

func getBot() *tgbot.Bot {
	return tokenValue.Load()
}

var twitterBearerValue atomic.Pointer[string]

// initTwitter initializes botToken.
func initTwitter(ctx context.Context) {
	secret := os.Getenv("SECRET_TWITTER_BEARER")
	twitterBearerValue.Store(&secret)
}

func getTwitterBearer() string {
	s := twitterBearerValue.Load()
	if s == nil {
		return ""
	}
	return *s
}

func getProjectID() string {
	return os.Getenv("CLOUD_PROJECT_ID")
}
