package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"cloud.google.com/go/datastore"
	"go.yhsif.com/ctxslog"
)

const (
	chatKey  = "chat-%d"
	chatKind = "rmapi-token"
)

type AccountType int

const (
	_ AccountType = iota
	AccountTypeRM
	AccountTypeKindle
	AccountTypeDropbox
)

func (at AccountType) String() string {
	switch at {
	default:
		return fmt.Sprintf("<UNKNOWN-%d>", at)
	case AccountTypeRM:
		return "rm"
	case AccountTypeKindle:
		return "kindle"
	case AccountTypeDropbox:
		return "dropbox"
	}
}

func (at AccountType) MarshalText() ([]byte, error) {
	switch at {
	default:
		return nil, fmt.Errorf("unknown account type %d", at)

	case AccountTypeRM:
		fallthrough
	case AccountTypeKindle:
		fallthrough
	case AccountTypeDropbox:
		return []byte(at.String()), nil
	}
}

func (at *AccountType) UnmarshalText(text []byte) error {
	switch strings.ToLower(strings.TrimSpace(string(text))) {
	default:
		return fmt.Errorf("unknown account type %q", text)

	case "":
		// special default fallback for backward compatibility
		fallthrough
	case "rm":
		*at = AccountTypeRM

	case "kindle":
		*at = AccountTypeKindle

	case "dropbox":
		*at = AccountTypeDropbox
	}
	return nil
}

// EntityChatToken is the entity rmapi token for a chat stored in datastore.
type EntityChatToken struct {
	Chat     int64       `datastore:"chat" json:"chat"`
	Type     AccountType `datastore:"type" json:"type"`
	FitImage int         `datastore:"fit_image" json:"fit_image"`

	// reMarkable related fields
	RMToken    string `datastore:"token" json:"token"`
	RMParentID string `datastore:"parent" json:"parent"`
	RMFont     string `datastore:"font" json:"font"`

	// kindle related fields
	KindleEmail string `datastore:"email" json:"email"`

	// dropbox related fields
	DropboxToken  string `datastore:"dropbox_token" json:"dropbox_token"`
	DropboxFolder string `datastore:"dropbox_folder" json:"dropbox_folder"`
}

func (e *EntityChatToken) getKey() string {
	return fmt.Sprintf(chatKey, e.Chat)
}

func (e *EntityChatToken) datastoreKey() *datastore.Key {
	return datastore.NameKey(chatKind, e.getKey(), nil)
}

// GetParentID returns the ParentID to use, after stripping prefix,
func (e *EntityChatToken) GetParentID() string {
	return strings.TrimPrefix(e.RMParentID, dirIDPrefix)
}

// GetFont returns the Font to use, after stripping prefix,
func (e *EntityChatToken) GetFont() string {
	return strings.TrimPrefix(e.RMFont, fontPrefix)
}

// SaveDatastore saves this entity into datastore.
func (e *EntityChatToken) SaveDatastore(ctx context.Context) error {
	key := e.datastoreKey()
	_, err := dsClient.Put(ctx, key, e)
	return err
}

// Save saves this entity into datastore
func (e *EntityChatToken) Save(ctx context.Context) error {
	return e.SaveDatastore(ctx)
}

// Delete deletes this entity from datastore.
func (e *EntityChatToken) Delete(ctx context.Context) {
	key := e.datastoreKey()
	if err := dsClient.Delete(ctx, key); err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to delete datastore key",
			"err", err,
			"key", key,
		)
	}
}

// GetChat gets an entity from db.
func GetChat(ctx context.Context, id int64) (context.Context, *EntityChatToken) {
	e := &EntityChatToken{
		Chat: id,
	}
	key := e.datastoreKey()
	if err := dsClient.Get(ctx, key, e); err != nil {
		if !errors.Is(err, datastore.ErrNoSuchEntity) {
			slog.ErrorContext(
				ctx,
				"Failed to get datastore key",
				"err", err,
				"key", key,
			)
		}
		return ctx, nil
	}
	return ctxslog.Attach(ctx, "accountType", e.Type), e
}
