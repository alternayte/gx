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
