package devserver

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestSI_11_DevClientNonce covers the dev proxy under a strict policy: the
// injected dev client carries the nonce of the page's policy, and a page
// with no policy gets a plain script (SI-11).
func TestSI_11_DevClientNonce(t *testing.T) {
	inject := func(policy string) string {
		t.Helper()
		res := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
			Body:       io.NopCloser(strings.NewReader("<!doctype html><html><head></head><body></body></html>")),
		}
		if policy != "" {
			res.Header.Set("Content-Security-Policy", policy)
		}
		if err := (&server{}).injectDevClient(res); err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	got := inject("script-src 'nonce-q1w2e3r4t5y6u7i8o9p0aa' 'strict-dynamic'; object-src 'none'")
	want := `<script type="module" src="/_gx/dev-client.js" nonce="q1w2e3r4t5y6u7i8o9p0aa"></script></head>`
	if !strings.Contains(got, want) {
		t.Fatalf("the dev client lacks the nonce:\n%s", got)
	}
	if got := inject(""); !strings.Contains(got, `<script type="module" src="/_gx/dev-client.js"></script></head>`) {
		t.Fatalf("a page with no policy changed:\n%s", got)
	}
}
