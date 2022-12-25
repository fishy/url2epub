package tgbot

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.yhsif.com/url2epub"
	"go.yhsif.com/url2epub/logger"
)

const (
	urlPrefix           = "https://api.telegram.org/bot"
	postFormContentType = "application/x-www-form-urlencoded"
)

// Bot defines a telegram b with token.
type Bot struct {
	Token string

	GlobalURLPrefix string
	WebhookPrefix   string

	Logger logger.Logger

	hashOnce   sync.Once
	hashPrefix string
}

func (b *Bot) String() string {
	return b.Token
}

func (b *Bot) getURL(endpoint string) string {
	return fmt.Sprintf("%s%s/%s", urlPrefix, b.String(), endpoint)
}

// PostRequest use POST method to send a request to telegram
func (b *Bot) PostRequest(
	ctx context.Context,
	endpoint string,
	params url.Values,
) (code int, err error) {
	start := time.Now()
	defer func() {
		b.Logger.Log(fmt.Sprintf("HTTP POST for %s took %v", endpoint, time.Since(start)))
	}()

	var req *http.Request
	req, err = http.NewRequest(
		http.MethodPost,
		b.getURL(endpoint),
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		err = fmt.Errorf("failed to construct http request: %w", err)
		return
	}
	req.Header.Set("Content-Type", postFormContentType)
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if resp != nil && resp.Body != nil {
		defer url2epub.DrainAndClose(resp.Body)
	}
	if err != nil {
		err = fmt.Errorf("%s err: %w", endpoint, err)
		return
	}
	code = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf(
			"%s failed: code = %d, body = %q",
			endpoint,
			resp.StatusCode,
			buf,
		)
	}
	return
}

// SendMessage sents a telegram messsage.
func (b *Bot) SendMessage(
	ctx context.Context,
	id int64,
	msg string,
	replyTo *int64,
	markup *InlineKeyboardMarkup,
) (code int, err error) {
	values := url.Values{}
	values.Add("chat_id", strconv.FormatInt(id, 10))
	values.Add("text", msg)
	if replyTo != nil {
		values.Add("reply_to", strconv.FormatInt(*replyTo, 10))
	}
	if markup != nil {
		var sb strings.Builder
		err = json.NewEncoder(&sb).Encode(*markup)
		if err != nil {
			return
		}
		values.Add("reply_markup", sb.String())
	}
	return b.PostRequest(ctx, "sendMessage", values)
}

// ReplyCallback sents an answerCallbackQuery request.
func (b *Bot) ReplyCallback(ctx context.Context, id string, msg string) (code int, err error) {
	values := url.Values{}
	values.Add("callback_query_id", id)
	if msg != "" {
		values.Add("text", msg)
	}
	return b.PostRequest(ctx, "answerCallbackQuery", values)
}

func (b *Bot) initHashPrefix(ctx context.Context) {
	b.hashOnce.Do(func() {
		hash := sha512.Sum512_224([]byte(b.String()))
		b.hashPrefix = b.WebhookPrefix + base64.URLEncoding.EncodeToString(hash[:])
		b.Logger.Log(fmt.Sprintf("hashPrefix == %s", b.hashPrefix))
	})
}

func (b *Bot) getWebhookURL(ctx context.Context) string {
	b.initHashPrefix(ctx)
	return fmt.Sprintf("%s%s", b.GlobalURLPrefix, b.hashPrefix)
}

// ValidateWebhookURL validates whether requested URI in request matches hash
// path.
func (b *Bot) ValidateWebhookURL(r *http.Request) bool {
	b.initHashPrefix(r.Context())
	return r.URL.Path == b.hashPrefix
}

// SetWebhook sets webhook with telegram.
func (b *Bot) SetWebhook(ctx context.Context, webhookMaxConn int) (code int, err error) {
	b.initHashPrefix(ctx)

	values := url.Values{}
	values.Add("url", b.getWebhookURL(ctx))
	values.Add("max_connections", fmt.Sprintf("%d", webhookMaxConn))
	return b.PostRequest(ctx, "setWebhook", values)
}
