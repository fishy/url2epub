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
		"",
		"Destination URL for the HTTP GET request",
	)
	htmlOutput = flag.Bool(
		"html",
		false,
		"Output stripped html instead",
	)
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	root, baseURL, err := url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
		URL: *url,
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
	if *htmlOutput {
		if !root.IsAMP() {
			root, baseURL, err = url2epub.GetHTML(ctx, url2epub.GetHTMLArgs{
				URL: root.GetAMPurl(),
			})
			if err != nil {
				log.Fatal(err)
			}
		}
		if !root.IsAMP() {
			log.Fatal("Not AMP")
		}
		node, _, err := root.Readable(ctx, url2epub.ReadableArgs{
			BaseURL: baseURL,
		})
		if err != nil {
			log.Fatal(err)
		}
		err = html.Render(os.Stdout, node)
		if err != nil {
			log.Fatal(err)
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
		recursivePrint(c, prefix+"| ")
		return true
	})
}
