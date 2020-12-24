package url2epub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type lastURLKeyType struct{}

var lastURLKey lastURLKeyType

var client = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			// Copied from:
			// https://go.googlesource.com/go/+/go1.15.6/src/net/http/client.go#805
			return errors.New("stopped after 10 redirects")
		}
		value := req.Context().Value(lastURLKey)
		if ptr, ok := value.(**url.URL); ok {
			*ptr = req.URL
		}
		return nil
	},
}

// ChromeAndroidUserAgent is an User-Agent copied from Chrome Android.
const ChromeAndroidUserAgent = `Mozilla/5.0 (Linux; Android 11; Pixel 4 XL) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.51 Mobile Safari/537.36`

// GetArgs define the arguments used by Get function.
type GetArgs struct {
	// The HTTP GET URL, required.
	URL string

	// The User-Agent header to use, optional.
	//
	// ChromeAndroidUserAgent will be used if this is empty.
	UserAgent string
}

// GetHTML does HTTP get requests on HTML content.
//
// It's different from standard http.Get in the following ways:
//
// - If there are redirects happening during the request, returned URL will be
// the URL of the last (final) request.
//
// - By default it uses an User-Agent copied from Chrome Android version, to
// get the best chance of discovering the AMP version of the URL. This can be
// overridden by the UserAgent arg.
//
// - Instead of returning *http.Response, it returns parsed *html.Node, with
// Type being ElementNode and DataAtom being Html (instead of root node, which
// is usually DoctypeNode).
//
// - The client used by Get does not have timeout set. It's expected that a
// deadline is set in the ctx passed in.
func GetHTML(ctx context.Context, args GetArgs) (*Node, *url.URL, error) {
	src, err := url.Parse(args.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse url %q: %w", args.URL, err)
	}

	body, lastURL, err := get(ctx, src, args.UserAgent)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get %q: %w", args.URL, err)
	}
	defer DrainAndClose(body)
	root, err := html.Parse(body)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse %q: %w", lastURL, err)
	}
	return FromNode(root).FindFirstAtomNode(atom.Html), lastURL, nil
}

// DrainAndClose drains and closes r.
func DrainAndClose(r io.ReadCloser) error {
	io.Copy(ioutil.Discard, r)
	return r.Close()
}

func get(ctx context.Context, src *url.URL, ua string) (io.ReadCloser, *url.URL, error) {
	req := &http.Request{
		Method: http.MethodGet,
		URL:    src,
		Header: make(http.Header),
	}

	lastURL := new(*url.URL)
	*lastURL = src
	ctx = context.WithValue(ctx, lastURLKey, lastURL)
	req = req.WithContext(ctx)
	if ua == "" {
		ua = ChromeAndroidUserAgent
	}
	req.Header.Set("user-agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		DrainAndClose(resp.Body)
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return resp.Body, *lastURL, nil
}
