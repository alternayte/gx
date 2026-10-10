package gx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// apiSave is a hand-written action input with one field of each source.
// Generated route types provide the same methods.
type apiSave struct {
	ID   int
	Name string
	Qty  int
}

func (apiSave) Pattern() string { return "POST /api/save/{id}" }

func (in *apiSave) Bind(r *http.Request) error {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return err
	}
	in.ID, in.Name = id, r.FormValue("name")
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	return gx.BindSignal(signals, gx.Scope(r), "qty", &in.Qty)
}

func (in *apiSave) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Name, gx.Required), gx.Field(&in.Qty, gx.Max(9))}
}

func (apiSave) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "api_save", Fields: []gx.ToolField{{Name: "id", In: "path"}, {Name: "name", In: "form"}, {Name: "qty", In: "signal"}}}
}

type apiSaved struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Qty  int    `json:"qty"`
}

// apiApp has one action with API that gives its input back, and the adapter
// of a page.
func apiApp(t *testing.T, mark bool, fn func(*gx.Ctx, apiSave) error, mw ...func(http.Handler) http.Handler) (*gx.App, *fakeAdapter) {
	t.Helper()
	a := &fakeAdapter{}
	act := gx.Action(fn)
	if mark {
		act = act.API()
	}
	app := gx.New(gx.Config{Adapter: a})
	parts := []any{}
	for _, m := range mw {
		parts = append(parts, m)
	}
	app.Group("/", append(parts, gx.Collect(act))...)
	return app, a
}

func apiEcho(c *gx.Ctx, in apiSave) error {
	gx.ToolResult(c, apiSaved{ID: in.ID, Name: in.Name, Qty: in.Qty})
	return c.Patch(gx.El("p", gx.Attrs{{Key: "id", Value: "saved"}}, gx.Text(in.Name)))
}

func apiCall(app *gx.App, body string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/save/7", strings.NewReader(body))
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(header); i += 2 {
		if header[i+1] == "" {
			req.Header.Del(header[i])
		} else {
			req.Header.Set(header[i], header[i+1])
		}
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// apiErrors reads the errors of a failed JSON call.
func apiErrors(t *testing.T, rec *httptest.ResponseRecorder) []map[string]string {
	t.Helper()
	var out struct {
		Errors []map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("the answer is not the JSON of errors: %v: %s", err, rec.Body.String())
	}
	return out.Errors
}

// TestREQ_ACT_19_JSONWire checks each answer kind of an action with API for
// a request that asks for JSON (REQ-ACT-19).
func TestREQ_ACT_19_JSONWire(t *testing.T) {
	app, page := apiApp(t, true, apiEcho)

	// The result: the path, the body field and the signal field arrive,
	// and the answer is the ToolResult value with no patch.
	rec := apiCall(app, `{"name":"tea","qty":3}`)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status %d, type %q, body %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var got apiSaved
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got != (apiSaved{ID: 7, Name: "tea", Qty: 3}) {
		t.Errorf("result = %s (%v), want id 7, name tea, qty 3", rec.Body.String(), err)
	}
	if page.responded {
		t.Error("a JSON call got the answer of a page")
	}

	// A rule failure: 422 with the field and the key.
	rec = apiCall(app, `{"name":"","qty":12}`)
	errs := apiErrors(t, rec)
	// The name of the field comes from the generated code of a route
	// type; the browser test of the shop checks it.
	if rec.Code != 422 || len(errs) == 0 || errs[0]["key"] != "required" {
		t.Errorf("rule failure: status %d, errors %v, want 422 with the key of the rule", rec.Code, errs)
	}

	// Input that is not the input of the action.
	if rec = apiCall(app, `{"name":"tea","qty":"many"}`); rec.Code != 400 {
		t.Errorf("a signal of the wrong type: status %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if rec = apiCall(app, `[1,2]`); rec.Code != 400 {
		t.Errorf("a body that is not an object: status %d, want 400", rec.Code)
	}
	if rec = apiCall(app, `name=tea`, "Content-Type", "application/x-www-form-urlencoded"); rec.Code != 415 {
		t.Errorf("a form body: status %d, want 415", rec.Code)
	}

	// No result: 204. An error of the handler: its status and its text.
	quiet, _ := apiApp(t, true, func(c *gx.Ctx, in apiSave) error { return nil })
	if rec = apiCall(quiet, `{"name":"tea"}`); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Errorf("no result: status %d, body %q, want 204 and no body", rec.Code, rec.Body.String())
	}
	broken, _ := apiApp(t, true, func(c *gx.Ctx, in apiSave) error { return errors.New("no stock") })
	rec = apiCall(broken, `{"name":"tea"}`)
	if errs = apiErrors(t, rec); rec.Code != 500 || len(errs) != 1 || errs[0]["message"] != "no stock" {
		t.Errorf("handler error: status %d, errors %v, want 500 with the text", rec.Code, errs)
	}
	missing, _ := apiApp(t, true, func(c *gx.Ctx, in apiSave) error { return gx.NotFound() })
	if rec = apiCall(missing, `{"name":"tea"}`); rec.Code != 404 {
		t.Errorf("gx.NotFound: status %d, want 404", rec.Code)
	}
}

// TestREQ_ACT_19_OnlyAMarkedActionAnswersJSON checks that an action with no
// API answers 406 to a JSON call, and that a request of a page or of an
// adapter gets patches from an action with API (REQ-ACT-19).
func TestREQ_ACT_19_OnlyAMarkedActionAnswersJSON(t *testing.T) {
	plain, page := apiApp(t, false, apiEcho)
	rec := apiCall(plain, `{"name":"tea"}`)
	if errs := apiErrors(t, rec); rec.Code != 406 || len(errs) != 1 || errs[0]["key"] != "gx.not_api" {
		t.Errorf("an action with no API: status %d, errors %v, want 406", rec.Code, errs)
	}
	if page.responded {
		t.Error("the action with no API ran for a JSON call")
	}

	marked, page := apiApp(t, true, apiEcho)
	for name, accept := range map[string]string{
		"a browser form":    "",
		"the adapter":       "text/event-stream, text/html, application/json",
		"a page navigation": "text/html,application/json;q=0.9",
	} {
		page.responded = false
		req := httptest.NewRequest("POST", "/api/save/7", strings.NewReader("name=tea"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// The form of a page has the token of the page (SI-03).
		req.Header.Set("Cookie", "gx_csrf=abc")
		req.Header.Set("Gx-CSRF", "abc")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		marked.ServeHTTP(rec, req)
		if !page.responded || strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("%s: the action answered JSON, want the patches of a page", name)
		}
	}
}

// TestSI_16_JSONCallWithNoCookieNeedsNoToken checks the cross-origin rule of
// the JSON wire: a call with no cookie needs no token and reaches the
// middleware of the group; a call with a cookie passes the check of SI-03 as
// each other request (SI-16).
func TestSI_16_JSONCallWithNoCookieNeedsNoToken(t *testing.T) {
	var seen []string
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = append(seen, r.Header.Get("Authorization"))
			next.ServeHTTP(w, r)
		})
	}
	app, _ := apiApp(t, true, apiEcho, auth)
	// A browser with no Fetch Metadata names its origin. With no cookie,
	// the call needs no token.
	rec := apiCall(app, `{"name":"tea"}`, "Origin", "http://example.com", "Authorization", "Bearer t")
	if rec.Code != 200 || len(seen) != 1 || seen[0] != "Bearer t" {
		t.Errorf("no cookie: status %d, middleware saw %v, want 200 and the Authorization header", rec.Code, seen)
	}
	// With a cookie and no token, the same call is refused.
	seen = nil
	rec = apiCall(app, `{"name":"tea"}`, "Origin", "http://example.com", "Cookie", "gx_csrf=abc; session=1")
	if rec.Code != 403 || len(seen) != 0 {
		t.Errorf("a cookie and no token: status %d, middleware saw %v, want 403 before the middleware", rec.Code, seen)
	}
	// With a cookie and its token, it passes.
	rec = apiCall(app, `{"name":"tea"}`, "Origin", "http://example.com", "Cookie", "gx_csrf=abc; session=1", "Gx-CSRF", "abc")
	if rec.Code != 200 {
		t.Errorf("a cookie with its token: status %d, want 200", rec.Code)
	}
	// A cross-site call with a cookie is refused.
	seen = nil
	rec = apiCall(app, `{"name":"tea"}`, "Origin", "https://other.example", "Sec-Fetch-Site", "cross-site", "Cookie", "session=1")
	if rec.Code != 403 || len(seen) != 0 {
		t.Errorf("cross-site with a cookie: status %d, middleware saw %v, want 403", rec.Code, seen)
	}
	// A form of a different site cannot use the rule: it cannot ask for
	// JSON with a JSON body, and with no Accept header it needs the token.
	req := httptest.NewRequest("POST", "/api/save/7", strings.NewReader("name=tea"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Origin", "http://example.com")
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("a text/plain form with no token: status %d, want 403", rec.Code)
	}
}
