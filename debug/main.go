package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	flagutils "github.com/fishy/go-flagutils"
	"golang.org/x/net/html"

	"github.com/fishy/url2epub"
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
		URL:       *url,
		UserAgent: *ua,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf(
		`base url: "%v", amp: %v, amp url: "%s"`,
		baseURL,
		root.IsAMP(),
		root.GetAMPurl(),
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
					log.Fatal(err)
				}
			}
		}
		if !root.IsAMP() {
			log.Print("Not AMP")
		}

		node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
			BaseURL:   baseURL,
			ImagesDir: "images",
			UserAgent: *ua,
		})
		if err != nil {
			log.Fatal(err)
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
				log.Fatal(err)
			}
			log.Printf("epub id: %v", id)

		case htmlOutput.Bool:
			err = html.Render(os.Stdout, node)
			if err != nil {
				log.Fatal(err)
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
	n.ForEachChild(func(c *url2epub.Node) bool {
		recursivePrint(c, prefix+"| ")
		return true
	})
}
