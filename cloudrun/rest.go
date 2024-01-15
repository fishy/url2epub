package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	neturl "net/url"
	"strconv"
	"time"

	"go.yhsif.com/url2epub"
)

const (
	queryURL                  = "url"
	queryGray                 = "gray"
	queryFit                  = "fit"
	queryPassthroughUserAgent = "passthrough-user-agent"
)

const minArticleNodes = 20

func restEpubHandler(w http.ResponseWriter, r *http.Request) {
	ctx := logContext(r)

	url := r.FormValue(queryURL)
	gray, _ := strconv.ParseBool(r.FormValue(queryGray))
	fit64, _ := strconv.ParseInt(r.FormValue(queryFit), 10, 64)
	fit := int(fit64)
	passthroughUA, _ := strconv.ParseBool(r.FormValue(queryPassthroughUserAgent))
	userAgent := defaultUserAgent
	if passthroughUA {
		userAgent = r.Header.Get("user-agent")
	}
	_, title, data, err := getEpub(ctx, url, userAgent, gray, fit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set(
		"content-disposition",
		fmt.Sprintf(`attachment; filename="%s.epub"`, title),
	)
	w.Header().Set("content-type", url2epub.EpubMimeType)
	w.Header().Set("content-length", strconv.FormatInt(int64(data.Len()), 10))
	data.WriteTo(w)
}

var errUnsupportedURL = errors.New("unsupported URL")

func getEpub(ctx context.Context, url string, ua string, gray bool, fit int) (id, title string, data *bytes.Buffer, err error) {
	if ua == "" {
		ua = defaultUserAgent
	}

	defer func(start time.Time) {
		args := []any{
			slog.Duration("took", time.Since(start)),
			slog.String("url", url),
			slog.String("ua", ua),
		}
		level := slog.LevelDebug
		if err != nil {
			args = append(
				args,
				slog.Any("err", err),
			)
			level = slog.LevelError
		} else {
			args = append(
				args,
				slog.String("id", id),
				slog.String("title", title),
				slog.Int("size", data.Len()),
			)
		}
		slog.Log(ctx, level, "getEpub finished", args...)
	}(time.Now())

	ctx, cancel := context.WithTimeout(ctx, epubTimeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
		URL:           url,
		UserAgent:     ua,
		TwitterBearer: getTwitterBearer(),
	})
	if err != nil {
		return "", "", nil, fmt.Errorf(
			"unable to get html for %q: %v",
			url,
			err,
		)
	}
	if !root.IsAMP() {
		if ampURL := root.GetAMPurl(); ampURL != "" {
			// ampURL could be relative, resolve to full url
			u, err := neturl.Parse(ampURL)
			if err != nil {
				return "", "", nil, fmt.Errorf(
					"unable to parse amp url %q: %w",
					ampURL,
					err,
				)
			}
			ampURL = baseURL.ResolveReference(u).String()
			root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
				URL:       ampURL,
				UserAgent: ua,
			})
			if err != nil {
				return "", "", nil, fmt.Errorf(
					"unable to get amp html for %q: %v",
					ampURL,
					err,
				)
			}
		}
	}
	if !root.IsAMP() {
		slog.InfoContext(
			ctx,
			"Generating epub from non-amp url",
			"url", baseURL.String(),
		)
	}
	node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
		BaseURL:         baseURL,
		ImagesDir:       "images",
		Grayscale:       gray,
		FitImage:        fit,
		MinArticleNodes: minArticleNodes,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf(
			"unable to generate readable html: %w",
			err,
		)
	}
	if node == nil {
		// Should not happen
		return "", "", nil, fmt.Errorf(
			"%w: %q",
			errUnsupportedURL,
			url,
		)
	}

	buf := new(bytes.Buffer)
	data = buf
	title = root.GetTitle()
	id, err = url2epub.Epub(url2epub.EpubArgs{
		Dest:   buf,
		Title:  title,
		Node:   node,
		Images: images,
	})
	if err != nil {
		err = fmt.Errorf("unable to create epub: %w", err)
	}
	return
}
