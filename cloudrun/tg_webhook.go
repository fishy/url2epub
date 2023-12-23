package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"go.yhsif.com/url2epub/rmapi"
	"go.yhsif.com/url2epub/tgbot"

	// For grayscale to work correctly
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

const (
	startErrMsg  = `🚫 Failed to register token %q. Please double check your token is correct. It should be a 8-digit code from https://my.remarkable.com/device/desktop/connect.`
	startSaveErr = `🚫 Failed to save this registration. Please try again later.`
	startExplain = `ℹ️

To link your reMarkable account, go to https://my.remarkable.com/device/desktop/connect, copy the 8-digit code, and come back to type "` + startCommand + ` rm <8-digit code>".

To link your kindle device (experimental), type "` + startCommand + ` kindle <send-to-kindle-email>". You need to add "%s" to your "Approved Personal Document E-mail List".`
	startSuccessRM = `✅ Successfully linked your reMarkable account! It should appear as a "%s" device registered around %s in your account (https://my.remarkable.com/device/desktop).
By default all epubs are sent to your root directory. To set a different one, use ` + dirCommand + ` command. (Note that if you have a lot of files stored ` + dirCommand + ` command could be very slow or unable to success).
You can also use ` + fontCommand + ` to set the default font on the created epub files.`
	startSuccessKindle = `✅ Successfully saved your kindle email! Please remember that you need to add "%s" to your "Approved Personal Document E-mail List".`

	startExplainDropbox = `ℹ️

To link your dropbox account, go to %s to grant access, then copy the code at the final step, and come back to type "` + startCommand + ` dropbox <code>".`
	startSuccessDropbox = `✅ Successfully linked your Dropbox account!
By default all epubs are sent to your root directory. To set a different one, use ` + dirCommand + ` command. (Note that if you have a lot of files stored ` + dirCommand + ` command could be very slow or unable to success).`

	dropboxAuthURL     = "https://www.dropbox.com/oauth2/authorize?client_id=%s&token_access_type=offline&response_type=code"
	dropboxAuthExplain = `Please go to %s, copy the code at the end, and come back with "` + startCommand + ` dropbox <code>"`
	dropboxFailure     = `🚫 Failed to auth with dropbox.`

	notStartedMsg = `🚫 You had not run ` + startCommand + ` command successfully yet.`

	stopMsg = `✅ Successfully deleted your reMarkable token or Kindle email.
You can now go to https://my.remarkable.com/device/desktop to revoke access if it was reMarkable token.`

	dirMsg          = `You are currently saving to "%s", please choose a new directory to save to:`
	dirErrMsg       = `🚫 Failed to list directories. Please try again later.`
	dirSaveErr      = `🚫 Failed to save this directory. Please try again later.`
	dirOldErr       = `🚫 Failed to save this directory. Please try ` + dirCommand + ` command again later.`
	dirSuccess      = `✅ Saved!`
	dirSuccessMsg   = `✅ Your new directory "%s" is saved.`
	dirWrongAccount = dirCommand + ` is not supported by your account.`

	noURLmsg             = `🚫 No URL found in message.`
	unsupportedURLmsg    = `⚠️ Unsupported URL: "%s"`
	failedEpubMsg        = `🚫 Failed to generate epub from URL: "%s"`
	failedUploadRM       = `🚫 Failed to upload epub to your reMarkable account for URL: "%s"`
	failedUploadDropbox  = `🚫 Failed to upload epub to your Dropbox account for URL: "%s"`
	failedEmail          = `🚫 Failed to email epub to your kindle device for URL: "%s"`
	successUploadRM      = `✅ Uploaded "%s.epub" (%s) to your reMarkable account from URL: "%s"`
	successUploadDropbox = `✅ Uploaded "%s" (%s) to your Dropbox account from URL: "%s"`
	successEmail         = `✅ Sent "%s.epub" (%s) to your kindle device from URL: "%s"`
	epubMsg              = "ℹ️ Download your epub file here: %s"
)

func firstURLInMessage(ctx context.Context, message *tgbot.Message) string {
	for _, entity := range message.Entities {
		switch entity.Type {
		case "url":
			u16 := utf16.Encode([]rune(message.Text))
			if int64(len(u16)) < entity.Offset+entity.Length {
				slog.ErrorContext(
					ctx,
					"Unable to process url entity",
					"entity", entity,
					"text", message.Text,
				)
				continue
			}
			return string(utf16.Decode(u16[entity.Offset : entity.Offset+entity.Length]))
		case "text_link":
			return entity.URL
		}
	}
	return ""
}

func urlHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message) {
	chat := GetChat(ctx, message.Chat.ID)
	if chat == nil {
		replyMessage(ctx, w, message, notStartedMsg, true, nil)
		return
	}
	url := firstURLInMessage(ctx, message)
	if url == "" {
		replyMessage(ctx, w, message, noURLmsg, true, nil)
		return
	}
	id, title, data, err := getEpub(ctx, url, defaultUserAgent, true)
	if err != nil {
		if errors.Is(err, errUnsupportedURL) {
			replyMessage(ctx, w, message, fmt.Sprintf(unsupportedURLmsg, url), true, nil)
		} else {
			replyMessage(ctx, w, message, fmt.Sprintf(failedEpubMsg, url), true, nil)
		}
		return
	}
	switch chat.Type {
	default:
		// Should not happen, but just in case
		slog.WarnContext(
			ctx,
			"urlHandler: unknown chat type",
			"type", chat.Type,
		)
		replyMessage(ctx, w, message, notStartedMsg, true, nil)

	case 0:
		// Should not happen, but just in case
		slog.WarnContext(ctx, "urlHandler: chat type = 0")
		fallthrough
	case AccountTypeRM:
		uploadRM(ctx, w, message, chat, url, id, title, data)

	case AccountTypeDropbox:
		uploadDropbox(ctx, w, message, chat, url, id, title, data)

	case AccountTypeKindle:
		sendKindleEmail(ctx, w, message, chat, url, title, data)
	}
}

func sendKindleEmail(
	ctx context.Context,
	w http.ResponseWriter,
	message *tgbot.Message,
	chat *EntityChatToken,
	url, title string,
	data *bytes.Buffer,
) {
	size := data.Len()
	var err error
	defer func(start time.Time) {
		slog.InfoContext(
			ctx,
			"sendKindleEmail: Finished",
			"took", time.Since(start),
			"epubSize", size,
			"title", title,
			"err", err,
		)
	}(time.Now())

	err = sendEmail(ctx, chat.KindleEmail, title, data, chat.Chat)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"sendKindleEmail: Failed to send kindle email",
			"err", err,
			"email", chat.KindleEmail,
		)
		replyMessage(ctx, w, message, fmt.Sprintf(failedEmail, url), true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(successEmail, title, prettySize(size), url), true, nil)
}

func uploadRM(
	ctx context.Context,
	w http.ResponseWriter,
	message *tgbot.Message,
	chat *EntityChatToken,
	url, id, title string,
	data *bytes.Buffer,
) {
	client := &rmapi.Client{
		RefreshToken: chat.RMToken,
	}
	size := data.Len()
	var err error
	defer func(start time.Time) {
		slog.InfoContext(
			ctx,
			"uploadRM: Finished",
			"took", time.Since(start),
			"epubSize", size,
			"id", id,
			"title", title,
			"err", err,
		)
	}(time.Now())
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	err = client.Upload(ctx, rmapi.UploadArgs{
		ID:       id,
		Title:    title,
		Data:     data,
		Type:     rmapi.FileTypeEpub,
		ParentID: chat.GetParentID(),
		ContentArgs: rmapi.ContentArgs{
			Font: chat.GetFont(),
		},
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"uploadRM: Upload failed",
			"err", err,
		)
		replyMessage(ctx, w, message, fmt.Sprintf(failedUploadRM, url), true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(successUploadRM, title, prettySize(size), url), true, nil)
}

func handleDropboxAuthError(
	ctx context.Context,
	w http.ResponseWriter,
	message *tgbot.Message,
	clientID string,
) func(*DropboxClient, error) *DropboxClient {
	return func(client *DropboxClient, err error) *DropboxClient {
		if err != nil {
			slog.ErrorContext(ctx, "dropbox auth failed", "err", err)

			var sb strings.Builder
			sb.WriteString(dropboxFailure)
			var dae DropboxAPIError
			if errors.As(err, &dae) {
				sb.WriteString(fmt.Sprintf(" This error detail might be helpful: %q.", dae.Summary))
				if dae.Tag == "invalid_grant" {
					sb.WriteString(" ")
					sb.WriteString(fmt.Sprintf(dropboxAuthExplain, fmt.Sprintf(dropboxAuthURL, clientID)))
				}
			}
			replyMessage(ctx, w, message, sb.String(), true, nil)
			return nil
		}
		return client
	}
}

func dropboxClientFromChat(
	ctx context.Context,
	w http.ResponseWriter,
	message *tgbot.Message,
	chat *EntityChatToken,
) *DropboxClient {
	clientID := os.Getenv("DROPBOX_CLIENT_ID")
	clientSecret := os.Getenv("SECRET_DROPBOX_TOKEN")

	return handleDropboxAuthError(ctx, w, message, clientID)(authDropboxRefresh(ctx, clientID, clientSecret, chat.DropboxToken))
}

func uploadDropbox(
	ctx context.Context,
	w http.ResponseWriter,
	message *tgbot.Message,
	chat *EntityChatToken,
	url, id, title string,
	data *bytes.Buffer,
) {
	client := dropboxClientFromChat(ctx, w, message, chat)
	if client == nil {
		// error message already replied
		return
	}
	var err error
	size := data.Len()
	defer func(start time.Time) {
		slog.InfoContext(
			ctx,
			"uploadDropbox: Finished",
			"took", time.Since(start),
			"epubSize", size,
			"id", id,
			"title", title,
			"err", err,
		)
	}(time.Now())
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	filename := dropboxFilenameCleaner.Replace(title) + ".epub"
	if chat.DropboxFolder != "" {
		filename = path.Join(chat.DropboxFolder, filename)
	}
	err = client.Upload(ctx, filename, data)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"uploadDropbox: Upload failed",
			"err", err,
		)
		replyMessage(ctx, w, message, fmt.Sprintf(failedUploadDropbox, url), true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(successUploadDropbox, filename, prettySize(size), url), true, nil)
}

func epubHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message) {
	url := firstURLInMessage(ctx, message)
	if url == "" && message.ReplyTo != nil {
		url = firstURLInMessage(ctx, message.ReplyTo)
	}
	if url == "" {
		replyMessage(ctx, w, message, noURLmsg, true, nil)
		return
	}
	var sb strings.Builder
	sb.WriteString(globalURLPrefix)
	sb.WriteString(epubEndpoint)
	sb.WriteString("?")
	params := make(neturl.Values)
	params.Set(queryURL, url)
	params.Set(queryGray, "1")
	params.Set(queryPassthroughUserAgent, "1")
	sb.WriteString(params.Encode())
	restURL := fmt.Sprintf(epubMsg, sb.String())
	replyMessage(ctx, w, message, restURL, true, nil)
	slog.InfoContext(
		ctx,
		"epubHandler: Generated rest url",
		"origUrl", url,
		"restUrl", restURL,
	)
}

func startHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message, text string) {
	payload := strings.TrimSpace(strings.TrimPrefix(text, startCommand))
	if payload == "" {
		replyMessage(ctx, w, message, fmt.Sprintf(
			startExplain,
			mgFrom(),
		), true, nil)
		return
	}
	const (
		rmPrefix      = "rm "
		kindlePrefix  = "kindle "
		dropboxPrefix = "dropbox"
	)
	if strings.HasPrefix(strings.ToLower(payload), rmPrefix) {
		startRM(ctx, w, message, payload[len(rmPrefix):])
		return
	}
	if strings.HasPrefix(strings.ToLower(payload), kindlePrefix) {
		startKindle(ctx, w, message, payload[len(kindlePrefix):])
		return
	}
	if strings.HasPrefix(strings.ToLower(payload), dropboxPrefix) {
		startDropbox(ctx, w, message, payload[len(dropboxPrefix):])
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(
		startExplain,
		mgFrom(),
	), true, nil)
}

func startRM(ctx context.Context, w http.ResponseWriter, message *tgbot.Message, payload string) {
	token := strings.TrimSpace(payload)
	if token == "" {
		replyMessage(ctx, w, message, fmt.Sprintf(
			startExplain,
			mgFrom(),
		), true, nil)
		return
	}
	client, err := rmapi.Register(ctx, rmapi.RegisterArgs{
		Token:       token,
		Description: rmDescription,
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"startHandler: Unable to register",
			"err", err,
		)
		replyMessage(ctx, w, message, fmt.Sprintf(
			startErrMsg,
			token,
		), true, nil)
		return
	}
	chat := &EntityChatToken{
		Chat:    message.Chat.ID,
		Type:    AccountTypeRM,
		RMToken: client.RefreshToken,
	}
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"startHandler: Unable to save chat",
			"err", err,
		)
		replyMessage(ctx, w, message, startSaveErr, true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(
		startSuccessRM, rmDescription, time.Now().Format("2006-01-02"),
	), true, nil)
}

func startKindle(ctx context.Context, w http.ResponseWriter, message *tgbot.Message, payload string) {
	email := strings.TrimSpace(payload)
	if email == "" {
		replyMessage(ctx, w, message, fmt.Sprintf(
			startExplain,
			mgFrom(),
		), true, nil)
		return
	}
	chat := &EntityChatToken{
		Chat:        message.Chat.ID,
		Type:        AccountTypeKindle,
		KindleEmail: email,
	}
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"startRM: Unable to save chat",
			"err", err,
		)
		replyMessage(ctx, w, message, startSaveErr, true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(
		startSuccessKindle,
		mgFrom(),
	), true, nil)
}

func startDropbox(ctx context.Context, w http.ResponseWriter, message *tgbot.Message, payload string) {
	dropboxClientID := os.Getenv("DROPBOX_CLIENT_ID")
	dropboxSecret := os.Getenv("SECRET_DROPBOX_TOKEN")

	code := strings.TrimSpace(payload)
	if code == "" {
		replyMessage(ctx, w, message, fmt.Sprintf(
			startExplainDropbox,
			fmt.Sprintf(dropboxAuthURL, dropboxClientID),
		), true, nil)
		return
	}
	client := handleDropboxAuthError(ctx, w, message, dropboxClientID)(authDropboxCode(ctx, dropboxClientID, dropboxSecret, code))
	if client == nil {
		// error already handled
		return
	}
	chat := &EntityChatToken{
		Chat:         message.Chat.ID,
		Type:         AccountTypeDropbox,
		DropboxToken: client.RefreshToken,
	}
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"startDropbox: Unable to save chat",
			"err", err,
		)
		replyMessage(ctx, w, message, startSaveErr, true, nil)
		return
	}
	replyMessage(ctx, w, message, startSuccessDropbox, true, nil)
}

func stopHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message) {
	chat := GetChat(ctx, message.Chat.ID)
	if chat == nil {
		replyMessage(ctx, w, message, notStartedMsg, true, nil)
		return
	}
	chat.Delete(ctx)
	replyMessage(ctx, w, message, stopMsg, true, nil)
}

func dirHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message) {
	chat := GetChat(ctx, message.Chat.ID)
	if chat == nil {
		replyMessage(ctx, w, message, notStartedMsg, true, nil)
		return
	}
	switch chat.Type {
	default:
		replyMessage(ctx, w, message, dirWrongAccount, true, nil)

	case 0:
		// Should not happen, but just in case
		slog.WarnContext(ctx, "dirHandler: chat type = 0")
		fallthrough
	case AccountTypeRM:
		dirRM(ctx, w, chat, message)

	case AccountTypeDropbox:
		dirDropbox(ctx, w, chat, message)
	}
}

func dirRM(ctx context.Context, w http.ResponseWriter, chat *EntityChatToken, message *tgbot.Message) {
	client := &rmapi.Client{
		RefreshToken: chat.RMToken,
	}
	dirs, err := client.ListDirs(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"dirRM: ListDirs failed",
			"err", err,
		)
		replyMessage(ctx, w, message, dirErrMsg, true, nil)
		return
	}
	choices := make([][]tgbot.InlineKeyboardButton, 0, len(dirs))
	for id, name := range dirs {
		choices = append(choices, []tgbot.InlineKeyboardButton{
			{
				Text: name,
				Data: dirIDPrefix + id,
			},
		})
	}
	sort.Slice(choices, func(i, j int) bool {
		return choices[i][0].Text < choices[j][0].Text
	})
	replyMessage(
		ctx,
		w,
		message,
		fmt.Sprintf(dirMsg, dirs[chat.GetParentID()]),
		true,
		&tgbot.InlineKeyboardMarkup{
			InlineKeyboard: choices,
		},
	)
}

func dirDropbox(ctx context.Context, w http.ResponseWriter, chat *EntityChatToken, message *tgbot.Message) {
	client := dropboxClientFromChat(ctx, w, message, chat)
	if client == nil {
		// error message already replied
		return
	}
	dirs, err := client.ListDirs(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"dirDropbox: ListDirs failed",
			"err", err,
		)
		replyMessage(ctx, w, message, dirErrMsg, true, nil)
		return
	}
	choices := make([][]tgbot.InlineKeyboardButton, 0, len(dirs))
	for _, dir := range dirs {
		choices = append(choices, []tgbot.InlineKeyboardButton{
			{
				Text: dir.Display,
				Data: dropboxDirPrefix + dir.Display,
			},
		})
	}
	sort.Slice(choices, func(i, j int) bool {
		return choices[i][0].Text < choices[j][0].Text
	})
	replyMessage(
		ctx,
		w,
		message,
		fmt.Sprintf(dirMsg, chat.DropboxFolder),
		true,
		&tgbot.InlineKeyboardMarkup{
			InlineKeyboard: choices,
		},
	)
}

func dirRMCallbackHandler(ctx context.Context, w http.ResponseWriter, data string, callback *tgbot.CallbackQuery) {
	if callback.Message == nil {
		slog.ErrorContext(
			ctx,
			"dirRMCallbackHandler: Bad callback",
			"data", data,
			"callback", callback,
		)
		getBot().ReplyCallback(ctx, callback.ID, dirOldErr)
		reply200(w)
		return
	}
	chat := GetChat(ctx, callback.Message.Chat.ID)
	if chat == nil {
		slog.ErrorContext(
			ctx,
			"dirRMCallbackHandler: Bad callback",
			"data", data,
			"chat", callback.Message.Chat.ID,
		)
		getBot().ReplyCallback(ctx, callback.ID, notStartedMsg)
		reply200(w)
		return
	}
	chat.RMParentID = data
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"dirRMCallbackHandler: Unable to save chat",
			"err", err,
		)
		getBot().ReplyCallback(ctx, callback.ID, dirSaveErr)
		reply200(w)
		return
	}
	if _, err := getBot().ReplyCallback(ctx, callback.ID, dirSuccess); err != nil {
		slog.ErrorContext(
			ctx,
			"dirRMCallbackHandler: Unable to reply to callback",
			"err", err,
		)
	}
	reply200(w)

	client := &rmapi.Client{
		RefreshToken: chat.RMToken,
	}
	dirs, err := client.ListDirs(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"dirRMCallbackHandler: Unable to list dir",
			"err", err,
		)
		return
	}
	getBot().SendMessage(
		ctx,
		callback.Message.Chat.ID,
		fmt.Sprintf(dirSuccessMsg, dirs[chat.GetParentID()]),
		&callback.Message.ID,
		nil,
	)
}

func dirDropboxCallbackHandler(ctx context.Context, w http.ResponseWriter, data string, callback *tgbot.CallbackQuery) {
	if callback.Message == nil {
		slog.ErrorContext(
			ctx,
			"dirDropboxCallbackHandler: Bad callback",
			"data", data,
			"callback", callback,
		)
		getBot().ReplyCallback(ctx, callback.ID, dirOldErr)
		reply200(w)
		return
	}
	chat := GetChat(ctx, callback.Message.Chat.ID)
	if chat == nil {
		slog.ErrorContext(
			ctx,
			"dirDropboxCallbackHandler: Bad callback",
			"data", data,
			"chat", callback.Message.Chat.ID,
		)
		getBot().ReplyCallback(ctx, callback.ID, notStartedMsg)
		reply200(w)
		return
	}
	dir := strings.TrimPrefix(data, dropboxDirPrefix)
	chat.DropboxFolder = dir
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"dirDropboxCallbackHandler: Unable to save chat",
			"err", err,
		)
		getBot().ReplyCallback(ctx, callback.ID, dirSaveErr)
		reply200(w)
		return
	}
	if _, err := getBot().ReplyCallback(ctx, callback.ID, dirSuccess); err != nil {
		slog.ErrorContext(
			ctx,
			"dirDropboxCallbackHandler: Unable to reply to callback",
			"err", err,
		)
	}
	reply200(w)

	getBot().SendMessage(
		ctx,
		callback.Message.Chat.ID,
		fmt.Sprintf(dirSuccessMsg, dir),
		&callback.Message.ID,
		nil,
	)
}

func reply200(w http.ResponseWriter) {
	code := http.StatusOK
	http.Error(w, http.StatusText(code), code)
}

func replyMessage(
	ctx context.Context,
	w http.ResponseWriter,
	orig *tgbot.Message,
	msg string,
	quote bool,
	markup *tgbot.InlineKeyboardMarkup,
) {
	reply := tgbot.ReplyMessage{
		Method:      "sendMessage",
		ChatID:      orig.Chat.ID,
		Text:        msg,
		ReplyMarkup: markup,
	}
	if quote {
		reply.ReplyTo = orig.ID
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reply)
}

var sizeUnits = []string{"KiB", "MiB"}

func prettySize(size int) string {
	s := fmt.Sprintf("%d B", size)
	n := float64(size)
	for _, unit := range sizeUnits {
		n = n / 1024
		if n < 0.95 {
			return s
		}
		s = fmt.Sprintf("%.1f %s", n, unit)
	}
	return s
}
