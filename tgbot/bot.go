package tgbot

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.yhsif.com/url2epub"
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
		slog.DebugContext(
			ctx,
			"tgbot.Bot.PostRequest: HTTP POST",
			"endpoint", endpoint,
			"took", time.Since(start),
		)
	}()

	var req *http.Request
	req, err = http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		b.getURL(endpoint),
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return 0, fmt.Errorf("tgbot.PostRequest: failed to construct http request: %w", err)
	}
	req.Header.Set("Content-Type", postFormContentType)
	var resp *http.Response
	resp, err = http.DefaultClient.Do(req)
	if resp != nil && resp.Body != nil {
		defer url2epub.DrainAndClose(resp.Body)
	}
	if err != nil {
		return 0, fmt.Errorf("tgbot.PostRequest: endpoint %s err: %w", endpoint, err)
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
	return code, err
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
		if err := json.NewEncoder(&sb).Encode(*markup); err != nil {
			return 0, fmt.Errorf("tgbot.SendMessage: failed to create InlineKeyboardMarkup: %w", err)
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
		slog.DebugContext(ctx, fmt.Sprintf("hashPrefix == %s", b.hashPrefix))
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
