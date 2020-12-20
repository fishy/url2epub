package main

import (
	"context"
	"flag"
	"fmt"
	"time"

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
		"http://github.com",
		"Destination URL for the HTTP GET request",
	)
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	resp, url, err := url2epub.Get(ctx, url2epub.GetArgs{
		URL: *url,
	})
	fmt.Println("Got:", resp, url, err)
}
