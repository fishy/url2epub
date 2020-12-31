package main

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/datastore"
)

const (
	chatKey  = "chat-%d"
	chatKind = "rmapi-token"
)

// EntityChatToken is the entity rmapi token for a chat stored in datastore.
type EntityChatToken struct {
	Chat     int64  `datastore:"chat"`
	Token    string `datastore:"token"`
	ParentID string `datastore:"parent"`
	Font     string `datastore:"font"`
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

// Delete deletes this entity from datastore.
func (e *EntityChatToken) Delete(ctx context.Context) {
	key := e.datastoreKey()
	if err := dsClient.Delete(ctx, key); err != nil {
		l(ctx).Errorw(
			"Failed to delete datastore key",
			"key", key,
			"err", err,
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
		l(ctx).Errorw(
			"Failed to get datastore key",
			"key", key,
			"err", err,
		)
		return nil
	}
	return e
}
