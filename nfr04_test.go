package gx_test

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// nfr04Adapter marks its runtime script, so a test can see which scripts a
// page ships (NFR-04).
type nfr04Adapter struct{ fakeAdapter }

func (nfr04Adapter) Runtime() gx.Node {
	return gx.El("script", gx.Attrs{
		{Key: "type", Value: "module"},
		{Key: "src", Value: "/_gx/nfr04-adapter.js"},
		{Key: "data-gx-adapter", Value: "nfr04"},
	})
}

func (nfr04Adapter) Assets() map[string][]byte {
	return map[string][]byte{"nfr04-adapter.js": []byte("// nfr04 adapter")}
}

// nfr04Route is a hand-written page input; generated route types provide the
// same Pattern and Bind methods.
type nfr04Route struct{}

func (nfr04Route) Pattern() string          { return "GET /nfr04" }
func (nfr04Route) Bind(*http.Request) error { return nil }

// nfr04Serve renders one page through an app with an adapter (NFR-04).
func nfr04Serve(t *testing.T, view func() gx.Node) string {
	t.Helper()
	return servePage(t, &nfr04Adapter{}, view)
}

// servePage renders one page through an app with the adapter (NFR-04).
func servePage(t *testing.T, adapter gx.Adapter, view func() gx.Node) string {
	t.Helper()
	page := gx.Page(func(*gx.Ctx, nfr04Route) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return view() })
	app := gx.New(gx.Config{Adapter: adapter})
	app.Group("/", gx.Collect(page))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/nfr04", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestNFR_04_ZeroJSWithoutFeatures checks that a page with no signals,
// islands, actions or morph navigation ships zero JS (NFR-04).
func TestNFR_04_ZeroJSWithoutFeatures(t *testing.T) {
	body := nfr04Serve(t, func() gx.Node {
		return gx.El("h1", nil, gx.Text("Plain page"))
	})
	if strings.Contains(body, "<script") {
		t.Fatalf("plain page ships JS:\n%s", body)
	}
	if !strings.Contains(body, "<h1>Plain page</h1>") {
		t.Fatalf("plain page lost its content:\n%s", body)
	}
}

// TestNFR_04_FeatureScripts checks that signals and actions pull the adapter
// runtime, and that forms, navigation and behaviours pull the Gx runtime
// (NFR-04).
func TestNFR_04_FeatureScripts(t *testing.T) {
	tests := []struct {
		name     string
		view     func() gx.Node
		core     bool
		adapter  bool
		behavior bool
	}{
		{"signals", func() gx.Node {
			return gx.El("input", gx.Attrs{
				{Key: "data-signals", Value: "{}"},
				{Key: "data-bind", Value: "$q"},
			})
		}, true, true, false},
		{"form", func() gx.Node {
			return gx.El("form", gx.Attrs{{Key: "data-gx-form", Value: "signup"}})
		}, true, true, false},
		{"navigation", func() gx.Node {
			return gx.El("div", gx.Attrs{{Key: "data-gx-slot", Value: "page"}})
		}, true, true, false},
		{"page-shell behaviour", func() gx.Node {
			return gx.El("div", gx.Attrs{{Key: "data-gx-theme", Value: "dark"}})
		}, true, false, false},
		{"component behaviour", func() gx.Node {
			return gx.El("div", gx.Attrs{{Key: "data-gx-roving", Value: ""}})
		}, false, false, true},
		{"tabs behaviour", func() gx.Node {
			return gx.El("div", gx.Attrs{{Key: "data-gx-tabs", Value: ""}})
		}, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := nfr04Serve(t, tt.view)
			if got := strings.Contains(body, "/_gx/gx.js"); got != tt.core {
				t.Fatalf("core runtime = %v, want %v:\n%s", got, tt.core, body)
			}
			if got := strings.Contains(body, "/_gx/nfr04-adapter.js"); got != tt.adapter {
				t.Fatalf("adapter runtime = %v, want %v:\n%s", got, tt.adapter, body)
			}
			if got := strings.Contains(body, "/_gx/behavior.js"); got != tt.behavior {
				t.Fatalf("behaviour runtime = %v, want %v:\n%s", got, tt.behavior, body)
			}
		})
	}
}

// TestNFR_04_CoreRuntimeBudget checks the 6 KB gzipped budget of the Gx core
// runtime, adapter excluded (NFR-04).
func TestNFR_04_CoreRuntimeBudget(t *testing.T) {
	data, err := os.ReadFile("runtime/js/gx.js")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	const budget = 6 * 1024
	if buf.Len() > budget {
		t.Fatalf("core runtime = %d bytes gzipped, budget %d", buf.Len(), budget)
	}
	t.Logf("core runtime: %d bytes gzipped (budget %d)", buf.Len(), budget)
}
