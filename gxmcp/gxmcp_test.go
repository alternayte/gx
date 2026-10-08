package gxmcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/gxmcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toolAdd is a hand-written tool input. A generated route type has the same
// methods.
type toolAdd struct {
	ID    int64
	Tab   string
	Qty   int
	Email string
	Lines []toolLine
}

type toolLine struct {
	SKU string
	Qty int
}

func (toolAdd) Pattern() string { return "POST /products/{id}/cart" }

func (in *toolAdd) Bind(r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return err
	}
	in.ID = id
	in.Tab = "overview"
	if v := r.URL.Query().Get("tab"); v != "" {
		in.Tab = v
	}
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	if err := gx.BindSignal(signals, gx.Scope(r), "qty", &in.Qty); err != nil {
		return err
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	in.Email = r.PostForm.Get("email")
	for i := 0; ; i++ {
		sku, ok := r.PostForm["lines["+strconv.Itoa(i)+"].sKU"]
		if !ok {
			break
		}
		qty, err := strconv.Atoi(r.PostForm.Get("lines[" + strconv.Itoa(i) + "].qty"))
		if err != nil {
			return err
		}
		in.Lines = append(in.Lines, toolLine{SKU: sku[0], Qty: qty})
	}
	return nil
}

func (in *toolAdd) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Min(1), gx.Max(99)), gx.Field(&in.Email, gx.Required)}
}

func (in *toolAdd) GxFieldName(ptr any) string {
	switch ptr {
	case &in.Qty:
		return "qty"
	case &in.Email:
		return "email"
	}
	return ""
}

const toolAddSchema = `{"additionalProperties":false,"properties":{"email":{"type":"string"},"id":{"type":"integer"},"lines":{"items":{"additionalProperties":false,"properties":{"qty":{"type":"integer"},"sKU":{"type":"string"}},"type":"object"},"type":"array"},"qty":{"maximum":99,"minimum":1,"type":"integer"},"tab":{"default":"overview","type":"string"}},"required":["id","email"],"type":"object"}`

func (toolAdd) GxTool() gx.ToolInfo {
	return gx.ToolInfo{
		Name: "products_add_to_cart", Description: "Adds a product to the cart of the user.", Schema: toolAddSchema,
		Fields: []gx.ToolField{{Name: "id", In: "path"}, {Name: "tab", In: "query"}, {Name: "qty", In: "signal"}, {Name: "email", In: "form"}, {Name: "lines", In: "form"}},
	}
}

// toolPlain is the input of an action that is not a tool.
type toolPlain struct{}

func (toolPlain) Pattern() string           { return "POST /plain" }
func (*toolPlain) Bind(*http.Request) error { return nil }

// toolApp is an app with one tool, one action that is not a tool, an MCP
// endpoint behind a token, and a group that needs the same token.
type toolApp struct {
	url  string
	seen []toolAdd
	// answer is what the handler of the tool does.
	answer func(c *gx.Ctx, in toolAdd) error
	plain  int
}

func needToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer token-of-") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// groupHost and groupRemote are the host and the client address that the
// middleware of the group saw last.
var groupHost, groupRemote string

// adminOnly is the auth rule of the group of the tool: a user request and a
// tool call pass it in the same way.
func adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groupHost, groupRemote = r.Host, r.RemoteAddr
		if r.Header.Get("Authorization") != "Bearer token-of-admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newToolApp(t *testing.T) *toolApp {
	t.Helper()
	ta := &toolApp{}
	add := gx.Action(func(c *gx.Ctx, in toolAdd) error {
		ta.seen = append(ta.seen, in)
		if ta.answer != nil {
			return ta.answer(c, in)
		}
		return nil
	}).Tool()
	plain := gx.Action(func(c *gx.Ctx, in toolPlain) error {
		ta.plain++
		return nil
	})
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/shop", adminOnly, gx.Collect(add, plain))
	gxmcp.Mount(app, "/mcp", needToken)
	srv := httptest.NewServer(app)
	t.Cleanup(srv.Close)
	ta.url = srv.URL
	return ta
}

// headerClient adds headers to each request of an MCP client.
type headerClient struct{ header http.Header }

func (h headerClient) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h.header {
		r.Header[k] = v
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (ta *toolApp) connect(t *testing.T, header http.Header) (*mcp.ClientSession, error) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   ta.url + "/mcp",
		HTTPClient: &http.Client{Transport: headerClient{header}},
	}, nil)
	if err == nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	return session, err
}

func (ta *toolApp) session(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	session, err := ta.connect(t, http.Header{"Authorization": {"Bearer token-of-" + token}})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func textOf(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

var toolArgs = map[string]any{
	"id": 42, "tab": "reviews", "qty": 3, "email": "a@b.example",
	"lines": []any{map[string]any{"sKU": "A-1", "qty": 2}, map[string]any{"sKU": "B & C", "qty": 1}},
}

// TestREQ_AI_07_MCPEndpoint checks gxmcp.Mount: an MCP client lists the tools of
// the app over streamable HTTP and calls one, the arguments reach the
// handler through the binder, and a call with no auth is rejected.
func TestREQ_AI_07_MCPEndpoint(t *testing.T) {
	ta := newToolApp(t)
	session := ta.session(t, "admin")
	list, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 1 {
		t.Fatalf("tools = %d", len(list.Tools))
	}
	tool := list.Tools[0]
	schema, _ := json.Marshal(tool.InputSchema)
	var got, want any
	_ = json.Unmarshal(schema, &got)
	_ = json.Unmarshal([]byte(toolAddSchema), &want)
	gotText, _ := json.Marshal(got)
	wantText, _ := json.Marshal(want)
	if tool.Name != "products_add_to_cart" || tool.Description != "Adds a product to the cart of the user." || string(gotText) != string(wantText) {
		t.Errorf("tool = %s, %q, %s", tool.Name, tool.Description, schema)
	}
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "products_add_to_cart", Arguments: toolArgs})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("the call failed: %s", textOf(res))
	}
	if len(ta.seen) != 1 {
		t.Fatalf("the handler ran %d times", len(ta.seen))
	}
	// The request of the action is the request of the caller: the
	// middleware of the group sees the host and the client address of the
	// MCP request.
	if groupHost != strings.TrimPrefix(ta.url, "http://") || !strings.HasPrefix(groupRemote, "127.0.0.1:") {
		t.Errorf("the middleware of the group saw the host %q and the client address %q", groupHost, groupRemote)
	}
	in := ta.seen[0]
	if in.ID != 42 || in.Tab != "reviews" || in.Qty != 3 || in.Email != "a@b.example" ||
		len(in.Lines) != 2 || in.Lines[0] != (toolLine{"A-1", 2}) || in.Lines[1] != (toolLine{"B & C", 1}) {
		t.Errorf("input = %+v", in)
	}

	// The rules of the input run for a tool call: the handler does not run,
	// and the answer names the field and the key.
	bad := map[string]any{"id": 1, "qty": 500, "email": "a@b.example"}
	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "products_add_to_cart", Arguments: bad})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(ta.seen) != 1 || !strings.Contains(textOf(res), "qty") {
		t.Errorf("a call that breaks a rule: error %v, text %q, handler runs %d", res.IsError, textOf(res), len(ta.seen))
	}

	// A call with no auth does not reach the endpoint.
	if _, err := ta.connect(t, nil); err == nil {
		t.Error("a client with no token connected")
	}
	plain, err := http.Post(ta.url+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatal(err)
	}
	plain.Body.Close()
	if plain.StatusCode != http.StatusUnauthorized {
		t.Errorf("a request with no token: status %d", plain.StatusCode)
	}
}

// TestREQ_AI_08_ToolResult checks the output of a tool: gx.ToolResult sets a
// structured value, and an action with none gives a summary of its patches.
func TestREQ_AI_08_ToolResult(t *testing.T) {
	ta := newToolApp(t)
	session := ta.session(t, "admin")
	call := func() *mcp.CallToolResult {
		t.Helper()
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "products_add_to_cart", Arguments: toolArgs})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	type added struct {
		Count int    `json:"count"`
		SKU   string `json:"sku"`
	}
	ta.answer = func(c *gx.Ctx, in toolAdd) error {
		gx.ToolResult(c, added{Count: in.Qty, SKU: in.Lines[0].SKU})
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "cart-count"}}, gx.Text("3")))
	}
	res := call()
	structured, _ := json.Marshal(res.StructuredContent)
	if res.IsError || string(structured) != `{"count":3,"sku":"A-1"}` || textOf(res) != `{"count":3,"sku":"A-1"}` {
		t.Errorf("a typed result: error %v, structured %s, text %q", res.IsError, structured, textOf(res))
	}

	ta.answer = func(c *gx.Ctx, in toolAdd) error {
		if err := c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "cart-count"}}, gx.Text("3"))); err != nil {
			return err
		}
		if err := c.Toast("Added to the cart"); err != nil {
			return err
		}
		return c.SetSignals(struct {
			Qty int `json:"qty"`
		}{Qty: 1})
	}
	res = call()
	text := textOf(res)
	for _, want := range []string{"#cart-count", "Added to the cart", `{"qty":1}`} {
		if !strings.Contains(text, want) {
			t.Errorf("the summary has no %q: %q", want, text)
		}
	}
	if res.IsError || res.StructuredContent != nil {
		t.Errorf("a summary: error %v, structured %v", res.IsError, res.StructuredContent)
	}

	// An action with no answer says so.
	ta.answer = nil
	if res = call(); res.IsError || textOf(res) == "" {
		t.Errorf("an action with no patch: error %v, text %q", res.IsError, textOf(res))
	}
	// An error of the handler is an error of the tool.
	ta.answer = func(c *gx.Ctx, in toolAdd) error { return errors.New("the product is not in stock") }
	if res = call(); !res.IsError || !strings.Contains(textOf(res), "the product is not in stock") {
		t.Errorf("a handler error: error %v, text %q", res.IsError, textOf(res))
	}
}

// TestSI_07_OnlyToolsAreTools checks the security rule of tools: an action
// with no Tool is not a tool, and a tool call passes the middleware of the
// group of its action and the cross-origin check, as a user request does.
func TestSI_07_OnlyToolsAreTools(t *testing.T) {
	ta := newToolApp(t)
	admin := ta.session(t, "admin")
	// The action with no Tool is not in the list and has no call.
	res, err := admin.CallTool(context.Background(), &mcp.CallToolParams{Name: "plain", Arguments: map[string]any{}})
	if err == nil && !res.IsError {
		t.Error("an action with no Tool ran as a tool")
	}
	if ta.plain != 0 {
		t.Errorf("the action with no Tool ran %d times", ta.plain)
	}

	// A user who passes the auth of the endpoint, and not the auth of the
	// group, cannot run the tool: the middleware of the group refuses the
	// call as it refuses the request of that user.
	user := ta.session(t, "user")
	res, err = user.CallTool(context.Background(), &mcp.CallToolParams{Name: "products_add_to_cart", Arguments: toolArgs})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || len(ta.seen) != 0 || !strings.Contains(textOf(res), "403") {
		t.Errorf("a call of a user that the group refuses: error %v, text %q, handler runs %d", res.IsError, textOf(res), len(ta.seen))
	}
	direct, _ := http.NewRequest("POST", ta.url+"/shop/products/42/cart", strings.NewReader("email=a%40b.example"))
	direct.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	direct.Header.Set("Authorization", "Bearer token-of-user")
	answer, err := http.DefaultClient.Do(direct)
	if err != nil {
		t.Fatal(err)
	}
	answer.Body.Close()
	if answer.StatusCode != http.StatusForbidden {
		t.Errorf("the request of the same user: status %d", answer.StatusCode)
	}

	// A page of a different site cannot call the endpoint with the
	// credentials of the user.
	cross, _ := http.NewRequest("POST", ta.url+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"products_add_to_cart","arguments":{"id":1,"email":"a@b.example"}}}`))
	cross.Header.Set("Content-Type", "application/json")
	cross.Header.Set("Accept", "application/json, text/event-stream")
	cross.Header.Set("Authorization", "Bearer token-of-admin")
	cross.Header.Set("Origin", "https://evil.example")
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	answer, err = http.DefaultClient.Do(cross)
	if err != nil {
		t.Fatal(err)
	}
	answer.Body.Close()
	if answer.StatusCode != http.StatusForbidden || len(ta.seen) != 0 {
		t.Errorf("a cross-site call: status %d, handler runs %d", answer.StatusCode, len(ta.seen))
	}

	// Two tools with one name stop the app at its start.
	defer func() {
		if recover() == nil {
			t.Error("two tools with one name gave no panic")
		}
	}()
	app := gx.New(gx.Config{})
	one := gx.Action(func(c *gx.Ctx, in toolAdd) error { return nil }).Tool()
	app.Group("/a", gx.Collect(one))
	app.Group("/b", gx.Collect(gx.Action(func(c *gx.Ctx, in toolAdd) error { return nil }).Tool()))
}
