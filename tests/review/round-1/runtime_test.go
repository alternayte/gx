package round1_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/tests/review/round-1/adminui"
	"github.com/alternayte/gx/tests/review/round-1/shopui"
)

// Hand-written route inputs. Generated route types provide the same Pattern
// and Bind methods (the repository's own unit tests use this shape).
type accountPage struct{}

func (accountPage) Pattern() string          { return "GET /account" }
func (accountPage) Bind(*http.Request) error { return nil }

type adminPage struct{}

func (adminPage) Pattern() string          { return "GET /admin" }
func (adminPage) Bind(*http.Request) error { return nil }

type transferAction struct{}

func (transferAction) Pattern() string          { return "POST /transfer" }
func (transferAction) Bind(*http.Request) error { return nil }

// requireLogin is plain net/http middleware that rejects every request.
func requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "login required", http.StatusUnauthorized)
	})
}

func accountHandler() gx.Handler {
	return gx.Page(func(c *gx.Ctx, in accountPage) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("private account data") })
}

// REQ-RTE-06: "Mounting is explicit: ... app.Group(prefix, layout,
// middleware..., routes...)". REQ-RTE-09: "Middleware is
// func(http.Handler) http.Handler." SDD §5 shows
// app.Group("/", layouts.Shop, auth.Optional, products.Routes, cart.Routes).
//
// Defect: App.Group builds the middleware chain around the route and then
// throws it away when the group has a layout: the layoutHandler wraps the
// bare route (route.go:424-430, `inner: h`). Auth middleware in a group with
// a layout never runs, and the page renders for everyone. The covering test
// TestREQ_RTE_09_Middleware has no layout in its group.
func TestREQ_RTE_09_MiddlewareRunsForPageUnderLayout(t *testing.T) {
	app := gx.New(gx.Config{})
	app.Group("/", shopui.Layout, requireLogin, gx.Collect(accountHandler()))

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/account", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 from the group middleware", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "private account data") {
		t.Errorf("the page rendered although the group middleware rejects the request:\n%s", rec.Body.String())
	}
}

// REQ-RTE-09 and REQ-RTE-12: a group takes middleware and morph navigation
// together.
//
// Defect: with gx.Nav(gx.MorphNavigation), a middleware and no layout,
// App.Group asserts that the middleware-wrapped handler is a gx.Handler
// (route.go:432, `handler.(Handler)`). An http.HandlerFunc is not, so the
// app panics at startup.
func TestREQ_RTE_09_MiddlewareWithMorphNavigation(t *testing.T) {
	var rec *httptest.ResponseRecorder
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Group panicked for a group with middleware and morph navigation: %v", r)
			}
		}()
		app := gx.New(gx.Config{Adapter: fakeAdapter{}})
		app.Group("/", gx.Nav(gx.MorphNavigation), requireLogin, gx.Collect(accountHandler()))
		rec = httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/account", nil))
	}()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 from the group middleware", rec.Code)
	}
}

// SI-03: "Every non-GET request passes Go net/http cross-origin protection".
// REQ-RTE-17: "Every page, action and form value is an http.Handler."
// REQ-RTE-19: "Levels 1 to 3 need no Gx-specific middleware beyond what
// gx.Render and the route handlers set up themselves." §5.2 gives typed
// actions and forms to adoption level 2 (single routes on another router).
//
// Defect: only gx.App.ServeHTTP applies the protection (route.go:373). An
// action registered on another router, as level 2 shows, runs a cross-site
// POST. D-034 makes gx.CSRF a middleware the user must remember, which
// REQ-RTE-19 forbids. The covering tests TestSI_03_* all go through gx.App.
func TestSI_03_ActionOnAnotherRouterRejectsCrossSitePOST(t *testing.T) {
	ran := false
	transfer := gx.Action(func(c *gx.Ctx, in transferAction) error {
		ran = true
		return nil
	})
	mux := http.NewServeMux()
	mux.Handle(transfer.Pattern(), transfer) // adoption level 2

	req := httptest.NewRequest("POST", "http://shop.example/transfer", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if ran {
		t.Errorf("a cross-site POST ran the action (status %d)", rec.Code)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// SI-02: "`javascript:` URLs cannot be produced." REQ-RTE-05 lets href take
// a gx.URL value, and gx.URL is a plain string type, so gx.URL(row.Website)
// is the documented way to link to an address from data
// (docs/site: GX2011 fix 2, "Gx cannot write a javascript: address").
//
// Defect: the compiler emits gx.Attr{Key: "href", Value: string(u), Kind:
// gx.AttrURL} for a gx.URL, and escapeURL (gx.go:406) only percent-encodes.
// It keeps the scheme, so the link runs script on click. The fuzz test
// FuzzSI_02_URLOutput only checks that the value stays inside the
// attribute, and TestREQ_AUT_12_EscapingMatrix pins the javascript: output.
func TestSI_02_JavascriptURLIsNotRendered(t *testing.T) {
	for _, u := range []gx.URL{"javascript:alert(document.cookie)", "JavaScript:alert(1)", " javascript:alert(1)", "java\tscript:alert(1)"} {
		// This is the attribute the generated code builds for href={u}.
		out := gx.String(gx.El("a", gx.Attrs{{Key: "href", Value: string(u), Kind: gx.AttrURL}}, gx.Text("site")))
		m := regexp.MustCompile(`href="([^"]*)"`).FindStringSubmatch(out)
		if m == nil {
			continue // no href at all is safe
		}
		// A browser strips leading spaces and inner tabs before it reads
		// the scheme.
		scheme := strings.ToLower(strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(strings.TrimLeft(m[1], " \t\r\n\f")))
		if strings.HasPrefix(scheme, "javascript:") {
			t.Errorf("gx.URL(%q) rendered a javascript: link: %s", string(u), out)
		}
	}
}

// REQ-RTE-13: "An <a> with a typed route href that matches the current
// request exactly gets aria-current=\"page\"." REQ-RTE-18: "A gx.App is an
// http.Handler that can be mounted under a prefix. Config.BasePath prefixes
// typed links"; acceptance: the mounted shop passes the shop suite.
//
// Defect: the renderer compares the link with r.URL.RequestURI()
// (gx.go:247, gx.go:334). Under a mount the request path has no prefix
// ("/account") and the typed link has it ("/shop/account"), so no link of a
// mounted app is ever active. TestREQ_RTE_13_ActiveLinks runs with no base
// path, and TestREQ_RTE_18_Mount renders no link.
func TestREQ_RTE_13_ActiveLinkInMountedApp(t *testing.T) {
	app := gx.New(gx.Config{BasePath: "/shop"})
	defer gx.SetBasePath("")
	pg := gx.Page(func(c *gx.Ctx, in accountPage) (int, error) { return 1, nil },
		func(int) gx.Node {
			// The generated URL() of a route returns gx.BasePath() + path,
			// and the generated href has Active: "page".
			return gx.El("a", gx.Attrs{{Key: "href", Value: gx.BasePath() + "/account", Kind: gx.AttrURL, Active: "page"}}, gx.Text("Account"))
		})
	app.Group("/", gx.Collect(pg))
	mux := http.NewServeMux()
	mux.Handle("/shop/", http.StripPrefix("/shop", app)) // as TestREQ_RTE_18_Mount mounts it

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/shop/account", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `aria-current="page"`) {
		t.Fatalf("the link to the current page of a mounted app has no aria-current:\n%s", rec.Body.String())
	}
}

// REQ-RTE-12: "A different layout chain or an error falls back to a full
// load."
//
// Defect: a layout is identified by the base file name and line of its
// gx.Layout call (route.go:182-188, filepath.Base(file)+":"+line). Two
// slices that each declare their layout in layout.go at the same line get
// the same id. A navigation from a shop page to an admin page then counts
// as "same layout": the server sends only the inner slot, and the admin
// page shows inside the shop layout. TestREQ_RTE_12_LayoutFallback uses a
// made-up id ("other.go:1") for the other layout.
func TestREQ_RTE_12_LayoutsWithTheSameFileNameAndLine(t *testing.T) {
	app := gx.New(gx.Config{Adapter: fakeAdapter{}})
	shop := gx.Page(func(c *gx.Ctx, in accountPage) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("shop page") })
	admin := gx.Page(func(c *gx.Ctx, in adminPage) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("admin page") })
	app.Group("/", shopui.Layout, gx.Nav(gx.MorphNavigation), gx.Collect(shop))
	app.Group("/", adminui.Layout, gx.Nav(gx.MorphNavigation), gx.Collect(admin))

	// The browser is on the shop page and holds its layout slots.
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("GET", "/account", nil))
	var held []string
	for _, m := range regexp.MustCompile(`data-gx-slot="([^"]+)"`).FindAllStringSubmatch(rec.Body.String(), -1) {
		held = append(held, m[1])
	}
	if len(held) == 0 {
		t.Fatalf("the shop page has no layout slot:\n%s", rec.Body.String())
	}

	// It follows a link to the admin page, as runtime/js/gx.ts does.
	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Gx-Nav", "1")
	req.Header.Set("Gx-Layouts", strings.Join(held, ","))
	req.Header.Set("Accept", "text/event-stream")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "admin page") {
		t.Fatalf("the answer does not hold the admin page: %d %q", rec.Code, body)
	}
	if !strings.Contains(body, `id="admin-layout"`) {
		t.Fatalf("a navigation to a page with a different layout sent the page without its layout, so it shows inside the shop layout:\n%s", body)
	}
}

// REQ-ACT-13: "`int` values outside the 53-bit safe range fail in dev."
// DR-05: "A typed Go subset must not lie about semantics."
//
// Defect: nothing implements the rule. gx.JSON (gx.go:114) is the function
// the compiler emits for every server value inlined into a client
// expression; it encodes any int64 and the browser then rounds it. The
// covering test TestREQ_ACT_13_Differential draws ints from -1e6..1e6 only.
func TestREQ_ACT_13_IntOutsideSafeRangeFailsInDev(t *testing.T) {
	gx.SetDev(true)
	defer gx.SetDev(false)
	const big = int64(1)<<53 + 1 // 9007199254740993: JavaScript reads ...992

	failed := false
	out := ""
	func() {
		defer func() {
			if recover() != nil {
				failed = true
			}
		}()
		out = gx.JSON(big)
	}()
	if !failed {
		t.Fatalf("gx.JSON(%d) in dev returned %q: the value entered a client expression and did not fail", big, out)
	}
}

// REQ-RTE-08: "Layout loaders run concurrently with the page loader."
// REQ-RTE-10: a failing loader is a 500 (e2e for 404, 403, redirect, 500).
// REQ-DEV-06: a panic shows the overlay and "the page recovers when the
// error is fixed", so a panic must not end the process.
//
// Defect: layoutHandler.loadChain runs the page loader and the layout
// loaders in new goroutines with no recover (route.go:639-651). net/http
// recovers a panic only on the goroutine that serves the request, so one
// nil dereference in a loader of a page under a layout stops the whole
// server. The same page without a layout only fails its own request.
func TestREQ_RTE_10_LoaderPanicUnderLayoutKeepsTheServerUp(t *testing.T) {
	if os.Getenv("GX_REVIEW_PANIC_CHILD") == "1" {
		panicChild()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestREQ_RTE_10_LoaderPanicUnderLayoutKeepsTheServerUp$")
	cmd.Env = append(os.Environ(), "GX_REVIEW_PANIC_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "server alive after the panic") {
		lines := strings.Split(string(out), "\n")
		if len(lines) > 12 {
			lines = lines[:12]
		}
		t.Fatalf("a loader panic under a layout stopped the server process (%v):\n%s", err, strings.Join(lines, "\n"))
	}
}

// panicChild serves one request whose page loader panics, then one healthy
// request, on a real net/http server.
func panicChild() {
	app := gx.New(gx.Config{})
	broken := gx.Page(func(c *gx.Ctx, in accountPage) (int, error) {
		var p *int
		return *p, nil // a plain bug in app code
	}, func(int) gx.Node { return gx.Text("x") })
	healthy := gx.Page(func(c *gx.Ctx, in adminPage) (int, error) { return 1, nil },
		func(int) gx.Node { return gx.Text("healthy") })
	app.Group("/", shopui.Layout, gx.Collect(broken, healthy))
	srv := httptest.NewServer(app)
	defer srv.Close()
	srv.Config.ErrorLog = nil

	if resp, err := http.Get(srv.URL + "/account"); err == nil {
		resp.Body.Close()
	}
	resp, err := http.Get(srv.URL + "/admin")
	if err != nil {
		fmt.Println("second request failed:", err)
		os.Exit(1)
	}
	resp.Body.Close()
	fmt.Println("server alive after the panic")
}
