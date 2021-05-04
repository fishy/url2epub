package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"

	"go.yhsif.com/url2epub/tgbot"
)

const (
	epubTimeout   = time.Second * 5
	uploadTimeout = time.Second * 5
)

const (
	webhookMaxConn = 5

	errNoToken = "no telebot token"

	globalURLPrefix = `https://url2epub.appspot.com`
	webhookPrefix   = `/w/`

	rmDescription = `desktop-windows`

	startCommand = `/start`
	stopCommand  = `/stop`
	dirCommand   = `/dir`
	fontCommand  = `/font`

	unknownCallback = `🚫 Unknown callback`

	dirIDPrefix = `dir:`
	fontPrefix  = `font:`

	restDocURL = `https://github.com/fishy/url2epub/blob/main/REST.md`
)

var dsClient *datastore.Client

func main() {
	initLogger()

	ctx := context.Background()
	if err := initDatastoreClient(ctx); err != nil {
		l(ctx).Fatalw(
			"Failed to get data store client",
			"err", err,
		)
	}
	initBot(ctx)
	initTwitter(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc(webhookPrefix, webhookHandler)
	mux.HandleFunc("/epub", restEpubHandler)
	mux.HandleFunc("/_ah/health", healthCheckHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		l(ctx).Infow(
			"Using default port",
			"port", port,
		)
	}
	l(ctx).Infow(
		"Start listening",
		"port", port,
	)
	l(ctx).Errorw(
		"HTTP listener returned",
		"err", http.ListenAndServe(fmt.Sprintf(":%s", port), mux),
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
		l(ctx).Error("Empty webhook request body")
		http.NotFound(w, r)
		return
	}

	var update tgbot.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		l(ctx).Errorw(
			"Unable to decode json",
			"err", err,
		)
		http.NotFound(w, r)
		return
	}

	if callback := update.Callback; callback != nil {
		data := callback.Data
		switch {
		default:
			l(ctx).Errorw(
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
		l(ctx).Warnw("Not a message nor callback, ignoring...", "update", update)
		reply200(w)
		return
	}
	text := update.Message.Text
	switch {
	default:
		urlHandler(ctx, w, r, update.Message, text)
	case strings.HasPrefix(text, startCommand):
		startHandler(ctx, w, update.Message, text)
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

var tokenValue atomic.Value

// initBot initializes botToken.
func initBot(ctx context.Context) {
	secret, err := getSecret(ctx, tokenID)
	if err != nil {
		l(ctx).Errorw(
			"Failed to get token secret",
			"err", err,
		)
	}
	tokenValue.Store(&tgbot.Bot{
		Token:           secret,
		GlobalURLPrefix: globalURLPrefix,
		WebhookPrefix:   webhookPrefix,
		Logger: func(msg string) {
			l(ctx).Info(msg)
		},
	})
	_, err = getBot().SetWebhook(ctx, webhookMaxConn)
	if err != nil {
		l(ctx).Fatalw(
			"Failed to set webhook",
			"err", err,
		)
	}
}

func getBot() *tgbot.Bot {
	return tokenValue.Load().(*tgbot.Bot)
}

var twitterBearerValue atomic.Value

// initTwitter initializes botToken.
func initTwitter(ctx context.Context) {
	secret, err := getSecret(ctx, twitterBearer)
	if err != nil {
		l(ctx).Errorw(
			"Failed to get twitter bearer secret",
			"err", err,
		)
	}
	twitterBearerValue.Store(secret)
}

func getTwitterBearer() string {
	s, _ := twitterBearerValue.Load().(string)
	return s
}

func getProjectID() string {
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}
