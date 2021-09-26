package rmapi

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"go.yhsif.com/url2epub"
)

// RootDisplayName is the display name to be used by the root directory.
var RootDisplayName = "<ROOT>"

// ListDirs lists all the directories user created on their reMarkable account.
//
// The returned map is in format of <id> -> <display name>.
// When error is nil, the map is guaranteed to have at least an entry of
// "" -> RootDisplayName.
func (c *Client) ListDirs(ctx context.Context) (map[string]string, error) {
	rootEntries, _, err := c.DownloadRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("rmapi.ListDirs: %w", err)
	}
	items := make(map[string]*Metadata)
	for _, entry := range rootEntries {
		if entry.NumFiles > 2 {
			// Directories should ot have more than 2 files (metadata + empty content
			// file), so we can skip every root entry with >2 files to save some
			// requests.
			continue
		}
		indexEntries, err := c.DownloadIndex(ctx, entry.Path)
		if err != nil {
			c.Logger.Log(fmt.Sprintf(
				"rmapi.ListDirs: failed to download index file for path %q and uuid %q: %v",
				entry.Path,
				entry.Filename,
				err,
			))
			continue
		}
		var metadataFound bool
		for _, index := range indexEntries {
			if !strings.HasSuffix(index.Filename, MetadataSuffix) {
				continue
			}
			resp, err := c.Download15(ctx, index.Path)
			if err != nil {
				c.Logger.Log(fmt.Sprintf(
					"rmapi.ListDirs: failed to download %s file for index %+v: %v",
					MetadataSuffix,
					index,
					err,
				))
				continue
			}
			var meta Metadata
			if err := func() error {
				defer url2epub.DrainAndClose(resp.Body)
				return json.NewDecoder(resp.Body).Decode(&meta)
			}(); err != nil {
				c.Logger.Log(fmt.Sprintf(
					"rmapi.ListDirs: failed to parse %s file for index %+v: %v",
					MetadataSuffix,
					index,
					err,
				))
				continue
			}
			metadataFound = true
			if meta.Type == "CollectionType" {
				items[entry.Filename] = &meta
			}
			break
		}
		if !metadataFound {
			c.Logger.Log(fmt.Sprintf(
				"rmapi.ListDirs: no %s file found for %+v: %v",
				MetadataSuffix,
				entry,
				err,
			))
		}
	}
	m := make(map[string]string)
	m[""] = RootDisplayName
	for k := range items {
		m[k] = resolveName(k, items, m)
	}
	return m, nil
}

func resolveName(k string, items map[string]*Metadata, m map[string]string) string {
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
