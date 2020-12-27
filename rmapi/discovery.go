package rmapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"go.yhsif.com/url2epub"
	"go.yhsif.com/url2epub/logger"
)

const (
	discoveryURL = `https://service-manager-production-dot-remarkable-production.appspot.com/service/json/1/document-storage?environment=production&group=auth0%7C5a68dc51cb30df3877a1d7c4&apiVer=2`
	defaultHost  = `service-manager-production-dot-remarkable-production.appspot.com`
)

type discoveryPayload struct {
	Status string `json:"Status"`
	Host   string `json:"Host"`
}

var (
	host     string
	hostOnce sync.Once
)

func getHost(logger logger.Logger) string {
	hostOnce.Do(func() {
		err := func() error {
			req, err := http.NewRequest(http.MethodGet, discoveryURL, nil)
			if err != nil {
				return fmt.Errorf("rmapi.Discovery: unable to create http request: %w", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("rmapi.Discovery: http request failed: %w", err)
			}
			defer url2epub.DrainAndClose(resp.Body)
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("rmapi.Discovery: http status: %s", resp.Status)
			}
			var payload discoveryPayload
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return fmt.Errorf("rmapi.Discovery: unable to decode json: %w", err)
			}
			if payload.Status != "OK" {
				return fmt.Errorf("rmapi.Discovery: payload not ok: %#v", payload)
			}
			host = payload.Host
			return nil
		}()

		if err != nil {
			logger.Log(err.Error())
			host = defaultHost
		}
	})
	return host
}

var baseURL = parseURLMust("https://" + defaultHost + "/")

func (c *Client) createRequestURL(path string) *url.URL {
	u := new(url.URL)
	*u = *baseURL
	u.Host = getHost(c.Logger)
	u.Path = path
	return u
}

func parseURLMust(src string) *url.URL {
	u, err := url.Parse(src)
	if err != nil {
		panic(err)
	}
	return u
}
