package gx_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

var (
	cspNonceRe  = regexp.MustCompile(`'nonce-([A-Za-z0-9+/=_-]{16,})'`)
	scriptTagRe = regexp.MustCompile(`<script\b[^>]*>`)
)

// cspPage is a page with a signal, a theme control, a toaster, an inline
// script of the page author and a behaviour marker, so the shell writes
// every kind of script Gx owns.
func cspPage() gx.Node {
	return gx.Frag(
		gx.El("input", gx.Attrs{{Key: "data-signals", Value: "{}"}, {Key: "data-bind", Value: "q"}}),
		gx.El("button", gx.Attrs{{Key: "data-gx-theme", Value: "dark"}}, gx.Text("Dark")),
		gx.El("div", gx.Attrs{{Key: "data-gx-toaster", Value: ""}, {Key: "data-gx-tabs", Value: ""}, {Key: "data-gx-behavior", Value: "menu"}, {Key: "data-gx-place", Value: "bottom"}}),
		gx.El("script", nil, gx.Raw("window.inlineRan = true")),
	)
}

// checkNonces fails when a script tag of body lacks the nonce of the policy.
func checkNonces(t *testing.T, policy, body string, wantScripts int) string {
	t.Helper()
	m := cspNonceRe.FindStringSubmatch(policy)
	if m == nil {
		t.Fatalf("the policy has no nonce: %q", policy)
	}
	scripts := scriptTagRe.FindAllString(body, -1)
	if len(scripts) < wantScripts {
		t.Fatalf("the page has %d scripts, want at least %d:\n%s", len(scripts), wantScripts, body)
	}
	for _, tag := range scripts {
		if !strings.Contains(tag, ` nonce="`+m[1]+`"`) {
			t.Fatalf("a script lacks the nonce %q: %s", m[1], tag)
		}
	}
	return m[1]
}

// TestSI_11_ScriptNonce covers the CSP nonce: under gx.CSP every script the
// page ships carries the nonce of the response policy, the nonce changes
// per request, and a page with no policy has no nonce (SI-11).
func TestSI_11_ScriptNonce(t *testing.T) {
	page := gx.Page(func(*gx.Ctx, nfr04Route) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return cspPage() })
	app := gx.New(gx.Config{Adapter: &nfr04Adapter{}})
	app.Group("/", gx.Collect(page))

	serve := func(h http.Handler) (string, string) {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/nfr04", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
		}
		return rec.Header().Get("Content-Security-Policy"), rec.Body.String()
	}

	strict := gx.CSP(gx.CSPOptions{})(app)
	policy, body := serve(strict)
	// theme, core, adapter, behavior, tabs, toast, overlay and the inline one.
	first := checkNonces(t, policy, body, 8)
	for _, want := range []string{"script-src 'nonce-" + first + "' 'strict-dynamic'", "object-src 'none'", "base-uri 'self'"} {
		if !strings.Contains(policy, want) {
			t.Fatalf("the policy lacks %q: %s", want, policy)
		}
	}
	if strings.Contains(policy, "unsafe-eval") || strings.Contains(policy, "unsafe-inline") {
		t.Fatalf("the default policy is not strict: %s", policy)
	}
	policy2, body2 := serve(strict)
	if second := checkNonces(t, policy2, body2, 8); second == first {
		t.Fatalf("two requests share the nonce %q", first)
	}

	// Datastar evaluates expressions at runtime (OI-01).
	policy, body = serve(gx.CSP(gx.CSPOptions{UnsafeEval: true, Directives: "img-src 'self' data:"})(app))
	nonce := checkNonces(t, policy, body, 8)
	for _, want := range []string{"script-src 'nonce-" + nonce + "' 'strict-dynamic' 'unsafe-eval'", "img-src 'self' data:"} {
		if !strings.Contains(policy, want) {
			t.Fatalf("the policy lacks %q: %s", want, policy)
		}
	}

	// The policy can also be group middleware: the shell still sees it.
	grouped := gx.New(gx.Config{Adapter: &nfr04Adapter{}})
	grouped.Group("/", gx.CSP(gx.CSPOptions{}), gx.Collect(page))
	policy, body = serve(grouped)
	checkNonces(t, policy, body, 8)

	// No policy, no nonce.
	policy, body = serve(app)
	if policy != "" || strings.Contains(body, "nonce=") {
		t.Fatalf("a page with no policy has a nonce: %q\n%s", policy, body)
	}
}

// TestSI_11_RenderNonce covers adoption level 1: an app with its own CSP
// middleware hands the nonce to Gx, and gx.Render writes it on every script
// (SI-11, REQ-RTE-16).
func TestSI_11_RenderNonce(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = gx.WithNonce(r, "app-made-nonce-0001")
		if got := gx.Nonce(r); got != "app-made-nonce-0001" {
			t.Errorf("gx.Nonce = %q", got)
		}
		_ = gx.Render(w, r, gx.Frag(
			gx.El("script", nil, gx.Raw("window.a = 1")),
			gx.El("script", gx.Attrs{{Key: "nonce", Value: "kept"}}, gx.Raw("window.b = 1")),
			gx.El("p", nil, gx.Text("<script>")),
		))
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	want := `<script nonce="app-made-nonce-0001">window.a = 1</script><script nonce="kept">window.b = 1</script><p>&lt;script&gt;</p>`
	if got := rec.Body.String(); got != want {
		t.Fatalf("render = %s\nwant    %s", got, want)
	}
	if got := gx.Nonce(httptest.NewRequest("GET", "/", nil)); got != "" {
		t.Fatalf("a request with no policy has the nonce %q", got)
	}
}
