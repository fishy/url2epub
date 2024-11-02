package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.yhsif.com/ctxslog"
	"go.yhsif.com/flagutils"
	"golang.org/x/net/html"

	"go.yhsif.com/url2epub"

	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

var (
	timeout = flag.Duration(
		"timeout",
		time.Second,
		"Timeout for the HTTP GET request.",
	)
	url = flag.String(
		"url",
		"",
		"Destination URL for the HTTP GET request.",
	)
	ua = flag.String(
		"ua",
		"",
		"User-Agent to use",
	)
	grayscale = flag.Bool(
		"gray",
		false,
		"Grayscale images.",
	)
	fit = flag.Int(
		"fit",
		0,
		"Downscale images to fit",
	)
	minArticleNodes = flag.Int(
		"min-article-nodes",
		0,
		"Minimal nodes to use article node",
	)
)

func main() {
	var (
		readableOutput flagutils.OneOf
		htmlOutput     flagutils.OneOf
		epubOutput     flagutils.OneOf
	)

	flagutils.GroupOneOf(&readableOutput, &htmlOutput, &epubOutput)
	flag.Var(
		&readableOutput,
		"readable",
		"Recursively print out the readable html tree instead of the original one.",
	)
	flag.Var(
		&htmlOutput,
		"html",
		"Output stripped html instead.",
	)
	flag.Var(
		&epubOutput,
		"epub",
		"Output epub.",
	)
	flag.Parse()

	slog.SetDefault(slog.New(ctxslog.ContextHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
		URL:       *url,
		UserAgent: *ua,
	})
	if err != nil {
		slog.Error("url2epub.GetHTML failed", "err", err)
		os.Exit(1)
	}
	slog.Debug(
		"GetHTML returned",
		"baseURL", baseURL.String(),
		"amp", root.IsAMP(),
		"ampURL", root.GetAMPurl(),
	)
	if readableOutput.Bool || htmlOutput.Bool || epubOutput.Bool {
		if !root.IsAMP() {
			ampURL := root.GetAMPurl()
			if ampURL != "" {
				root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
					URL:       ampURL,
					UserAgent: *ua,
				})
				if err != nil {
					slog.Error("url2epub.GetHTML failed", "err", err)
					os.Exit(1)
				}
			}
		}
		if !root.IsAMP() {
			slog.Debug("Not AMP")
		}

		node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
			BaseURL:         baseURL,
			ImagesDir:       "images",
			UserAgent:       *ua,
			Grayscale:       *grayscale,
			FitImage:        *fit,
			MinArticleNodes: *minArticleNodes,
		})
		if err != nil {
			slog.Error("url2epub.Readable failed", "err", err)
			os.Exit(1)
		}

		switch {
		case epubOutput.Bool:
			id, err := url2epub.Epub(url2epub.EpubArgs{
				Dest:   os.Stdout,
				Title:  root.GetTitle(),
				Node:   node,
				Images: images,
			})
			if err != nil {
				slog.Error("url2epub.Epub failed", "err", err)
				os.Exit(1)
			}
			slog.Info("epub generated", "id", id)

		case htmlOutput.Bool:
			err = html.Render(os.Stdout, node)
			if err != nil {
				slog.Error("html.Render failed", "err", err)
				os.Exit(1)
			}

		case readableOutput.Bool:
			recursivePrint(url2epub.FromNode(node), "")
		}
	} else {
		recursivePrint(root, "")
	}
}

func recursivePrint(n *url2epub.Node, prefix string) {
	if n == nil {
		return
	}
	node := n.AsNode()
	switch node.Type {
	case html.TextNode:
		text := []rune(node.Data)
		if len(text) > 10 {
			text = append(text[:10], []rune("...")...)
		}
		fmt.Printf("%s[text: %q]\n", prefix, string(text))
	case html.ElementNode:
		var sb strings.Builder
		sb.WriteString(prefix)
		sb.WriteString(node.Data)
		sb.WriteString(fmt.Sprintf(" atom=%v", node.DataAtom))
		sb.WriteString(" attributes=[")
		for i, attr := range node.Attr {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q:%q", attr.Key, attr.Val))
		}
		sb.WriteString("]")
		fmt.Println(sb.String())
	}
	for c := range n.Children() {
		recursivePrint(c, prefix+"| ")
	}
}
