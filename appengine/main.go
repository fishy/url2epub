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
		"Timeout for the HTTP GET request",
	)
	url = flag.String(
		"url",
		`https://www.theverge.com/22158504/best-games-2020-ps5-xbox-nintendo-tlou2-animal-crossing-miles-morales`,
		"Destination URL for the HTTP GET request",
	)
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetArgs{
		URL: *url,
	})
	if err != nil {
		log.Fatal(err)
	}
	if !root.IsAMP() {
		ampURL := root.GetAMPurl()
		if ampURL == "" {
			log.Fatalf("Unsupported URL: %q", baseURL)
		}
		root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetArgs{
			URL: ampURL,
		})
		if err != nil {
			log.Fatal(err)
		}
		if !root.IsAMP() {
			log.Fatal("Unsupported URL: %q", baseURL)
		}
	}
	node, images, err := root.Readable(ctx, url2epub.ReadableArgs{
		BaseURL:   baseURL,
		ImagesDir: "images",
	})
	if err != nil {
		log.Fatal(err)
	}
	if node == nil {
		// Should not happen
		log.Fatal("Unsupported URL 2")
	}
	log.Print(url2epub.Epub(url2epub.EpubArgs{
		Dest:   os.Stdout,
		Title:  root.GetTitle(),
		Node:   node,
		Images: images,
	}))
}

func recursivePrint(n *url2epub.Node, prefix string) {
	if n == nil {
		return
	}
	node := n.AsNode()
	if node.Type == html.ElementNode {
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
		recursivePrint(c, prefix+"  ")
		return true
	})
}
