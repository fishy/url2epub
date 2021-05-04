package url2epub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"go.yhsif.com/url2epub/birds"
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

// GetHTMLArgs define the arguments used by GetHTML function.
type GetHTMLArgs struct {
	// The HTTP GET URL, required.
	URL string

	// The User-Agent header to use, optional.
	UserAgent string

	// The bearer token for the twitter client.
	// If non-empty and the URL is a twitter URL,
	// it uses Twitter API to get the thread instead of the raw HTML.
	TwitterBearer string
}

var twitterRE = regexp.MustCompile(`^https://twitter.com/.*/status/(\d*)$`)

// GetHTML does HTTP get requests on HTML content.
//
// It's different from standard http.Get in the following ways:
//
// - If there are redirects happening during the request, returned URL will be
// the URL of the last (final) request.
//
// - Instead of returning *http.Response, it returns parsed *html.Node, with
// Type being ElementNode and DataAtom being Html (instead of root node, which
// is usually DoctypeNode).
//
// - The client used by Get does not have timeout set. It's expected that a
// deadline is set in the ctx passed in.
func GetHTML(ctx context.Context, args GetHTMLArgs) (*Node, *url.URL, error) {
	src, err := url.Parse(args.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse url %q: %w", args.URL, err)
	}

	var reader io.Reader
	if args.TwitterBearer != "" {
		if matches := twitterRE.FindStringSubmatch(args.URL); len(matches) > 0 {
			s := birds.Session{
				Bearer:    args.TwitterBearer,
				UserAgent: args.UserAgent,
			}
			text, err := s.Thread(ctx, matches[1])
			if err != nil {
				return nil, nil, fmt.Errorf("unable to get twitter thread %q: %w", args.URL, err)
			}
			reader = strings.NewReader(text)
		}
	} else {
		body, lastURL, err := get(ctx, src, args.UserAgent)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get %q: %w", args.URL, err)
		}
		defer DrainAndClose(body)
		reader = body
		src = lastURL
	}
	root, err := html.Parse(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse %q: %w", src, err)
	}
	return FromNode(root).FindFirstAtomNode(atom.Html), src, nil
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
	if ua != "" {
		req.Header.Set("user-agent", ua)
	}

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
