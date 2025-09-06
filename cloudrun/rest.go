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
	"strings"
	"text/template"
	"time"

	"go.yhsif.com/ctxslog"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.yhsif.com/url2epub"
)

const (
	queryURL                  = "url"
	queryGray                 = "gray"
	queryFit                  = "fit"
	queryLang                 = "lang"
	queryPassthroughUserAgent = "passthrough-user-agent"
	queryFontSize             = "font-size"
)

const minArticleNodes = 20

func restEpubHandler(w http.ResponseWriter, r *http.Request) {
	ctx := ctxslog.Attach(
		logContext(r),
		"accountType", "rest",
	)

	url := r.FormValue(queryURL)
	ctx = ctxslog.Attach(ctx, "origUrl", url)
	gray, _ := strconv.ParseBool(r.FormValue(queryGray))
	fit64, _ := strconv.ParseInt(r.FormValue(queryFit), 10, 64)
	fit := int(fit64)
	passthroughUA, _ := strconv.ParseBool(r.FormValue(queryPassthroughUserAgent))
	userAgent := defaultUserAgent
	if passthroughUA {
		userAgent = r.Header.Get("user-agent")
		ctx = ctxslog.Attach(ctx, "userAgent", userAgent)
	}
	_, title, data, err := getEpub(ctx, url, userAgent, r.FormValue(queryLang), gray, fit, r.FormValue(queryFontSize))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set(
		"content-disposition",
		fmt.Sprintf(`attachment; filename*=UTF-8''%s.epub`, neturl.QueryEscape(title)),
	)
	w.Header().Set("content-type", url2epub.EpubMimeType)
	w.Header().Set("content-length", strconv.FormatInt(int64(data.Len()), 10))
	data.WriteTo(w)
}

var fontSizeStyleTmpl = template.Must(template.New("fontSize").Parse(`
html {
	font-size: {{.Size}};
}
`))

func fontSizeStyleNodes(size string) []*html.Node {
	if size == "" {
		return nil
	}
	node := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Style,
		Data:     atom.Style.String(),
	}
	var data strings.Builder
	fontSizeStyleTmpl.Execute(&data, struct{ Size string }{Size: size})
	node.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: data.String(),
	})
	return []*html.Node{node}
}

var errUnsupportedURL = errors.New("unsupported URL")

func getEpub(ctx context.Context, url string, ua string, lang string, gray bool, fit int, fontSize string) (id, title string, data *bytes.Buffer, err error) {
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
		URL:       url,
		UserAgent: ua,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf(
			"unable to get html for %q: %v",
			url,
			err,
		)
	}
	node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
		BaseURL:         baseURL,
		ImagesDir:       "images",
		Grayscale:       gray,
		FitImage:        fit,
		MinArticleNodes: minArticleNodes,
		ExtraHeadNodes:  fontSizeStyleNodes(fontSize),
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
		Dest:         buf,
		Title:        title,
		Author:       root.GetAuthor(),
		Node:         node,
		OverrideLang: lang,
		Images:       images,
	})
	if err != nil {
		err = fmt.Errorf("unable to create epub: %w", err)
	}
	return
}
