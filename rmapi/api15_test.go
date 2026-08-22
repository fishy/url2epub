package rmapi_test

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yhsif.com/url2epub/rmapi"
)

func TestAPIResponseUnmarshalJSON(t *testing.T) {
	const rawJSON = `{
	"relative_path": "pathA",
	"url": "urlB",
	"method": "methodC",
	"expires": "2006-01-02T15:04:05Z",
	"maxuploadsize_bytes": 9223372036854775807,

	"header1": "value1",
	"header2": "value2"
}`
	want := rmapi.APIResponse{
		Path:       "pathA",
		URL:        "urlB",
		Method:     "methodC",
		RawExpires: "2006-01-02T15:04:05Z",
		Expires:    time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),

		Headers: map[string]string{
			"header1": "value1",
			"header2": "value2",

			"x-goog-content-length-range": "0,9223372036854775807",
		},
	}

	var got rmapi.APIResponse
	if err := json.UnmarshalRead(strings.NewReader(rawJSON), &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got.Path != want.Path {
		t.Errorf("Path: got %q, want %q", got.Path, want.Path)
	}
	if got.URL != want.URL {
		t.Errorf("URL: got %q, want %q", got.URL, want.URL)
	}
	if got.Method != want.Method {
		t.Errorf("Method: got %q, want %q", got.Method, want.Method)
	}
	if got.RawExpires != want.RawExpires {
		t.Errorf("RawExpires: got %q, want %q", got.RawExpires, want.RawExpires)
	}
	if diff := got.Expires.Sub(want.Expires); diff != 0 {
		t.Errorf("Expires: got %v, want %v (diff: %v)", got.Expires, want.Expires, diff)
	}
	if !reflect.DeepEqual(got.Headers, want.Headers) {
		t.Errorf("Headers: got %#v, want %#v", got.Headers, want.Headers)
	}
}
