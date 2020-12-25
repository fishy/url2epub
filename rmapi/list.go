package rmapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/fishy/url2epub"
)

// RootDisplayName is the display name to be used by the root directory.
var RootDisplayName = "<ROOT>"

const listRequestURL = `/document-storage/json/2/docs`

// ListDirs lists all the directories user created on their reMarkable account.
//
// The returned map is in format of <id> -> <display name>.
// When error is nil, the map is guaranteed to have at least an entry of
// "" -> RootDisplayName.
func (c *Client) ListDirs(ctx context.Context) (map[string]string, error) {
	url := c.createRequestURL(listRequestURL)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("rmapi.ListDirs: %w", ctx.Err())
	}
	req := &http.Request{
		Method: http.MethodGet,
		URL:    url,
	}
	req = req.WithContext(ctx)
	if err := c.setAuthHeader(ctx, req); err != nil {
		return nil, fmt.Errorf("rmapi.ListDirs: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rmapi.ListDirs: http request failed: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rmapi.ListDirs: http status: %s", resp.Status)
	}
	var data []ItemInfo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("rmapi.ListDirs: failed to decode json payload: %w", err)
	}
	items := make(map[string]*ItemInfo)
	for _, _item := range data {
		item := _item
		if err := item.IsSuccess(); err != nil {
			c.Logger.Log(err.Error())
			continue
		}
		if item.Type == "CollectionType" {
			items[item.ID] = &item
		}
	}
	m := make(map[string]string)
	m[""] = RootDisplayName
	for k := range items {
		m[k] = resolveName(k, items, m)
	}
	return m, nil
}

func resolveName(k string, items map[string]*ItemInfo, m map[string]string) string {
	if m[k] != "" {
		return m[k]
	}
	item := items[k]
	if item == nil {
		// Should not happen, but just in case
		return "<UNKNOWN-PARENT>"
	}
	if item.Parent == "" {
		return item.Name
	}
	parent := resolveName(item.Parent, items, m)
	return path.Join(parent, item.Name)
}
