package gx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// originsApp mounts the action of actRoute under /w with the given origin
// options. The middleware records the Cookie header that the handler side
// sees, and that the handler ran.
type originsApp struct {
	app    *gx.App
	ran    bool
	cookie string
}

func newOriginsApp(t *testing.T, options ...any) *originsApp {
	t.Helper()
	o := &originsApp{}
	record := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			o.ran = true
			o.cookie = r.Header.Get("Cookie")
			next.ServeHTTP(w, r)
		})
	}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return nil })
	o.app = gx.New(gx.Config{Adapter: &fakeAdapter{}})
	parts := append(options, record, gx.Collect(h))
	o.app.Group("/w", parts...)
	return o
}

// from sends a request as a browser page of the origin sends it.
func (o *originsApp) from(method, origin string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	o.ran, o.cookie = false, ""
	req := httptest.NewRequest(method, "https://api.acme.dev/w/act", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Sec-Fetch-Mode", "cors")
	}
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	o.app.ServeHTTP(rec, req)
	return rec
}

// TestREQ_ISL_22_AllowOrigins checks each origin form of a group: an exact
// origin, the subdomains of a host, and each origin.
func TestREQ_ISL_22_AllowOrigins(t *testing.T) {
	cases := []struct {
		name    string
		options []any
		origin  string
		want    int
	}{
		{"exact origin", []any{gx.AllowOrigins("https://shop.example.com")}, "https://shop.example.com", 204},
		{"exact origin with a port", []any{gx.AllowOrigins("http://localhost:5173")}, "http://localhost:5173", 204},
		{"a different port", []any{gx.AllowOrigins("http://localhost:5173")}, "http://localhost:5174", 403},
		{"a different scheme", []any{gx.AllowOrigins("https://shop.example.com")}, "http://shop.example.com", 403},
		{"an origin that is not in the list", []any{gx.AllowOrigins("https://shop.example.com")}, "https://evil.example", 403},
		{"a host that starts with the listed host", []any{gx.AllowOrigins("https://shop.example.com")}, "https://shop.example.com.evil.example", 403},
		{"subdomain", []any{gx.AllowOrigins("https://*.partner.io")}, "https://app.partner.io", 204},
		{"two subdomain labels", []any{gx.AllowOrigins("https://*.partner.io")}, "https://a.b.partner.io", 204},
		{"the host of a wildcard is not a subdomain", []any{gx.AllowOrigins("https://*.partner.io")}, "https://partner.io", 403},
		{"a host that ends with the name and no dot", []any{gx.AllowOrigins("https://*.partner.io")}, "https://evilpartner.io", 403},
		{"wildcard with a different scheme", []any{gx.AllowOrigins("https://*.partner.io")}, "http://app.partner.io", 403},
		{"wildcard with a port that is not listed", []any{gx.AllowOrigins("https://*.partner.io")}, "https://app.partner.io:8443", 403},
		{"each origin", []any{gx.AllowOrigins(gx.AnyOrigin)}, "https://any.example", 204},
		{"the second option of a group", []any{gx.AllowOrigins("https://a.example"), gx.AllowCredentials("https://app.acme.dev")}, "https://app.acme.dev", 204},
		{"no option", nil, "https://shop.example.com", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := newOriginsApp(t, tc.options...)
			rec := o.from("POST", tc.origin, nil)
			if rec.Code != tc.want {
				t.Fatalf("POST from %s = %d, want %d", tc.origin, rec.Code, tc.want)
			}
			got := rec.Header().Get("Access-Control-Allow-Origin")
			if tc.want == 204 {
				if got != tc.origin {
					t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.origin)
				}
				if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Origin") {
					t.Errorf("Vary = %q, want Origin in it", rec.Header().Values("Vary"))
				}
				if !o.ran {
					t.Error("the handler did not run")
				}
			} else {
				if got != "" {
					t.Errorf("Access-Control-Allow-Origin = %q for a refused origin", got)
				}
				if o.ran {
					t.Error("the handler ran for a refused origin")
				}
			}
		})
	}
}

// TestREQ_ISL_22_Preflight checks that a group with origins answers the CORS
// preflight of a listed origin, and that a group with none does not change.
func TestREQ_ISL_22_Preflight(t *testing.T) {
	preflight := func(r *http.Request) {
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "authorization,gx-widget")
	}
	o := newOriginsApp(t, gx.AllowOrigins("https://shop.example.com"))

	rec := o.from("OPTIONS", "https://shop.example.com", preflight)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("Access-Control-Allow-Methods = %q, want POST in it", got)
	}
	if got := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers")); !strings.Contains(got, "authorization") || !strings.Contains(got, "gx-widget") {
		t.Errorf("Access-Control-Allow-Headers = %q", got)
	}
	if o.ran {
		t.Error("the handler ran for a preflight")
	}

	rec = o.from("OPTIONS", "https://evil.example", preflight)
	if rec.Code != http.StatusForbidden {
		t.Errorf("preflight of an origin that is not in the list = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q for a refused preflight", got)
	}

	plain := newOriginsApp(t)
	rec = plain.from("OPTIONS", "https://shop.example.com", preflight)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" || rec.Code == http.StatusNoContent {
		t.Errorf("a group with no origins answered a preflight: %d, Access-Control-Allow-Origin %q", rec.Code, got)
	}
}

// TestREQ_ISL_22_OriginsOfOneGroupOnly checks that the origins of one group
// open no route of a different group.
func TestREQ_ISL_22_OriginsOfOneGroupOnly(t *testing.T) {
	ran := false
	open := gx.Action(func(c *gx.Ctx, in actRoute) error { return nil })
	closed := gx.Action(func(c *gx.Ctx, in actRoute) error { ran = true; return nil })
	app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
	app.Group("/w", gx.AllowOrigins("https://shop.example.com"), gx.Collect(open))
	app.Group("/app", gx.Collect(closed))

	req := httptest.NewRequest("POST", "https://api.acme.dev/app/act", nil)
	req.Header.Set("Origin", "https://shop.example.com")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || ran {
		t.Fatalf("POST to a group with no origins = %d, handler ran: %v; want 403 and no run", rec.Code, ran)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q on a group with no origins", got)
	}
}

// TestREQ_ISL_22_BadOrigins checks that a wrong origin stops the app at
// startup, and that AllowCredentials takes exact origins only.
func TestREQ_ISL_22_BadOrigins(t *testing.T) {
	cases := []struct {
		name string
		make func()
	}{
		{"no scheme", func() { gx.AllowOrigins("shop.example.com") }},
		{"a path", func() { gx.AllowOrigins("https://shop.example.com/cart") }},
		{"a slash at the end", func() { gx.AllowOrigins("https://shop.example.com/") }},
		{"a wildcard in the middle", func() { gx.AllowOrigins("https://shop.*.com") }},
		{"a wildcard with no dot", func() { gx.AllowOrigins("https://*example.com") }},
		{"a wildcard for each host", func() { gx.AllowOrigins("https://*") }},
		{"an empty origin", func() { gx.AllowOrigins("") }},
		{"no origin", func() { gx.AllowOrigins() }},
		{"credentials with a wildcard", func() { gx.AllowCredentials("https://*.acme.dev") }},
		{"credentials with each origin", func() { gx.AllowCredentials(gx.AnyOrigin) }},
		{"credentials with no origin", func() { gx.AllowCredentials() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("no panic")
				}
			}()
			tc.make()
		})
	}
}

// TestSI_14_CookiesAcrossOrigins checks that a cross-origin request carries
// the cookies of the user only for an origin of AllowCredentials.
func TestSI_14_CookiesAcrossOrigins(t *testing.T) {
	withCookie := func(r *http.Request) { r.Header.Set("Cookie", "session=alice") }
	options := []any{
		gx.AllowOrigins("https://shop.example.com", "https://*.partner.io"),
		gx.AllowCredentials("https://app.acme.dev"),
	}

	t.Run("an origin of AllowOrigins has no cookie", func(t *testing.T) {
		for _, origin := range []string{"https://shop.example.com", "https://x.partner.io"} {
			o := newOriginsApp(t, options...)
			rec := o.from("POST", origin, withCookie)
			if rec.Code != http.StatusNoContent || !o.ran {
				t.Fatalf("POST from %s = %d, ran %v", origin, rec.Code, o.ran)
			}
			if o.cookie != "" {
				t.Errorf("the handler side of %s saw the cookie %q", origin, o.cookie)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Errorf("Access-Control-Allow-Credentials = %q for %s", got, origin)
			}
			if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
				t.Errorf("Set-Cookie = %q in an answer to %s", got, origin)
			}
		}
	})

	t.Run("each origin has no cookie", func(t *testing.T) {
		o := newOriginsApp(t, gx.AllowOrigins(gx.AnyOrigin))
		rec := o.from("POST", "https://evil.example", withCookie)
		if rec.Code != http.StatusNoContent || o.cookie != "" {
			t.Fatalf("POST = %d, cookie on the handler side %q; want 204 and none", rec.Code, o.cookie)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("Access-Control-Allow-Credentials = %q", got)
		}
	})

	t.Run("a GET of an origin of AllowOrigins has no cookie", func(t *testing.T) {
		seen := "unset"
		record := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("Cookie")
				w.WriteHeader(http.StatusNoContent)
			})
		}
		h := gx.Action(func(c *gx.Ctx, in getRoute) error { return nil })
		app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
		app.Group("/w", gx.AllowOrigins("https://shop.example.com"), record, gx.Collect(h))
		req := httptest.NewRequest("GET", "https://api.acme.dev/w/get", nil)
		req.Header.Set("Origin", "https://shop.example.com")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Cookie", "session=alice")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if seen != "" {
			t.Errorf("the handler side saw the cookie %q", seen)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://shop.example.com" {
			t.Errorf("Access-Control-Allow-Origin = %q", got)
		}
	})

	t.Run("an origin of AllowCredentials keeps the cookie", func(t *testing.T) {
		o := newOriginsApp(t, options...)
		rec := o.from("POST", "https://app.acme.dev", withCookie)
		if rec.Code != http.StatusNoContent || o.cookie != "session=alice" {
			t.Fatalf("POST = %d, cookie on the handler side %q", rec.Code, o.cookie)
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Access-Control-Allow-Credentials = %q, want true", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.acme.dev" {
			t.Errorf("Access-Control-Allow-Origin = %q, want the exact origin", got)
		}
	})

	t.Run("the same origin keeps the cookie", func(t *testing.T) {
		o := newOriginsApp(t, options...)
		rec := o.from("POST", "", func(r *http.Request) {
			withCookie(r)
			r.Header.Set("Origin", "https://api.acme.dev")
			r.Header.Set("Sec-Fetch-Site", "same-origin")
			r.Header.Set("Sec-Fetch-Mode", "cors")
		})
		if rec.Code != http.StatusNoContent || o.cookie != "session=alice" {
			t.Fatalf("same-origin POST = %d, cookie on the handler side %q", rec.Code, o.cookie)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q on a same-origin answer", got)
		}
	})

	t.Run("a hostile origin cannot write with the cookie", func(t *testing.T) {
		o := newOriginsApp(t, options...)
		rec := o.from("POST", "https://evil.example", withCookie)
		if rec.Code != http.StatusForbidden || o.ran {
			t.Fatalf("POST = %d, ran %v; want 403 and no run", rec.Code, o.ran)
		}
		// A request with a false Sec-Fetch-Site and a hostile Origin is
		// not a browser request; it still gets no cookie path.
		rec = o.from("POST", "https://evil.example", func(r *http.Request) {
			withCookie(r)
			r.Header.Del("Sec-Fetch-Site")
			r.Header.Del("Sec-Fetch-Mode")
		})
		if rec.Code != http.StatusForbidden || o.ran {
			t.Fatalf("POST with no Fetch Metadata = %d, ran %v; want 403 and no run", rec.Code, o.ran)
		}
	})
}

// getRoute is a hand-written GET action input.
type getRoute struct{}

func (getRoute) Pattern() string          { return "GET /get" }
func (getRoute) Bind(*http.Request) error { return nil }
