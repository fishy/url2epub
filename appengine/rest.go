package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"go.yhsif.com/url2epub"
)

func restEpubHandler(w http.ResponseWriter, r *http.Request) {
	ctx := logContext(r)

	url := r.FormValue("url")
	gray, _ := strconv.ParseBool(r.FormValue("gray"))
	_, title, data, err := getEpub(r.Context(), url, r.Header.Get("user-agent"), gray)
	if err != nil {
		l(ctx).Errorw(
			"getEpub failed",
			"err", err,
		)
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

const defaultUserAgent = "url2epub (https://github.com/fishy/url2epub)"

func getEpub(ctx context.Context, url string, ua string, gray bool) (id, title string, data *bytes.Buffer, err error) {
	if ua == "" {
		ua = defaultUserAgent
	}

	ctx, cancel := context.WithTimeout(ctx, epubTimeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
		URL:       url,
		UserAgent: ua,
	})
	if err != nil {
		return "", "", nil, err
	}
	if !root.IsAMP() {
		if ampURL := root.GetAMPurl(); ampURL != "" {
			root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
				URL:       ampURL,
				UserAgent: ua,
			})
			if err != nil {
				return "", "", nil, err
			}
		}
	}
	if !root.IsAMP() {
		l(ctx).Infow(
			"Generating epub from non-amp url",
			"url", baseURL.String(),
		)
	}
	node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
		BaseURL:   baseURL,
		ImagesDir: "images",
		Grayscale: gray,
		Logger: func(msg string) {
			l(ctx).Error(msg)
		},
	})
	if err != nil {
		return "", "", nil, err
	}
	if node == nil {
		// Should not happen
		return "", "", nil, errUnsupportedURL
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
	return
}
