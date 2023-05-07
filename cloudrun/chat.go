package main

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/datastore"
	"golang.org/x/exp/slog"
)

const (
	chatKey  = "chat-%d"
	chatKind = "rmapi-token"
)

// EntityChatToken is the entity rmapi token for a chat stored in datastore.
type EntityChatToken struct {
	Chat     int64  `datastore:"chat" json:"chat"`
	Token    string `datastore:"token" json:"token"`
	ParentID string `datastore:"parent" json:"parent"`
	Font     string `datastore:"font" json:"font"`
}

func (e *EntityChatToken) getKey() string {
	return fmt.Sprintf(chatKey, e.Chat)
}

func (e *EntityChatToken) datastoreKey() *datastore.Key {
	return datastore.NameKey(chatKind, e.getKey(), nil)
}

// GetParentID returns the ParentID to use, after stripping prefix,
func (e *EntityChatToken) GetParentID() string {
	return strings.TrimPrefix(e.ParentID, dirIDPrefix)
}

// GetFont returns the Font to use, after stripping prefix,
func (e *EntityChatToken) GetFont() string {
	return strings.TrimPrefix(e.Font, fontPrefix)
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
		slog.ErrorCtx(
			ctx,
			"Failed to delete datastore key",
			"err", err,
			"key", key,
		)
	}
}

// GetChat gets an entity from db.
func GetChat(ctx context.Context, id int64) *EntityChatToken {
	e := &EntityChatToken{
		Chat: id,
	}
	key := e.datastoreKey()
	if err := dsClient.Get(ctx, key, e); err != nil {
		slog.ErrorCtx(
			ctx,
			"Failed to get datastore key",
			"err", err,
			"key", key,
		)
		return nil
	}
	return e
}
