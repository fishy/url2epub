package rmapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"go.yhsif.com/url2epub"
	"go.yhsif.com/url2epub/logger"
)

const (
	registerURL = `https://webapp-production-dot-remarkable-production.appspot.com/token/json/2/device/new`
	refreshURL  = `https://webapp-production-dot-remarkable-production.appspot.com/token/json/2/user/new`
)

// RegisterArgs defines args to be used with Register.
type RegisterArgs struct {
	// A token got from either https://my.remarkable.com/device/desktop/connect or
	// https://my.remarkable.com/device/mobile/connect, usually with length of 8.
	Token string

	// A description of this device, usually something like "desktop-linux",
	// "mobile-android".
	Description string
}

type registerPayload struct {
	Token       string `json:"code"`
	Description string `json:"deviceDesc"`
	ID          string `json:"deviceID"`
}

// Register registers a new token with reMarkable API server.
//
// Upon success, it returns a *Client with RefreshToken set.
func Register(ctx context.Context, args RegisterArgs) (*Client, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("rmapi.Register: unable to generate uuid: %w", err)
	}
	data := registerPayload{
		Token:       args.Token,
		Description: args.Description,
		ID:          id.String(),
	}
	payload := new(bytes.Buffer)
	if err := json.NewEncoder(payload).Encode(data); err != nil {
		return nil, fmt.Errorf("rmapi.Register: unable to encode json payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, payload)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Register: unable to create http request: %w", err)
	}
	refresh, err := readToken(req, 1024)
	if err != nil {
		return nil, fmt.Errorf("rmapi.Register: %w", err)
	}
	return &Client{
		RefreshToken: refresh,
	}, nil
}

// Client defines a reMarkable API client.
//
// You can get it either by using Register with a new token, or construct
// directly from refresh token stored previously.
type Client struct {
	RefreshToken string

	Logger logger.Logger

	token string
}

// Refresh refreshes the token.
func (c *Client) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return fmt.Errorf("rmapi.Refresh: unable to create http request: %w", err)
	}
	req.Header.Set(
		"authorization",
		"Bearer "+c.RefreshToken,
	)
	token, err := readToken(req, 4096)
	if err != nil {
		return fmt.Errorf("rmapi.Refresh: %w", err)
	}
	c.token = token
	return nil
}

// AutoRefresh refreshes the token when needed.
func (c *Client) AutoRefresh(ctx context.Context) error {
	if c.token != "" {
		return nil
	}
	return c.Refresh(ctx)
}

func readToken(req *http.Request, size int) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer url2epub.DrainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status: %s", resp.Status)
	}
	buf := make([]byte, size)
	n, err := io.ReadFull(resp.Body, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", fmt.Errorf("unable to read response: %w", err)
	}
	return string(buf[:n]), nil
}

func (c *Client) setAuthHeader(ctx context.Context, req *http.Request) error {
	if err := c.AutoRefresh(ctx); err != nil {
		return err
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set("authorization", "Bearer "+c.token)
	return nil
}
