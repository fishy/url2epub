package rmapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// UploadArgs defines the args used by Upload function.
type UploadArgs struct {
	ID    string
	Title string

	Data io.Reader
	Type FileType

	// Optional
	ParentID    string
	ContentArgs ContentArgs
}

const (
	defaultPagedata = ""
)

// Upload uploads a document to reMarkable.
//
// It returns the new root generation for debugging.
func (c *Client) Upload(ctx context.Context, args UploadArgs) (string, error) {
	now := time.Now()
	var entries []IndexEntry

	metaName := args.ID + MetadataSuffix
	meta := Metadata{
		Type:         "DocumentType",
		Name:         args.Title,
		Parent:       args.ParentID,
		Version:      1,
		LastModified: TimestampMillisecond(now),
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(meta); err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to json encode for %s: %w", metaName, err)
	}
	metaPath, metaSize, err := c.Upload15(ctx, buf)
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", metaName, err)
	}
	entries = append(entries, IndexEntry{
		Path:     metaPath,
		Unused1:  IndexEntryUnused1Magic,
		Filename: metaName,
		Size:     metaSize,
	})

	contentName := args.ID + ".content"
	content, err := args.Type.InitialContent(args.ContentArgs)
	if err != nil {
		return "", fmt.Errorf("unable to create %s: %w", contentName, err)
	}
	contentPath, contentSize, err := c.Upload15(ctx, strings.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", contentName, err)
	}
	entries = append(entries, IndexEntry{
		Path:     contentPath,
		Unused1:  IndexEntryUnused1Magic,
		Filename: contentName,
		Size:     contentSize,
	})

	pagedataName := args.ID + ".pagedata"
	pagedataPath, pagedataSize, err := c.Upload15(ctx, strings.NewReader(defaultPagedata))
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", pagedataName, err)
	}
	entries = append(entries, IndexEntry{
		Path:     pagedataPath,
		Unused1:  IndexEntryUnused1Magic,
		Filename: pagedataName,
		Size:     pagedataSize,
	})

	fileName := args.ID + args.Type.Ext()
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to generate gcs path for %s: %w", fileName, err)
	}
	filePath, fileSize, err := c.Upload15(ctx, args.Data)
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", fileName, err)
	}
	entries = append(entries, IndexEntry{
		Path:     filePath,
		Unused1:  IndexEntryUnused1Magic,
		Filename: fileName,
		Size:     fileSize,
	})

	indexName := args.ID
	indexPath, _, err := c.Upload15(ctx, GenerateIndex(entries))
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", indexName, err)
	}
	newEntry := IndexEntry{
		Path:     indexPath,
		Unused1:  RootEntryUnused1Magic,
		Filename: indexName,
		NumFiles: int64(len(entries)),
	}

	rootEntries, generation, err := c.DownloadRoot(ctx)
	if err != nil {
		return "", fmt.Errorf("rmapi.Client.Upload: failed to get current root: %w", err)
	}
	rootEntries = append(rootEntries, newEntry)
	rootPath, _, err := c.Upload15(ctx, GenerateIndex(rootEntries))
	if err != nil {
		return generation, fmt.Errorf("rmapi.Client.Upload: failed to upload %s: %w", indexName, err)
	}
	return generation, c.UpdateRoot(ctx, generation, rootPath)
}
