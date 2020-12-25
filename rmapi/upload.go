package rmapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fishy/url2epub"
	"github.com/fishy/url2epub/ziputil"
)

const (
	uploadURL = `/document-storage/json/2/upload/request`
	updateURL = `/document-storage/json/2/upload/update-status`
)

// UploadArgs defines the args used by Upload function.
type UploadArgs struct {
	ID    string
	Title string

	Data io.Reader
	Type FileType

	// Optional
	ParentID string
}

func (a UploadArgs) generateData(meta ItemMetadata) (_ io.Reader, err error) {
	buf := new(bytes.Buffer)
	z := zip.NewWriter(buf)
	defer func() {
		closeErr := z.Close()
		if err == nil {
			err = closeErr
		}
	}()

	content, err := a.Type.InitialContent()
	if err != nil {
		return nil, fmt.Errorf("unable to create .content file: %w", err)
	}
	if err := ziputil.WriteFile(
		z,
		a.ID+".content",
		ziputil.StringWriterTo(content),
	); err != nil {
		return nil, err
	}

	metadata := new(bytes.Buffer)
	if err := json.NewEncoder(metadata).Encode(meta); err != nil {
		return nil, fmt.Errorf("unable to encode .metadata file: %w", err)
	}
	if err := ziputil.WriteFile(
		z,
		a.ID+".metadata",
		ziputil.WriterToWrapper(func(w io.Writer) (int64, error) {
			return io.Copy(w, metadata)
		}),
	); err != nil {
		return nil, err
	}

	if err := ziputil.WriteFile(
		z,
		a.ID+".pagedata",
		ziputil.StringWriterTo("{}"),
	); err != nil {
		return nil, err
	}

	if err := ziputil.WriteFile(
		z,
		a.ID+a.Type.Ext(),
		ziputil.WriterToWrapper(func(w io.Writer) (int64, error) {
			return io.Copy(w, a.Data)
		}),
	); err != nil {
		return nil, err
	}

	return buf, nil
}

// Upload uploads a document to reMarkable.
func (c *Client) Upload(ctx context.Context, args UploadArgs) error {
	url := c.createRequestURL(uploadURL)
	if ctx.Err() != nil {
		return fmt.Errorf("rmapi.Upload: %w", ctx.Err())
	}

	payload := ItemInfo{
		ID:      args.ID,
		Name:    args.Title,
		Type:    "DocumentType",
		Version: 1,
		Parent:  args.ParentID,
	}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode([]ItemInfo{payload}); err != nil {
		return fmt.Errorf("rmapi.Upload: failed to encode json payload for upload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url.String(), buf)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: unable to create request for upload: %w", err)
	}
	if err := c.setAuthHeader(ctx, req); err != nil {
		return fmt.Errorf("rmapi.Upload: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: upload http request failed: %w", err)
	}
	item, err := parseResponse(resp)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: %w", err)
	}

	data, err := args.generateData(payload.ToMetadata())
	if err != nil {
		return fmt.Errorf("rmapi.Upload: unable to generate zip file: %w", err)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, item.UploadURL, data)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: unable to create request: %w", err)
	}
	if err := func() error {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer url2epub.DrainAndClose(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("http status: %s", resp.Status)
		}
		return nil
	}(); err != nil {
		return fmt.Errorf("rmapi.Upload: unable to upload content: %w", err)
	}

	url = c.createRequestURL(updateURL)
	buf = new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode([]ItemInfo{payload}); err != nil {
		return fmt.Errorf("rmapi.Upload: failed to encode json payload for update: %w", err)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, url.String(), buf)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: unable to create request for update: %w", err)
	}
	if err := c.setAuthHeader(ctx, req); err != nil {
		return fmt.Errorf("rmapi.Upload: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: update http request failed: %w", err)
	}
	item, err = parseResponse(resp)
	if err != nil {
		return fmt.Errorf("rmapi.Upload: %w", err)
	}
	return nil
}

func parseResponse(resp *http.Response) (*ItemInfo, error) {
	defer url2epub.DrainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status: %s", resp.Status)
	}
	var data []ItemInfo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode json payload: %w", err)
	}
	if len(data) != 1 {
		return nil, fmt.Errorf("unexpected payload: %#v", data)
	}
	if err := data[0].IsSuccess(); err != nil {
		return nil, err
	}
	return &data[0], nil
}
