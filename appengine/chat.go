package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/datastore"
	"google.golang.org/appengine/v2/memcache"
)

const (
	chatKey  = "chat-%d"
	chatKind = "rmapi-token"
)

var mc = memcache.JSON

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

// SaveMemcache saves this token into memcache.
func (e *EntityChatToken) SaveMemcache(ctx context.Context) error {
	return mc.Set(ctx, &memcache.Item{
		Key:    e.getKey(),
		Object: e,
	})
}

// SaveDatastore saves this entity into datastore.
func (e *EntityChatToken) SaveDatastore(ctx context.Context) error {
	key := e.datastoreKey()
	_, err := dsClient.Put(ctx, key, e)
	return err
}

// Save saves this entity into datastore and memcache
func (e *EntityChatToken) Save(ctx context.Context) error {
	err := e.SaveDatastore(ctx)
	if err == nil {
		e.SaveMemcache(ctx)
	}
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
	if err := memcache.Delete(ctx, e.getKey()); err != nil && !errors.Is(err, memcache.ErrCacheMiss) {
		l(ctx).Errorw(
			"Failed to delete memcache key",
			"key", e.getKey(),
			"err", err,
		)
	}
}

// GetChat gets an entity from db.
func GetChat(ctx context.Context, id int64) *EntityChatToken {
	e := &EntityChatToken{
		Chat: id,
	}
	_, err := mc.Get(ctx, e.getKey(), &e)
	if err == nil {
		return e
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
	if err := e.SaveMemcache(ctx); err != nil {
		l(ctx).Errorw(
			"Failed to save memcache key",
			"key", e.getKey(),
			"err", err,
		)
	}
	return e
}
