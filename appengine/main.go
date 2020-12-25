package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"

	"github.com/fishy/url2epub/logger"
	"github.com/fishy/url2epub/tgbot"
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
)

var dsClient *datastore.Client

// AppEngine log will auto add date and time, so there's no need to double log
// them in our own logger.
var (
	infoLog  = log.New(os.Stdout, "I ", log.Lshortfile)
	warnLog  = log.New(os.Stderr, "W ", log.Lshortfile)
	errorLog = log.New(os.Stderr, "E ", log.Lshortfile)
)

func main() {
	ctx := context.Background()
	if err := initDatastoreClient(ctx); err != nil {
		errorLog.Fatalf("Failed to get data store client: %v", err)
	}
	initBot(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc(webhookPrefix, webhookHandler)
	mux.HandleFunc("/_ah/health", healthCheckHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		infoLog.Printf("Defaulting to port %s", port)
	}
	infoLog.Printf("Listening on port %s", port)
	infoLog.Print(http.ListenAndServe(fmt.Sprintf(":%s", port), mux))
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
	ctx := context.Background()

	if !getBot().ValidateWebhookURL(r) {
		http.NotFound(w, r)
		return
	}

	if r.Body == nil {
		errorLog.Print("Empty webhook request body")
		http.NotFound(w, r)
		return
	}

	var update tgbot.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		errorLog.Printf("Unable to decode json: %v", err)
		http.NotFound(w, r)
		return
	}

	if callback := update.Callback; callback != nil {
		data := callback.Data
		switch {
		default:
			errorLog.Printf("Bad callback, data = %q, callback = %#v", data, callback)
			getBot().ReplyCallback(ctx, callback.ID, unknownCallback)
			reply200(w)
		case strings.HasPrefix(data, dirIDPrefix):
			dirCallbackHandler(ctx, w, data, callback)
		case strings.HasPrefix(data, fontPrefix):
			fontCallbackHandler(ctx, w, data, callback)
		}
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
	http.NotFound(w, r)
}

var tokenValue atomic.Value

// initBot initializes botToken.
func initBot(ctx context.Context) {
	secret, err := getSecret(ctx, tokenID)
	if err != nil {
		errorLog.Printf("Failed to get token secret: %v", err)
	}
	tokenValue.Store(&tgbot.Bot{
		Token:           secret,
		GlobalURLPrefix: globalURLPrefix,
		WebhookPrefix:   webhookPrefix,
		Logger:          logger.StdLogger(infoLog),
	})
	_, err = getBot().SetWebhook(ctx, webhookMaxConn)
	if err != nil {
		errorLog.Fatalf("Failed to set webhook: %v", err)
	}
}

func getBot() *tgbot.Bot {
	return tokenValue.Load().(*tgbot.Bot)
}

func getProjectID() string {
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}
