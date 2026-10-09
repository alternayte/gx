package gx_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/textproto"
	"slices"
	"testing"

	"github.com/alternayte/gx"
)

// hintsPlainRoute is the input of a page with no script.
type hintsPlainRoute struct{}

func (hintsPlainRoute) Pattern() string          { return "GET /home" }
func (hintsPlainRoute) Bind(*http.Request) error { return nil }

// hintsApp is an app with a stylesheet, one page that binds a signal and one
// plain page.
func hintsApp(t *testing.T) *gx.App {
	t.Helper()
	gx.SetStylesheet([]byte("p{color:red}"))
	t.Cleanup(func() { gx.SetStylesheet(nil) })
	live := gx.Page(func(*gx.Ctx, nfr04Route) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node {
			return gx.El("input", gx.Attrs{{Key: "data-bind", Value: "qty"}})
		})
	plain := gx.Page(func(*gx.Ctx, hintsPlainRoute) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return gx.El("h1", nil, gx.Text("Plain page")) })
	app := gx.New(gx.Config{Adapter: &nfr04Adapter{}})
	app.Group("/", gx.Collect(live, plain))
	return app
}

// TestNFR_04_LinkHeaders checks that a page names its stylesheet and its
// scripts in Link headers, so a CDN can send them as early hints, and that a
// plain page names no script (NFR-04).
func TestNFR_04_LinkHeaders(t *testing.T) {
	app := hintsApp(t)
	get := func(path string, header ...string) []string {
		req := httptest.NewRequest("GET", path, nil)
		for i := 0; i+1 < len(header); i += 2 {
			req.Header.Set(header[i], header[i+1])
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		return rec.Header().Values("Link")
	}
	want := []string{
		"</_gx/app.css>; rel=preload; as=style",
		"</_gx/gx.js>; rel=modulepreload",
		"</_gx/nfr04-adapter.js>; rel=modulepreload",
	}
	if got := get("/nfr04"); !slices.Equal(got, want) {
		t.Errorf("page with a signal: Link = %q, want %q", got, want)
	}
	if got, want := get("/home"), []string{"</_gx/app.css>; rel=preload; as=style"}; !slices.Equal(got, want) {
		t.Errorf("plain page: Link = %q, want %q", got, want)
	}
}

// TestNFR_04_LinkHeadersUnderCSP checks that a page with a nonce names no
// script in a Link header: a preload from a header has no nonce, and a strict
// policy refuses it (SI-11).
func TestNFR_04_LinkHeadersUnderCSP(t *testing.T) {
	app := hintsApp(t)
	rec := httptest.NewRecorder()
	gx.CSP(gx.CSPOptions{})(app).ServeHTTP(rec, httptest.NewRequest("GET", "/nfr04", nil))
	want := []string{"</_gx/app.css>; rel=preload; as=style"}
	if got := rec.Header().Values("Link"); !slices.Equal(got, want) {
		t.Errorf("Link = %q, want %q", got, want)
	}
}

// TestNFR_04_EarlyHints checks the 103 answer of a page over HTTP/2: the
// second request of a route gets, before its loader ends, the files that the
// first answer of the route named. HTTP/1.1 gets no 103: an old client can
// read it as the answer.
func TestNFR_04_EarlyHints(t *testing.T) {
	app := hintsApp(t)
	hinted := func(srv *httptest.Server, path string) []string {
		var links []string
		trace := &httptrace.ClientTrace{Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			if code == http.StatusEarlyHints {
				links = append(links, header["Link"]...)
			}
			return nil
		}}
		req, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), "GET", srv.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		return links
	}

	h2 := httptest.NewUnstartedServer(app)
	h2.EnableHTTP2 = true
	h2.StartTLS()
	defer h2.Close()
	if got := hinted(h2, "/nfr04"); len(got) != 0 {
		t.Errorf("the first request of a route got the hints %q; the app does not know the files yet", got)
	}
	want := []string{
		"</_gx/app.css>; rel=preload; as=style",
		"</_gx/gx.js>; rel=modulepreload",
		"</_gx/nfr04-adapter.js>; rel=modulepreload",
	}
	if got := hinted(h2, "/nfr04"); !slices.Equal(got, want) {
		t.Errorf("second request over HTTP/2: hints %q, want %q", got, want)
	}
	if got, want := hinted(h2, "/home"), []string(nil); !slices.Equal(got, want) {
		t.Errorf("first request of a different route: hints %q, want none", got)
	}

	h1 := httptest.NewServer(app)
	defer h1.Close()
	hinted(h1, "/nfr04")
	if got := hinted(h1, "/nfr04"); len(got) != 0 {
		t.Errorf("HTTP/1.1 got the hints %q, want none", got)
	}
}
