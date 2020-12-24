package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	imagesDir = flag.String(
		"images-dir",
		`images`,
		"Directory to download images",
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
			log.Fatal("Unsupported URL")
		}
		root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetArgs{
			URL: ampURL,
		})
		if err != nil {
			log.Fatal(err)
		}
		if !root.IsAMP() {
			log.Fatal("Unsupported URL")
		}
	}
	node, images, err := root.Readable(ctx, baseURL, "", *imagesDir)
	if err != nil {
		log.Fatal(err)
	}
	if node == nil {
		// Should not happen
		log.Fatal("Unsupported URL 2")
	}
	if err := html.Render(os.Stdout, node); err != nil {
		log.Fatal("Failed to output readable HTML:", err)
	}
	for filename, reader := range images {
		func(filename string, reader io.Reader) {
			filename = filepath.Join(*imagesDir, filename)
			f, err := os.Create(filename)
			if err != nil {
				log.Printf("Unable to write image %q: %v", filename, err)
				return
			}
			defer func() {
				if err := f.Close(); err != nil {
					log.Printf("Unable to close image file %q: %v", filename, err)
				}
			}()
			if _, err := io.Copy(f, reader); err != nil {
				log.Printf("Unable to write image %q: %v", filename, err)
			}
		}(filename, reader)
	}
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
