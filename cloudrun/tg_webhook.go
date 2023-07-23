package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/slog"

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
	startExplain = `ℹ️ To link your reMarkable account, go to https://my.remarkable.com/device/desktop/connect, copy the 8-digit code, and come back to type "` + startCommand + ` <8-digit code>"`
	startSuccess = `✅ Successfully linked your reMarkable account! It should appear as a "%s" device registered around %s in your account (https://my.remarkable.com/device/desktop).
By default all epubs are sent to your root directory. To set a different one, use ` + dirCommand + ` command. (Note that if you have a lot of files stored ` + dirCommand + ` command could be very slow or unable to success).
You can also use ` + fontCommand + ` to set the default font on the created epub files.`

	notStartedMsg = `🚫 You had not run ` + startCommand + ` command yet.`

	stopMsg = `✅ Successfully deleted your reMarkable token.
You can now go to https://my.remarkable.com/device/desktop to revoke access.`

	dirMsg        = `You are currently saving to "%s", please choose a new directory to save to:`
	dirErrMsg     = `🚫 Failed to list directories. Please try again later.`
	dirSaveErr    = `🚫 Failed to save this directory. Please try again later.`
	dirOldErr     = `🚫 Failed to save this directory. Please try ` + dirCommand + ` command again later.`
	dirSuccess    = `✅ Saved!`
	dirSuccessMsg = `✅ Your new directory "%s" is saved.`

	noURLmsg          = `🚫 No URL found in message.`
	unsupportedURLmsg = `⚠️ Unsupported URL: "%s"`
	failedEpubMsg     = `🚫 Failed to generate epub from URL: "%s"`
	failedUpload      = `🚫 Failed to upload epub to your reMarkable account for URL: "%s"`
	successUpload     = `✅ Uploaded "%s.epub" (%s) to your reMarkable account from URL: "%s"`
	epubMsg           = "ℹ️ Download your epub file here: %s"
)

func firstURLInMessage(ctx context.Context, message *tgbot.Message) string {
	for _, entity := range message.Entities {
		switch entity.Type {
		case "url":
			runes := []rune(message.Text)
			if int64(len(runes)) < entity.Offset+entity.Length {
				slog.ErrorContext(
					ctx,
					"Unable to process url entity",
					"entity", entity,
					"text", message.Text,
				)
				continue
			}
			return string(runes[entity.Offset : entity.Offset+entity.Length])
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
		slog.ErrorContext(
			ctx,
			"urlHandler: getEpub failed",
			"err", err,
			"url", url,
		)
		if errors.Is(err, errUnsupportedURL) {
			replyMessage(ctx, w, message, fmt.Sprintf(unsupportedURLmsg, url), true, nil)
		} else {
			replyMessage(ctx, w, message, fmt.Sprintf(failedEpubMsg, url), true, nil)
		}
		return
	}
	client := &rmapi.Client{
		RefreshToken: chat.Token,
	}
	start := time.Now()
	size := data.Len()
	defer func() {
		slog.InfoContext(
			ctx,
			"urlHandler: Uploaded",
			"took", time.Since(start),
			"epubSize", size,
			"err", err,
		)
	}()
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
			"urlHandler: Upload failed",
			"err", err,
			"url", url,
		)
		replyMessage(ctx, w, message, fmt.Sprintf(failedUpload, url), true, nil)
		return
	}
	replyMessage(ctx, w, message, fmt.Sprintf(successUpload, title, prettySize(size), url), true, nil)
	slog.InfoContext(
		ctx,
		"urlHandler: Uploaded epub to reMarkable",
		"epubSize", size,
		"id", id,
		"title", title,
		"chatId", message.Chat.ID,
	)
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
	token := strings.TrimPrefix(text, startCommand)
	token = strings.TrimSpace(token)
	if token == "" {
		replyMessage(ctx, w, message, startExplain, true, nil)
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
		Chat:  message.Chat.ID,
		Token: client.RefreshToken,
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
		startSuccess, rmDescription, time.Now().Format("2006-01-02"),
	), true, nil)
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
	client := &rmapi.Client{
		RefreshToken: chat.Token,
	}
	dirs, err := client.ListDirs(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"dirHandler: ListDirs failed",
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

func dirCallbackHandler(ctx context.Context, w http.ResponseWriter, data string, callback *tgbot.CallbackQuery) {
	if callback.Message == nil {
		slog.ErrorContext(
			ctx,
			"dirCallbackHandler: Bad callback",
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
			"dirCallbackHandler: Bad callback",
			"data", data,
			"chat", callback.Message.Chat.ID,
		)
		getBot().ReplyCallback(ctx, callback.ID, notStartedMsg)
		reply200(w)
		return
	}
	chat.ParentID = data
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"dirCallbackHandler: Unable to save chat",
			"err", err,
		)
		getBot().ReplyCallback(ctx, callback.ID, dirSaveErr)
		reply200(w)
		return
	}
	if _, err := getBot().ReplyCallback(ctx, callback.ID, dirSuccess); err != nil {
		slog.ErrorContext(
			ctx,
			"dirCallbackHandler: Unable to reply to callback",
			"err", err,
		)
	}
	reply200(w)

	client := &rmapi.Client{
		RefreshToken: chat.Token,
	}
	dirs, err := client.ListDirs(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"dirCallbackHandler: Unable to list dir",
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
