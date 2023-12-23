package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type DropboxAPIError struct {
	Code    int
	Summary string
	Tag     string
}

func (dae *DropboxAPIError) UnmarshalJSON(data []byte) error {
	var val struct {
		Summary     string `json:"error_summary"`
		Description string `json:"error_description"`

		Err json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	if val.Summary != "" {
		dae.Summary = val.Summary
	} else {
		dae.Summary = val.Description
	}

	var tag struct {
		Tag string `json:".tag"`
	}
	if err := json.Unmarshal([]byte(val.Err), &tag); err == nil {
		dae.Tag = tag.Tag
	} else if tag, err := strconv.Unquote(string(val.Err)); err == nil {
		dae.Tag = tag
	} else {
		dae.Tag = string(val.Err)
	}

	return nil
}

func (dae DropboxAPIError) Error() string {
	return fmt.Sprintf("dropbox API error response: code=%d, summary=%q, tag=%q", dae.Code, dae.Summary, dae.Tag)
}

func handleDropboxResponse(resp *http.Response, v any) error {
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	decoder := json.NewDecoder(resp.Body)
	if resp.StatusCode >= 400 {
		dae := DropboxAPIError{Code: resp.StatusCode}
		if err := decoder.Decode(&dae); err != nil {
			return fmt.Errorf("handleDropboxResponse: failed to json decode error response with code=%d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("handleDropboxResponse: %w", dae)
	}

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("handleDropboxResponse: failed to json decode response with code=%d: %w", resp.StatusCode, err)
	}
	return nil
}

type DropboxClient struct {
	Bearer       string
	RefreshToken string
}

func (c *DropboxClient) do(ctx context.Context, url string, req any) (*http.Response, error) {
	var body io.Reader
	if req != nil {
		buf := bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer func() {
			bufPool.Put(buf)
		}()
		if err := json.NewEncoder(buf).Encode(req); err != nil {
			return nil, fmt.Errorf("DropboxClient.do: failed to json encode request: %w", err)
		}
		body = buf
	} else {
		body = strings.NewReader("{}")
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("DropboxClient.do: failed to create request: %w", err)
	}
	r.Header.Set("authorization", "Bearer "+c.Bearer)
	r.Header.Set("content-type", "application/json")
	return httpClient.Do(r)
}

type DropboxEntry struct {
	Tag     string `json:".tag"`
	ID      string `json:"id"`
	Path    string `json:"path_lower,omitempty"`
	Display string `json:"path_display,omitempty"`
}

type listResult struct {
	Cursor  string         `json:"cursor"`
	HasMore bool           `json:"has_more"`
	Entries []DropboxEntry `json:"entries"`
}

type listFolderRequest struct {
	Path string `json:"path"`

	Recursive             bool `json:"recursive"`
	IncludeDeleted        bool `json:"include_deleted"`
	IncludeMountedFolders bool `json:"include_mounted_folders"`
}

type continueRequest struct {
	Cursor string `json:"cursor"`
}

func (c *DropboxClient) listDirsSingleRequest(ctx context.Context, cursor string) (*listResult, error) {
	var url string
	var req any
	if cursor == "" {
		url = "https://api.dropboxapi.com/2/files/list_folder"
		req = listFolderRequest{
			Path: "",

			Recursive:             true,
			IncludeDeleted:        false,
			IncludeMountedFolders: true,
		}
	} else {
		url = "https://api.dropboxapi.com/2/files/list_folder/continue"
		req = continueRequest{Cursor: cursor}
	}
	resp, err := c.do(ctx, url, req)
	if err != nil {
		return nil, err
	}

	var result listResult
	if err := handleDropboxResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *DropboxClient) ListDirs(ctx context.Context) ([]DropboxEntry, error) {
	start := time.Now()
	var folders []DropboxEntry
	var cursor string
	var i int
	for {
		i++
		result, err := c.listDirsSingleRequest(ctx, cursor)
		if err != nil {
			return folders, err
		}
		for _, entry := range result.Entries {
			if entry.Tag != "folder" {
				continue
			}
			folders = append(folders, entry)
		}
		cursor = result.Cursor
		if !result.HasMore {
			break
		}
	}
	slog.DebugContext(ctx, "ListDirs done", "took", time.Since(start), "n", i)
	return folders, nil
}

type uploadRequest struct {
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`

	Mode       string `json:"mode"`
	AutoRename bool   `json:"autorename"`
}

type uploadResult struct {
	Path        string `json:"path_lower"`
	Display     string `json:"path_display"`
	ContentHash string `json:"content_hash"`
	Size        int64  `json:"size"`
}

func (c *DropboxClient) Upload(ctx context.Context, path string, file *bytes.Buffer) error {
	hash := DropboxContentHash(file.Bytes())
	var sb strings.Builder
	if err := json.NewEncoder(&sb).Encode(uploadRequest{
		Path:        path,
		ContentHash: hash,

		Mode:       "add",
		AutoRename: true,
	}); err != nil {
		return fmt.Errorf("DropboxClient.Upload: failed to json encode args: %w", err)
	}
	args := strings.TrimSpace(sb.String())
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://content.dropboxapi.com/2/files/upload", file)
	if err != nil {
		return fmt.Errorf("DropboxClient.Upload: failed to generate request: %w", err)
	}
	r.Header.Set("authorization", "Bearer "+c.Bearer)
	r.Header.Set("content-type", "application/octet-stream")
	r.Header.Set("Dropbox-API-Arg", args)
	resp, err := httpClient.Do(r)
	if err != nil {
		return err
	}

	var result uploadResult
	if err := handleDropboxResponse(resp, &result); err != nil {
		return err
	}
	return nil
}

func DropboxContentHash(data []byte) string {
	const blockSize = 4 * 1024 * 1024
	if len(data) <= 0 {
		return ""
	}

	sum := sha256.New()
	for i := 0; i < len(data); i += blockSize {
		end := i + blockSize
		if end > len(data) {
			end = len(data)
		}
		block := sha256.Sum256(data[i:end])
		sum.Write(block[:])
	}
	return hex.EncodeToString(sum.Sum(nil))
}

type authResult struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func authDropboxCode(ctx context.Context, id, secret, code string) (*DropboxClient, error) {
	return authDropbox(ctx, id, secret, "" /* refreshToken */, code)
}

func authDropboxRefresh(ctx context.Context, id, secret, refreshToken string) (*DropboxClient, error) {
	return authDropbox(ctx, id, secret, refreshToken, "" /* code */)
}

func authDropbox(ctx context.Context, id, secret, refreshToken, code string) (*DropboxClient, error) {
	args := make(url.Values)
	args.Set("client_id", id)
	args.Set("client_secret", secret)
	if refreshToken == "" {
		args.Set("grant_type", "authorization_code")
		args.Set("code", code)
	} else {
		args.Set("grant_type", "refresh_token")
		args.Set("refresh_token", refreshToken)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.dropbox.com/oauth2/token", strings.NewReader(args.Encode()))
	if err != nil {
		return nil, fmt.Errorf("authDropbox: failed to generate request: %w", err)
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	var result authResult
	if err := handleDropboxResponse(resp, &result); err != nil {
		return nil, err
	}
	if refreshToken == "" {
		refreshToken = result.RefreshToken
	}
	return &DropboxClient{
		Bearer:       result.AccessToken,
		RefreshToken: refreshToken,
	}, nil
}

var dropboxFilenameCleaner = strings.NewReplacer(
	// Chars unallowed in filenames for Kobo e-ink readers
	`:`, "_",
	`?`, "_",
	`"`, "_",
	`\`, "_",
	`|`, "_",
	`/`, "_",
)
