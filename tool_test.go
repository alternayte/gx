package gx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// pageToolIn is a hand-written tool input. A generated route type has the
// same methods.
type pageToolIn struct {
	ID  int64
	Qty int
}

func (pageToolIn) Pattern() string { return "POST /cart/{id}/add" }

func (in *pageToolIn) Bind(r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return err
	}
	in.ID = id
	if err := r.ParseForm(); err != nil {
		return err
	}
	if v := r.PostForm.Get("qty"); v != "" {
		if in.Qty, err = strconv.Atoi(v); err != nil {
			return err
		}
	}
	return nil
}

func (in *pageToolIn) Rules() gx.Rules { return gx.Rules{gx.Field(&in.Qty, gx.Min(1))} }

func (in *pageToolIn) GxFieldName(ptr any) string {
	if ptr == &in.Qty {
		return "qty"
	}
	return ""
}

const pageToolSchema = `{"additionalProperties":false,"properties":{"id":{"type":"integer"},"qty":{"minimum":1,"type":"integer"}},"required":["id"],"type":"object"}`

func (pageToolIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "cart_add", Description: "Adds a product to the cart.", Schema: pageToolSchema,
		Fields: []gx.ToolField{{Name: "id", In: "path"}, {Name: "qty", In: "form"}}}
}

type pageToolHome struct{}

func (pageToolHome) Pattern() string           { return "GET /{$}" }
func (*pageToolHome) Bind(*http.Request) error { return nil }

type pageToolPlain struct{}

func (pageToolPlain) Pattern() string           { return "GET /plain" }
func (*pageToolPlain) Bind(*http.Request) error { return nil }

// TestREQ_AI_06_PageToolRoutes checks the server part of the tools of a
// page: a page with an element that invokes a tool loads the tool module,
// the module gets the description of the tool, and a call runs the action
// and answers for the page and for the agent.
func TestREQ_AI_06_PageToolRoutes(t *testing.T) {
	adapter := &fakeAdapter{}
	var seen []pageToolIn
	add := gx.Action(func(c *gx.Ctx, in pageToolIn) error {
		seen = append(seen, in)
		gx.ToolResult(c, map[string]int{"count": in.Qty})
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text(strconv.Itoa(in.Qty))))
	}).Tool(gx.Confirm)
	withTool := gx.Page(func(c *gx.Ctx, in pageToolHome) (struct{}, error) { return struct{}{}, nil }, func(struct{}) gx.Node {
		return gx.El("button", gx.Attrs{{Key: "data-gx-tool", Value: "cart_add"}, gx.On("click", "POST", "/cart/7/add", "")}, gx.Text("Add"))
	})
	plain := gx.Page(func(c *gx.Ctx, in pageToolPlain) (struct{}, error) { return struct{}{}, nil }, func(struct{}) gx.Node {
		return gx.El("p", nil, gx.Text("No tool here"))
	})
	app := gx.New(gx.Config{Adapter: adapter})
	app.Group("/", gx.Collect(add, withTool, plain))
	do := func(method, target, body string, header map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://app.example"+target, strings.NewReader(body))
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		for k, v := range header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}

	// The page with the tool loads the tool module; the page with none
	// does not.
	if page := do("GET", "/", "", nil).Body.String(); !strings.Contains(page, `/_gx/tool.js"`) || !strings.Contains(page, `data-gx-tool="cart_add"`) {
		t.Errorf("the page with a tool has no tool module:\n%s", page)
	}
	if page := do("GET", "/plain", "", nil).Body.String(); strings.Contains(page, "tool.js") {
		t.Errorf("the page with no tool loads the tool module:\n%s", page)
	}
	if rec := do("GET", "/_gx/tool.js", "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "modelContext") {
		t.Errorf("the tool module: status %d", rec.Code)
	}

	// The description of the tool.
	var meta struct {
		Name, Description string
		InputSchema       json.RawMessage
		Confirm, ReadOnly bool
	}
	rec := do("GET", "/_gx/tools/cart_add", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatalf("status %d: %v\n%s", rec.Code, err, rec.Body.String())
	}
	if meta.Name != "cart_add" || meta.Description != "Adds a product to the cart." || string(meta.InputSchema) != pageToolSchema || !meta.Confirm || meta.ReadOnly {
		t.Errorf("description = %+v", meta)
	}
	if rec := do("GET", "/_gx/tools/cart_nothing", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("the description of no tool: status %d", rec.Code)
	}

	// A call runs the action. The answer for the page goes through the
	// adapter, and the answer for the agent is in a header.
	answerOf := func(rec *httptest.ResponseRecorder) (a struct {
		Text       string
		Structured json.RawMessage
		IsError    bool
	}) {
		t.Helper()
		raw, err := url.PathUnescape(rec.Header().Get("Gx-Tool-Answer"))
		if err != nil || raw == "" {
			t.Fatalf("no answer for the agent: status %d, header %q", rec.Code, rec.Header().Get("Gx-Tool-Answer"))
		}
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	stream := map[string]string{"Content-Type": "application/json", "Accept": "text/event-stream", "Datastar-Request": "true"}
	rec = do("POST", "/_gx/tools/cart_add", `{"id":7,"qty":3}`, stream)
	a := answerOf(rec)
	if a.IsError || string(a.Structured) != `{"count":3}` || len(seen) != 1 || seen[0] != (pageToolIn{ID: 7, Qty: 3}) {
		t.Errorf("a call: answer %+v, seen %+v", a, seen)
	}
	if !adapter.responded || adapter.res == nil || len(adapter.res.Patches) != 1 {
		t.Errorf("the adapter did not get the patch of the action for the page")
	}
	// A call that breaks a rule does not run the handler.
	rec = do("POST", "/_gx/tools/cart_add", `{"id":7,"qty":0}`, stream)
	if a = answerOf(rec); !a.IsError || !strings.Contains(a.Text, "qty") || len(seen) != 1 {
		t.Errorf("a call that breaks a rule: %+v, handler runs %d", a, len(seen))
	}
	// A page of a different site cannot call a tool with the cookies of the
	// user.
	cross := map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}
	if rec = do("POST", "/_gx/tools/cart_add", `{"id":7,"qty":3}`, cross); rec.Code != http.StatusForbidden || len(seen) != 1 {
		t.Errorf("a cross-site call: status %d, handler runs %d", rec.Code, len(seen))
	}

	// An app with no tool has no tool route.
	bare := gx.New(gx.Config{Adapter: adapter})
	bare.Group("/", gx.Collect(plain))
	req := httptest.NewRequest("GET", "https://app.example/_gx/tool.js", nil)
	out := httptest.NewRecorder()
	bare.ServeHTTP(out, req)
	if out.Code != http.StatusNotFound {
		t.Errorf("an app with no tool serves the tool module: status %d", out.Code)
	}
}

// toolFixture is an app with the cart_add tool in a group with a guard, and
// a page with no tool.
func toolFixture(t *testing.T, guard func(http.Handler) http.Handler, opts ...gx.ToolOption) (app *gx.App, seen *[]pageToolIn) {
	t.Helper()
	seen = &[]pageToolIn{}
	add := gx.Action(func(c *gx.Ctx, in pageToolIn) error {
		*seen = append(*seen, in)
		return nil
	}).Tool(opts...)
	app = gx.New(gx.Config{Adapter: &fakeAdapter{}})
	if guard != nil {
		app.Group("/", guard, gx.Collect(add))
	} else {
		app.Group("/", gx.Collect(add))
	}
	return app, seen
}

// TestSI_03_PageToolCallNeedsJSON checks that the tool route of a page takes
// its arguments only as application/json. A form of a different site can
// send text/plain with no preflight; a browser with no Fetch Metadata and no
// Origin header then gives no sign of the other site.
func TestSI_03_PageToolCallNeedsJSON(t *testing.T) {
	app, seen := toolFixture(t, nil)
	send := func(contentType string) int {
		req := httptest.NewRequest("POST", "https://app.example/_gx/tools/cart_add", strings.NewReader(`{"id":7,"qty":3}`))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, ct := range []string{"text/plain", "text/plain;charset=UTF-8", "", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		if code := send(ct); code < 400 || len(*seen) != 0 {
			t.Errorf("a tool call with the Content-Type %q: status %d, handler runs %d", ct, code, len(*seen))
		}
	}
	if code := send("application/json"); code >= 400 || len(*seen) != 1 {
		t.Errorf("a tool call with application/json: status %d, handler runs %d", code, len(*seen))
	}
	if code := send("application/json; charset=utf-8"); code >= 400 || len(*seen) != 2 {
		t.Errorf("a tool call with application/json and a charset: status %d, handler runs %d", code, len(*seen))
	}
}

// TestSI_03_TextPlainFormNeedsTheToken checks the token rule for a browser
// with no Fetch Metadata: a POST with the form encoding text/plain is a form
// of a page, as the two other form encodings are, so it needs the token.
func TestSI_03_TextPlainFormNeedsTheToken(t *testing.T) {
	ran := 0
	h := gx.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ran++ }))
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		req := httptest.NewRequest("POST", "https://app.example/cart/clear", strings.NewReader("a=b"))
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("a POST with the form encoding %q, no Fetch Metadata and no token: status %d", ct, rec.Code)
		}
	}
	if ran != 0 {
		t.Errorf("the handler ran %d times", ran)
	}
	// A client that is not a browser sends JSON with no token.
	req := httptest.NewRequest("POST", "https://app.example/cart/clear", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ran != 1 {
		t.Errorf("a JSON request of a client with no browser shape ran the handler %d times", ran)
	}
}

// TestSI_07_ToolDescriptionPassesTheGroup checks that the description of a
// tool goes only to a caller that the middleware of the group of the tool
// lets pass: the name, the description and the schema of a tool say what the
// app can do for a user with that access.
func TestSI_07_ToolDescriptionPassesTheGroup(t *testing.T) {
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer token-of-admin" {
				http.Error(w, "sign in first", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	app, seen := toolFixture(t, guard)
	get := func(auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "https://app.example/_gx/tools/cart_add", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}
	rec := get("")
	if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "Adds a product") || strings.Contains(rec.Body.String(), "inputSchema") {
		t.Errorf("the description for a caller with no auth: status %d, body %q", rec.Code, rec.Body.String())
	}
	rec = get("Bearer token-of-admin")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Adds a product to the cart.") {
		t.Errorf("the description for a caller with auth: status %d, body %q", rec.Code, rec.Body.String())
	}
	// A read of the description does not run the action.
	if len(*seen) != 0 {
		t.Errorf("a read of the description ran the action %d times", len(*seen))
	}
}

// TestREQ_AI_07_ToolsSayConfirm checks that the tool list of the app says
// which tool has gx.Confirm and which is read-only, so a server for agents
// can tell its clients.
func TestREQ_AI_07_ToolsSayConfirm(t *testing.T) {
	plain, _ := toolFixture(t, nil)
	if tools := plain.Tools(); len(tools) != 1 || tools[0].Confirm || tools[0].ReadOnly {
		t.Errorf("a tool with no option = %+v", tools)
	}
	confirm, _ := toolFixture(t, nil, gx.Confirm)
	if tools := confirm.Tools(); len(tools) != 1 || !tools[0].Confirm {
		t.Errorf("a tool with gx.Confirm = %+v", tools)
	}
}
