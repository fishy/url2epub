package url2epub

import (
	"context"
	"errors"
	"net/http"
	"net/url"
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

// Get does HTTP get requests.
//
// It's different from standard http.Get in the following ways:
//
// 1. If there are redirects happening during the request, returned URL will be
// the URL of the last (final) request.
//
// 2. By default it uses an User-Agent copied from Chrome Android version, to
// get the best chance of discovering the AMP version of the URL. This can be
// overridden by the UserAgent arg.
//
// 3. The client used by Get does not have timeout set. It's expected that a
// deadline is set in the ctx passed in.
func Get(ctx context.Context, args GetArgs) (*http.Response, *url.URL, error) {
	if args.UserAgent == "" {
		args.UserAgent = ChromeAndroidUserAgent
	}

	req, err := http.NewRequest(http.MethodGet, args.URL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("user-agent", args.UserAgent)

	lastURL := req.URL
	ctx = context.WithValue(ctx, lastURLKey, &req.URL)
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	value := req.Context().Value(lastURLKey)
	if ptr, _ := value.(**url.URL); ptr != nil {
		lastURL = *ptr
	}
	return resp, lastURL, err
}
