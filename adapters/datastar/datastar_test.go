package datastar_test

import (
	"crypto/sha256"
	"encoding/hex"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

// actRoute is a hand-written action input for the adapter contract tests.
type actRoute struct{}

func (actRoute) Pattern() string          { return "POST /act" }
func (actRoute) Bind(*http.Request) error { return nil }

func serve(t *testing.T, h gx.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", gx.Collect(h))
	req.Header.Set("Datastar-Request", "true")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// TestREQ_PLG_04_DatastarWire checks the public hook reaches the wire format
// of the pinned browser runtime.
func TestREQ_PLG_04_DatastarWire(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2")))
	})
	rec := serve(t, h, httptest.NewRequest("POST", "/act", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, body)
	}
	if !strings.Contains(body, "event: datastar-patch-elements") {
		t.Fatalf("SSE body lacks the element event:\n%s", body)
	}
	if !strings.Contains(body, "selector #count") {
		t.Fatalf("SSE body lacks the selector:\n%s", body)
	}
	if !strings.Contains(body, `<span id="count">2</span>`) {
		t.Fatalf("SSE body lacks the patch HTML:\n%s", body)
	}
	if strings.Contains(body, "mode append") {
		t.Fatalf("a morph patch must not set append mode:\n%s", body)
	}
}

// TestREQ_PLG_04_SignalScope checks the invoking scope nests the signals.
func TestREQ_PLG_04_SignalScope(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.SetSignals(map[string]int{"qty": 2})
	})
	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.42")
	rec := serve(t, h, req)
	body := rec.Body.String()
	if !strings.Contains(body, "event: datastar-patch-signals") {
		t.Fatalf("SSE body lacks the signal event:\n%s", body)
	}
	if !strings.Contains(body, `signals {"cart":{"42":{"qty":2}}}`) {
		t.Fatalf("SSE body lacks the nested signal scope:\n%s", body)
	}
}

// TestREQ_PLG_04_Assets checks the pinned runtime is served and matches its
// recorded hash (SI-10 groundwork).
func TestREQ_PLG_04_Assets(t *testing.T) {
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/_gx/datastar.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("asset status = %d", rec.Code)
	}
	sum := sha256.Sum256(rec.Body.Bytes())
	if got := hex.EncodeToString(sum[:]); got != "727844adfc825ee651fb93c544a2a739986f9a21820a94524b35f0cac470cf91" {
		t.Fatalf("datastar.js hash = %s", got)
	}
	if !strings.Contains(rec.Body.String(), "Datastar v1.0.4") {
		t.Fatalf("asset is not Datastar v1.0.4")
	}
}

// TestREQ_PLG_04_ImportsOnlyPublicAPI checks an adapter uses public gx API
// only (REQ-PLG-04).
func TestREQ_PLG_04_ImportsOnlyPublicAPI(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := func(path string) bool {
		if path == "github.com/alternayte/gx" {
			return true
		}
		if strings.HasPrefix(path, "github.com/starfederation/datastar-go/") {
			return true
		}
		first := path
		if i := strings.IndexByte(path, '/'); i >= 0 {
			first = path[:i]
		}
		return !strings.Contains(first, ".")
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), e.Name(), src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if !allowed(path) {
				t.Errorf("%s imports %q; adapters use public API only", e.Name(), path)
			}
		}
	}
}
