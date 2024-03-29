package rmapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.yhsif.com/immutable"

	"go.yhsif.com/url2epub"
)

// Constants used in reMarkable 1.5 API.
const (
	// api urls
	APIBase         = "https://internal.cloud.remarkable.com/sync/v2"
	APIDownload     = APIBase + "/signed-urls/downloads"
	APIUpload       = APIBase + "/signed-urls/uploads"
	APISyncComplete = APIBase + "/sync-complete"

	// magic strings used in index files.
	IndexFileFirstMagic    = "3"
	RootEntryUnused1Magic  = "80000000"
	IndexEntryUnused1Magic = "0"

	// http headers
	HeaderRootGeneration = "x-goog-generation"

	// magic numbers
	NumEntrySplit = 5
	GCSPathBytes  = 32
)

// APIRequest defines the request json format for reMarkable 1.5 API.
type APIRequest struct {
	Method string `json:"http_method"`
	Path   string `json:"relative_path"`
}

// UpdateRootRequest defines the request json format for reMarkable 1.5 API.
type UpdateRootRequest struct {
	Method     string `json:"http_method"`
	Path       string `json:"relative_path"`
	Generation int64  `json:"generation,omitempty"`
	Root       string `json:"root_schema,omitempty"`
}

// SyncCompleteRequest is the payload for the request for sync-complete.
type SyncCompleteRequest struct {
	Generation int64 `json:"generation,omitempty"`
}

// APIResponse defines the response json format for reMarkable 1.5 API.
type APIResponse struct {
	Path    string
	URL     string
	Method  string
	Expires time.Time

	// It's possible the expires json value returned by reMarkable cloud cannot be
	// parsed as a timestamp. In such case Expires will be zero time.
	RawExpires string

	Headers map[string]string
}

var _ json.Unmarshaler = (*APIResponse)(nil)

// APIResponse special keys
const (
	APIResponseKeyPath    = "relative_path"
	APIResponseKeyURL     = "url"
	APIResponseKeyMethod  = "method"
	APIResponseKeyExpires = "expires"

	APIResponseMaxUploadSizeBytes = "maxuploadsize_bytes"
)

var responseNonHeaderKeys = immutable.SetLiteral(
	APIResponseKeyPath,
	APIResponseKeyURL,
	APIResponseKeyMethod,
	APIResponseKeyExpires,
	APIResponseMaxUploadSizeBytes,
)

// UnmarshalJSON implements json.Unmarshaler.
func (resp *APIResponse) UnmarshalJSON(data []byte) error {
	var m map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&m); err != nil {
		return err
	}

	resp.Path, _ = m[APIResponseKeyPath].(string)
	resp.URL, _ = m[APIResponseKeyURL].(string)
	resp.Method, _ = m[APIResponseKeyMethod].(string)

	var expires time.Time
	resp.RawExpires, _ = m[APIResponseKeyExpires].(string)
	if err := expires.UnmarshalJSON([]byte(`"` + resp.RawExpires + `"`)); err == nil {
		resp.Expires = expires
	}

	resp.Headers = make(map[string]string)
	for k, v := range m {
		if responseNonHeaderKeys.Contains(k) {
			continue
		}
		if s, ok := v.(string); ok {
			resp.Headers[k] = s
		}
	}

	if n, ok := m[APIResponseMaxUploadSizeBytes].(json.Number); ok {
		if i, err := n.Int64(); err == nil && i > 0 {
			resp.Headers["x-goog-content-length-range"] = fmt.Sprintf("0,%d", i)
		}
	}

	return nil
}

// Err checks for error returned by reMarkable cloud API.
func (resp *APIResponse) Err() error {
	if e, ok := resp.Headers["error"]; ok {
		return fmt.Errorf("reMarkable cloud API returned error: %q", e)
	}
	return nil
}

// ToRequest creates http request from the API response.
func (resp APIResponse) ToRequest(ctx context.Context, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, resp.Method, resp.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range resp.Headers {
		req.Header[k] = []string{v}
	}
	return req, nil
}

// Download15 is the "low-level" api that downloads a file in reMarkable Cloud
// API 1.5 via its GCS path.
func (c *Client) Download15(ctx context.Context, path string) (*http.Response, error) {
	payload := APIRequest{
		Method: http.MethodGet,
		Path:   path,
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return nil, fmt.Errorf("rmapi.Client.Download15: failed to json encode api request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, APIDownload, buf)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Client.Download15: failed to create api request: %w", err)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Client.Download15: failed to execute http request: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	var respPayload APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return nil, fmt.Errorf("rmapi.Client.Download15: failed to json decode api response: %w", err)
	}
	req, err = respPayload.ToRequest(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Client.Download15: failed to create gcs request: %w", err)
	}
	return http.DefaultClient.Do(req.WithContext(ctx))
}

// IndexEntry defines an entry in the index file in reMarkable 1.5 API.
type IndexEntry struct {
	// The GCS path
	Path string

	// For root this is the content uuid,
	// for index this is the filename (usually "uuid.ext")
	Filename string

	// For index NumFiles is always 0, for root Size is always 0
	NumFiles int64
	Size     int64

	// Should be either RootEntryUnused1Magic or IndexEntryUnused1Magic.
	Unused1 string
}

// ParseIndexEntry parses a line into IndexEntry.
func ParseIndexEntry(line string) (IndexEntry, error) {
	text := strings.TrimSpace(line)
	split := strings.Split(text, ":")
	if len(split) != NumEntrySplit {
		return IndexEntry{}, fmt.Errorf("rmapi.ParseIndexEntry: unrecognized index line: %q", text)
	}

	str := split[3]
	numFiles, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("rmapi.ParseIndexEntry: failed to parse num files %q: %w", str, err)
	}

	str = split[4]
	size, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return IndexEntry{}, fmt.Errorf("rmapi.ParseIndexEntry: failed to parse size %q: %w", str, err)
	}
	return IndexEntry{
		Path:     split[0],
		Unused1:  split[1],
		Filename: split[2],
		NumFiles: numFiles,
		Size:     size,
	}, nil
}

// DownloadIndex downloads and parses an index file by the GCS path in
// reMarkable 1.5 API.
func (c *Client) DownloadIndex(ctx context.Context, path string) ([]IndexEntry, error) {
	resp, err := c.Download15(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Client.DownloadIndex failed to download %q: %w", path, err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	var entries []IndexEntry
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		text := scanner.Text()
		if text == IndexFileFirstMagic {
			continue
		}
		entry, err := ParseIndexEntry(text)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to parse index line", "err", err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// DownloadRoot downloads and parses the root file in reMarkable 1.5 API.
func (c *Client) DownloadRoot(ctx context.Context) (entries []IndexEntry, generation string, err error) {
	resp, err := c.Download15(ctx, "root")
	if err != nil {
		return nil, "", fmt.Errorf("rmapi.Client.DownloadRoot: failed to get root file id: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	generation = resp.Header.Get(HeaderRootGeneration)
	var id strings.Builder
	if _, err := io.Copy(&id, resp.Body); err != nil {
		return nil, generation, fmt.Errorf("rmapi.Client.DownloadRoot: failed to read root file id: %w", err)
	}
	url2epub.DrainAndClose(resp.Body)
	entries, err = c.DownloadIndex(ctx, id.String())
	return entries, generation, err
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Upload15 is the "low-level" api that uploads a file in reMarkable Cloud API 1.5.
//
// It returns the GCS path (sha256 of the content) and the size.
func (c *Client) Upload15(ctx context.Context, content io.Reader) (path string, size int64, err error) {
	buf, ok := content.(*bytes.Buffer)
	if !ok {
		buf = bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufPool.Put(buf)
		if _, err := io.Copy(buf, content); err != nil {
			return "", 0, fmt.Errorf("rmapi.Client.Upload15: failed to read content: %w", err)
		}
	}
	hash := sha256.Sum256(buf.Bytes())
	path = hex.EncodeToString(hash[:])
	size = int64(buf.Len())

	payload := APIRequest{
		Method: http.MethodPut,
		Path:   path,
	}
	return path, size, c.upload15(ctx, payload, buf, nil)
}

func (c *Client) upload15(ctx context.Context, apiPayload interface{}, content io.Reader, extraHeaders map[string]string) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := json.NewEncoder(buf).Encode(apiPayload); err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to json encode api request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, APIUpload, buf)
	if err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to create api request: %w", err)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to execute http request: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	var payload APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to json decode api response: %w", err)
	}
	if err := payload.Err(); err != nil {
		return fmt.Errorf("rmapi.Client.upload15: %w", err)
	}
	for k, v := range extraHeaders {
		payload.Headers[k] = v
	}
	req, err = payload.ToRequest(ctx, content)
	if err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to create GCS upload request: %w, payload: %+v", err, payload)
	}
	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("rmapi.Client.upload15: failed to execute GCS upload request: %w, payload: %+v", err, payload)
	}
	defer url2epub.DrainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rmapi.Client.upload15: http status for GCS upload: %d/%s, %q", resp.StatusCode, resp.Status, readUpTo(resp.Body, 1024))
	}
	return nil
}

// GenerateIndex generates the index file expected by reMarkable API 1.5.
//
// It also sorts entries by its path as a side-effect.
func GenerateIndex(entries []IndexEntry) *bytes.Buffer {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	buf := new(bytes.Buffer)
	buf.WriteString(IndexFileFirstMagic)
	buf.WriteString("\n")
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf(
			"%s:%s:%s:%d:%d\n",
			entry.Path,
			entry.Unused1,
			entry.Filename,
			entry.NumFiles,
			entry.Size,
		))
	}
	return buf
}

// UpdateRoot updates the root file with the given path to the previously
// uploaded new root index.
func (c *Client) UpdateRoot(ctx context.Context, generation string, root string) error {
	gen, err := strconv.ParseInt(generation, 10, 64)
	if err != nil {
		return fmt.Errorf("rmapi.Client.syncComplete: failed to parse generation %q: %w", generation, err)
	}

	payload := UpdateRootRequest{
		Method:     http.MethodPut,
		Path:       "root",
		Generation: gen,
		Root:       root,
	}
	if err := c.upload15(ctx, payload, strings.NewReader(root), map[string]string{
		"x-goog-if-generation-match": generation,
	}); err != nil {
		return fmt.Errorf("rmapi.Client.UpdateRoot: failed to update root file: %w", err)
	}
	return c.syncComplete(ctx, gen)
}

func (c *Client) syncComplete(ctx context.Context, generation int64) error {
	payload := SyncCompleteRequest{
		Generation: generation,
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return fmt.Errorf("rmapi.Client.syncComplete: failed to json encode request payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APISyncComplete, buf)
	if err != nil {
		return fmt.Errorf("rmapi.Client.syncComplete: failed to create http request: %w", err)
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("rmapi.Client.syncComplete: failed to execute http request: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	body := readUpTo(resp.Body, 1024)
	switch resp.StatusCode {
	default:
		return fmt.Errorf("rmapi.Client.syncComplete: http status for sync-complete: %d/%s, %q", resp.StatusCode, resp.Status, body)

	case http.StatusOK:
		return nil

	case http.StatusConflict:
		// This is mostly harmless, just ignore it.
		return nil
	}
}

func readUpTo(r io.Reader, limit int64) string {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	io.Copy(buf, io.LimitReader(r, limit))
	return buf.String()
}
