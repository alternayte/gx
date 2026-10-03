package gx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

func csrfApp(t *testing.T, a *fakeAdapter) *gx.App {
	t.Helper()
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return nil })
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(h))
	return app
}

func post(app *gx.App, mutate func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/act", nil)
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// TestSI_03_CrossSitePOST checks cross-site non-GET requests are rejected
// (SI-03).
func TestSI_03_CrossSitePOST(t *testing.T) {
	app := csrfApp(t, &fakeAdapter{})
	rec := post(app, func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", rec.Code)
	}
}

// TestSI_03_FetchMetadataSameOrigin checks a modern browser request needs no
// token.
func TestSI_03_FetchMetadataSameOrigin(t *testing.T) {
	app := csrfApp(t, &fakeAdapter{})
	rec := post(app, func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("Sec-Fetch-Mode", "cors")
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("same-origin POST = %d, want 204", rec.Code)
	}
}

// TestSI_03_TokenForOldBrowsers checks a browser-shaped request without
// Fetch Metadata needs the token.
func TestSI_03_TokenForOldBrowsers(t *testing.T) {
	app := csrfApp(t, &fakeAdapter{})
	rec := post(app, func(r *http.Request) {
		r.Header.Set("Origin", "http://example.com")
		r.Host = "example.com"
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("tokenless POST = %d, want 403", rec.Code)
	}

	// A GET sets the token cookie.
	get := httptest.NewRequest("GET", "/act", nil)
	getRec := httptest.NewRecorder()
	app.ServeHTTP(getRec, get)
	cookies := getRec.Result().Cookies()
	var token string
	for _, c := range cookies {
		if c.Name == "gx_csrf" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("GET did not set a CSRF cookie")
	}

	rec = post(app, func(r *http.Request) {
		r.Header.Set("Origin", "http://example.com")
		r.Host = "example.com"
		r.Header.Set("Gx-CSRF", token)
		r.AddCookie(&http.Cookie{Name: "gx_csrf", Value: token})
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("token POST = %d, want 204", rec.Code)
	}
}

// TestREQ_FRM_12_CSRF checks that every form POST passes the CSRF layer
// (REQ-FRM-12).
func TestREQ_FRM_12_CSRF(t *testing.T) {
	form := gx.Form(func(c *gx.Ctx, in *signupInStub) error { return nil }, signupViewStub)
	app := gx.New(gx.Config{})
	app.Group("/", gx.Collect(form))

	formPost := func(mutate func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/signup", strings.NewReader("email=&age=20"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if mutate != nil {
			mutate(req)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}

	if rec := formPost(func(r *http.Request) {
		r.Header.Set("Origin", "https://evil.example")
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site form POST = %d, want 403", rec.Code)
	}
	if rec := formPost(nil); rec.Code != http.StatusForbidden {
		t.Fatalf("tokenless form POST = %d, want 403", rec.Code)
	}

	// A GET sets the token; the form POST carries it.
	get := httptest.NewRequest("GET", "/signup", nil)
	getRec := httptest.NewRecorder()
	app.ServeHTTP(getRec, get)
	token := ""
	for _, c := range getRec.Result().Cookies() {
		if c.Name == "gx_csrf" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("GET did not set a CSRF cookie")
	}
	rec := formPost(func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "gx_csrf", Value: token})
		r.Form = nil
		r.Body = io.NopCloser(strings.NewReader("email=&age=20&gx_csrf=" + token))
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("form POST with token = %d, want 422", rec.Code)
	}
}
