package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"golang.org/x/exp/slog"
)

const (
	mgUser = "api"
	mgURL  = "https://api.mailgun.net/v3/%s/messages"
)

var httpClient http.Client

func sendEmail(ctx context.Context, email string, title string, epub io.Reader, chatID int64) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("to", email); err != nil {
		return fmt.Errorf("sendEmail: failed to write multipart to: %w", err)
	}
	if err := writer.WriteField("from", mgFrom()); err != nil {
		return fmt.Errorf("sendEmail: failed to write multipart from: %w", err)
	}
	if err := writer.WriteField("subject", title); err != nil {
		return fmt.Errorf("sendEmail: failed to write multipart subject: %w", err)
	}
	if err := writer.WriteField("text", title); err != nil {
		return fmt.Errorf("sendEmail: failed to write multipart text: %w", err)
	}
	for _, tag := range []string{
		"url2epub",
		fmt.Sprintf("chat-%d", chatID),
	} {
		if err := writer.WriteField("tag", tag); err != nil {
			return fmt.Errorf("sendEmail: failed to write multipart tag %q: %w", tag, err)
		}
	}

	w, err := writer.CreateFormFile("attachment", title+".epub")
	if err != nil {
		return fmt.Errorf("sendEmail: failed to create form file: %w", err)
	}
	if _, err := io.Copy(w, epub); err != nil {
		return fmt.Errorf("sendEmail: failed to copy form file: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("sendEmail: failled to close multipart body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf(mgURL, os.Getenv("MAILGUN_DOMAIN")),
		&buf,
	)
	if err != nil {
		return fmt.Errorf("sendEmail: failed to create request: %w", err)
	}
	req.SetBasicAuth(mgUser, os.Getenv("SECRET_MAILGUN_TOKEN"))
	req.Header.Set(
		"Content-Type",
		writer.FormDataContentType(),
	)
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.ErrorContext(ctx, "sendEmail: http request failed", "err", err)
		return err
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	body, e := io.ReadAll(io.LimitReader(resp.Body, 2048))
	body = maybeDecodeBase64Body(body)
	slog.DebugContext(
		ctx,
		"sendEmail mailgun response",
		"code", resp.StatusCode,
		"err", e,
		"body", body,
	)
	if resp.StatusCode < 400 {
		return nil
	}

	return fmt.Errorf(
		"sendEmail: mailgun returned non-200 code: %d, body: %q",
		resp.StatusCode,
		body,
	)
}

func mgFrom() string {
	return os.Getenv("MAILGUN_FROM")
}

func maybeDecodeBase64Body(body []byte) []byte {
	// Mailgun seems to like to return base64 encoded json body, try decode it,
	// but return body as-is if it's not base64 encoded
	dst := make([]byte, base64.StdEncoding.DecodedLen(len(body)))
	if n, err := base64.StdEncoding.Decode(dst, body); err == nil {
		return dst[:n]
	}
	return body
}
