package main

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/exp/slog"

	"go.yhsif.com/url2epub/tgbot"
)

const (
	fontMsg        = `You are currently using default font "%s", please choose a new default font:`
	fontOldErr     = `🚫 Failed to save this default font. Please try ` + fontCommand + ` command again later.`
	fontSaveErr    = `🚫 Failed to save this default font. Please try again later.`
	fontSuccess    = `✅ Saved!`
	fontSuccessMsg = `✅ Your new default font "%s" is saved.`
)

type fontInfo struct {
	name string
	id   string
}

const (
	fontMaisonNeue = `Maison Neue`
	fontEBGaramond = `EB Garamond`
	fontNotoSans   = `Noto Sans`
	fontNotoSerif  = `Noto Serif`
	fontNotoMono   = `Noto Mono`
	fontNotoSansUI = `Noto Sans UI`
)

var fonts = [][]fontInfo{
	{
		{
			name: "<system default>",
			id:   "",
		},
	},
	{
		{
			name: fontMaisonNeue,
			id:   fontMaisonNeue,
		},
		{
			name: fontEBGaramond,
			id:   fontEBGaramond,
		},
	},
	{
		{
			name: fontNotoSans,
			id:   fontNotoSans,
		},
		{
			name: fontNotoSerif,
			id:   fontNotoSerif,
		},
	},
	{
		{
			name: fontNotoMono,
			id:   fontNotoMono,
		},
		{
			name: fontNotoSansUI,
			id:   fontNotoSansUI,
		},
	},
}

func fontHandler(ctx context.Context, w http.ResponseWriter, message *tgbot.Message) {
	chat := GetChat(ctx, message.Chat.ID)
	if chat == nil {
		replyMessage(ctx, w, message, notStartedMsg, true, nil)
		return
	}
	choices := make([][]tgbot.InlineKeyboardButton, len(fonts))
	for i, row := range fonts {
		rowChoices := make([]tgbot.InlineKeyboardButton, len(row))
		for j, item := range row {
			rowChoices[j] = tgbot.InlineKeyboardButton{
				Text: item.name,
				Data: fontPrefix + item.id,
			}
		}
		choices[i] = rowChoices
	}
	replyMessage(
		ctx,
		w,
		message,
		fmt.Sprintf(fontMsg, chat.GetFont()),
		true,
		&tgbot.InlineKeyboardMarkup{
			InlineKeyboard: choices,
		},
	)
}

func fontCallbackHandler(ctx context.Context, w http.ResponseWriter, data string, callback *tgbot.CallbackQuery) {
	if callback.Message == nil {
		slog.ErrorContext(
			ctx,
			"Bad callback",
			"data", data,
			"callback", callback,
		)
		getBot().ReplyCallback(ctx, callback.ID, fontOldErr)
		reply200(w)
		return
	}
	chat := GetChat(ctx, callback.Message.Chat.ID)
	if chat == nil {
		slog.ErrorContext(
			ctx,
			"Bad callback",
			"data", data,
			"chat", callback.Message.Chat.ID,
		)
		getBot().ReplyCallback(ctx, callback.ID, notStartedMsg)
		reply200(w)
		return
	}
	chat.Font = data
	if err := chat.Save(ctx); err != nil {
		slog.ErrorContext(
			ctx,
			"Unable to save chat",
			"err", err,
		)
		getBot().ReplyCallback(ctx, callback.ID, fontSaveErr)
		reply200(w)
		return
	}
	if _, err := getBot().ReplyCallback(ctx, callback.ID, dirSuccess); err != nil {
		slog.ErrorContext(
			ctx,
			"Unable to reply to callback",
			"err", err,
		)
	}
	getBot().SendMessage(
		ctx,
		callback.Message.Chat.ID,
		fmt.Sprintf(fontSuccessMsg, chat.GetFont()),
		&callback.Message.ID,
		nil,
	)
	reply200(w)
}
