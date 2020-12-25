package tgbot

// Update is a update from telegram webhook.
type Update struct {
	ID       int64          `json:"update_id,omitempty"`
	Message  *Message       `json:"message,omitempty"`
	Callback *CallbackQuery `json:"callback_query,omitempty"`
}

// Message is a telegram message.
type Message struct {
	ID   int64  `json:"message_id,omitempty"`
	From User   `json:"from,omitempty"`
	Chat Chat   `json:"chat,omitempty"`
	Date int64  `json:"date,omitempty"`
	Text string `json:"text,omitempty"`

	Entities []MessageEntity `json:"entities,omitempty"`
}

// User is a telegram user.
type User struct {
	ID        int64  `json:"id,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	Langueage string `json:"language_code,omitempty"`
}

// Chat is a telegram chat.
type Chat struct {
	ID        int64  `json:"id,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	Type      string `json:"type,omitempty"`
}

// ReplyMessage is a message sent on webhook requests.
type ReplyMessage struct {
	Method  string `json:"method,omitempty"`
	ChatID  int64  `json:"chat_id,omitempty"`
	ReplyTo int64  `json:"reply_to_message_id,omitempty"`
	Text    string `json:"text,omitempty"`

	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

// InlineKeyboardMarkup is used to provide single choice replies.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard,omitempty"`
}

// InlineKeyboardButton represents a single choice inside InlineKeyboardMarkup.
type InlineKeyboardButton struct {
	Text string `json:"text,omitempty"`
	Data string `json:"callback_data,omitempty"`
}

// CallbackQuery is the callback from InlineKeyboardButton.
type CallbackQuery struct {
	ID      string   `json:"id,omitempty"`
	Data    string   `json:"data,omitempty"`
	Message *Message `json:"message,omitempty"`
}

// AnswerCallbackQuery is the answer to CallbackQuery.
type AnswerCallbackQuery struct {
	ID   string `json:"callback_query_id,omitempty"`
	Text string `json:"text,omitempty"`
}

// MessageEntity represents one special entity in a message (e.g. url)
type MessageEntity struct {
	Type   string `json:"type,omitempty"`
	URL    string `json:"url,omitempty"`
	Offset int64  `json:"offset,omitempty"`
	Length int64  `json:"length,omitempty"`
}
