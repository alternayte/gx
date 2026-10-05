package gx_test

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// behaviorModules are the modules of the component behaviour runtime
// (REQ-REG-07).
var behaviorModules = []string{"behavior", "tabs", "toast", "overlay"}

// gzipLen returns the gzipped size of a script.
func gzipLen(t *testing.T, data []byte) int {
	t.Helper()
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
	return buf.Len()
}

var moduleScript = regexp.MustCompile(`<script type="module" src="/_gx/([a-z]+)\.js">`)

// modulesOf returns the behaviour modules that a page loads, in the order
// of its script tags.
func modulesOf(body string) []string {
	var out []string
	for _, m := range moduleScript.FindAllStringSubmatch(body, -1) {
		if slices.Contains(behaviorModules, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// TestREQ_REG_07_ModuleBudgets checks the 2 KB gzipped budget of each
// behaviour module (REQ-REG-07). The module budgets and the page budget of
// TestNFR_04_HeaviestBehaviorPage replace the earlier budget of 4 KB for
// one file: one file under 4 KB cannot hold sub-menus and collision
// handling, and a page now loads only the modules its markup uses.
func TestREQ_REG_07_ModuleBudgets(t *testing.T) {
	const budget = 2 * 1024
	for _, module := range behaviorModules {
		data, err := os.ReadFile("runtime/js/" + module + ".js")
		if err != nil {
			t.Fatal(err)
		}
		size := gzipLen(t, data)
		if size > budget {
			t.Errorf("%s module = %d bytes gzipped, budget %d", module, size, budget)
		}
		t.Logf("%s module: %d bytes gzipped (budget %d)", module, size, budget)
	}
}

// TestNFR_04_HeaviestBehaviorPage checks the 6 KB gzipped budget of the
// behaviour modules of the heaviest page: a menu with a sub-menu, tabs and
// the toaster (NFR-04, REQ-REG-07). The test measures the scripts that the
// app serves for the script tags of that page.
func TestNFR_04_HeaviestBehaviorPage(t *testing.T) {
	page := gx.Page(func(*gx.Ctx, nfr04Route) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node {
			return gx.Frag(
				gx.El("div", gx.Attrs{
					{Key: "popover", Value: "auto"},
					{Key: "role", Value: "menu"},
					{Key: "data-gx-dismiss", Value: ""},
					{Key: "data-gx-roving", Value: "nowrap"},
					{Key: "data-gx-place", Value: "bottom center 4"},
				},
					gx.El("div", gx.Attrs{{Key: "data-gx-sub", Value: ""}},
						gx.El("button", gx.Attrs{{Key: "data-gx-roving-item", Value: ""}}, gx.Text("More")),
						gx.El("div", gx.Attrs{
							{Key: "popover", Value: "auto"},
							{Key: "role", Value: "menu"},
							{Key: "data-gx-roving", Value: "nowrap"},
							{Key: "data-gx-place", Value: "right start"},
						}),
					),
				),
				gx.El("div", gx.Attrs{{Key: "data-gx-tabs", Value: ""}}),
				gx.Toaster(),
			)
		})
	app := gx.New(gx.Config{Adapter: &nfr04Adapter{}})
	app.Group("/", gx.Collect(page))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/nfr04", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	modules := modulesOf(rec.Body.String())
	if !slices.Equal(modules, behaviorModules) {
		t.Fatalf("heaviest page loads %v, want %v:\n%s", modules, behaviorModules, rec.Body.String())
	}
	const budget = 6 * 1024
	total := 0
	for _, module := range modules {
		asset := httptest.NewRecorder()
		app.ServeHTTP(asset, httptest.NewRequest("GET", "/_gx/"+module+".js", nil))
		if asset.Code != http.StatusOK || asset.Body.Len() == 0 {
			t.Fatalf("/_gx/%s.js: status %d, %d bytes", module, asset.Code, asset.Body.Len())
		}
		total += gzipLen(t, asset.Body.Bytes())
	}
	if total > budget {
		t.Fatalf("behaviour modules of the heaviest page = %d bytes gzipped, budget %d", total, budget)
	}
	t.Logf("behaviour modules of the heaviest page: %d bytes gzipped (budget %d)", total, budget)
}

// TestREQ_REG_07_BehaviorLoads checks that each behaviour module joins only
// a page whose markup uses one of its markers (REQ-REG-07).
func TestREQ_REG_07_BehaviorLoads(t *testing.T) {
	tests := []struct {
		name    string
		markers []string
		want    []string
	}{
		{"plain", nil, nil},
		{"accordion", []string{"open", "name"}, nil},
		{"roving and dismiss", []string{"data-gx-roving", "data-gx-dismiss"}, []string{"behavior"}},
		{"trap", []string{"data-gx-trap"}, []string{"behavior"}},
		{"open and close", []string{"data-gx-open", "data-gx-close"}, []string{"behavior"}},
		{"context menu area", []string{"data-gx-contextmenu"}, []string{"overlay"}},
		{"slider fill", []string{"data-gx-behavior"}, []string{"behavior"}},
		{"tabs", []string{"data-gx-tabs", "data-gx-tab", "data-gx-tab-panel"}, []string{"tabs"}},
		{"docs tab item", []string{"data-gx-tab-item"}, []string{"tabs"}},
		{"toaster", []string{"data-gx-toaster"}, []string{"toast"}},
		{"placed content", []string{"data-gx-place"}, []string{"overlay"}},
		{"sub-menu", []string{"data-gx-sub"}, []string{"overlay"}},
		{"menu", []string{"data-gx-roving", "data-gx-dismiss", "data-gx-place"}, []string{"behavior", "overlay"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := servePage(t, &nfr04Adapter{}, func() gx.Node {
				var attrs gx.Attrs
				for _, key := range tt.markers {
					attrs = append(attrs, gx.Attr{Key: key, Value: ""})
				}
				return gx.El("div", attrs)
			})
			if got := modulesOf(body); !slices.Equal(got, tt.want) {
				t.Fatalf("modules = %v, want %v:\n%s", got, tt.want, body)
			}
			if strings.Contains(body, "/_gx/gx.js") {
				t.Fatalf("behaviour page ships the core runtime:\n%s", body)
			}
			if strings.Contains(body, "/_gx/nfr04-adapter.js") {
				t.Fatalf("behaviour page ships the adapter runtime:\n%s", body)
			}
		})
	}
}
