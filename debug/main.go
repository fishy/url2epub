package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	readableOutput = flag.Bool(
		"readable",
		false,
		"Recursively print out the readable html tree instead of the original one.",
	)
	htmlOutput = flag.Bool(
		"html",
		false,
		"Output stripped html instead, disables -readable.",
	)
	epubOutput = flag.Bool(
		"epub",
		false,
		"Output epub, disables -readable, -html.",
	)
)

func main() {
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
	if *readableOutput || *htmlOutput || *epubOutput {
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
		case *epubOutput:
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
		case *htmlOutput:
			err = html.Render(os.Stdout, node)
			if err != nil {
				log.Fatal(err)
			}
		case *readableOutput:
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
