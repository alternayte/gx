// Package round5_test holds the blocking findings of review round 5 for
// release 0.3.0 (SDD §16.3). Every test fails on the reviewed HEAD 017be3a.
package round5_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// noAdapter is an adapter that answers with a status only.
type noAdapter struct{}

func (noAdapter) Name() string              { return "none" }
func (noAdapter) Signals() bool             { return true }
func (noAdapter) Runtime() gx.Node          { return nil }
func (noAdapter) Assets() map[string][]byte { return nil }
func (noAdapter) Respond(w http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	return nil
}
func (noAdapter) ReadSignals(*http.Request, any) error { return nil }
func (noAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-x-on", Value: method + " " + url}
}
func (noAdapter) On(inv gx.Invocation) []gx.Attr {
	return []gx.Attr{{Key: "data-x-on-" + inv.Event, Value: inv.Method + " " + inv.URL}}
}

// addIn is a hand-written tool input. A generated route type has the same
// methods.
type addIn struct {
	ID  int64
	Qty int
}

func (addIn) Pattern() string { return "POST /cart/{id}/add" }

func (in *addIn) Bind(r *http.Request) error {
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

func (in *addIn) Rules() gx.Rules { return gx.Rules{gx.Field(&in.Qty, gx.Min(1))} }

func (in *addIn) GxFieldName(ptr any) string {
	if ptr == &in.Qty {
		return "qty"
	}
	return ""
}

func (addIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "cart_add", Description: "Adds a product to the cart.",
		Schema: `{"additionalProperties":false,"properties":{"id":{"type":"integer"},"qty":{"minimum":1,"type":"integer"}},"required":["id"],"type":"object"}`,
		Fields: []gx.ToolField{{Name: "id", In: "path"}, {Name: "qty", In: "form"}}}
}

// TestSI_07_ToolCallIsTheRequestOfTheCaller checks that the middleware of
// the group of an action sees a tool call as it sees a user request (SI-07:
// "Tool calls pass the same auth and cross-origin checks as user requests").
// A middleware that decides by the address of the client, by the host of the
// request (a tenant) or by the TLS state (a client certificate) must get the
// values of the request of the caller.
func TestSI_07_ToolCallIsTheRequestOfTheCaller(t *testing.T) {
	var host, remote string
	var hasTLS, ran bool
	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, remote, hasTLS = r.Host, r.RemoteAddr, r.TLS != nil
			next.ServeHTTP(w, r)
		})
	}
	add := gx.Action(func(c *gx.Ctx, in addIn) error { ran = true; return nil }).Tool()
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", guard, gx.Collect(add))

	req := httptest.NewRequest("POST", "https://tenant-a.app.example/_gx/tools/cart_add", strings.NewReader(`{"id":7,"qty":2}`))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:4711"
	req.TLS = &tls.ConnectionState{}
	app.ServeHTTP(httptest.NewRecorder(), req)
	if !ran {
		t.Fatal("the tool call did not run the action")
	}
	if host != "tenant-a.app.example" {
		t.Errorf("the middleware of the group saw the host %q for a tool call; the request of the caller has tenant-a.app.example", host)
	}
	if remote != "203.0.113.9:4711" {
		t.Errorf("the middleware of the group saw the client address %q for a tool call; the request of the caller has 203.0.113.9:4711", remote)
	}
	if !hasTLS {
		t.Error("the middleware of the group saw no TLS state for a tool call; the request of the caller has one")
	}
}

// TestREQ_AI_07_ToolCallUnderBasePath checks that a tool call runs the
// action in an app with Config.BasePath (REQ-RTE-18: the app is mounted
// under a prefix, and its routes have no prefix). The tools of the app are
// the same tools over MCP (REQ-AI-07) and in the page (REQ-AI-06).
func TestREQ_AI_07_ToolCallUnderBasePath(t *testing.T) {
	defer gx.SetBasePath("")
	var seen []addIn
	add := gx.Action(func(c *gx.Ctx, in addIn) error { seen = append(seen, in); return nil }).Tool()
	app := gx.New(gx.Config{Adapter: noAdapter{}, BasePath: "/shop"})
	app.Group("/", gx.Collect(add))

	// The action answers at its own path under the app: the mount strips
	// the base path.
	direct := httptest.NewRequest("POST", "https://app.example/cart/7/add", strings.NewReader("qty=1"))
	direct.Header.Set("Sec-Fetch-Site", "same-origin")
	direct.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, direct)
	if len(seen) != 1 {
		t.Fatalf("the fixture is wrong: the action did not run at /cart/7/add (status %d)", rec.Code)
	}

	answer := app.CallTool(context.Background(), http.Header{}, "cart_add", json.RawMessage(`{"id":7,"qty":3}`))
	if answer.IsError || len(seen) != 2 {
		t.Fatalf("a tool call in an app with BasePath /shop did not run the action: %q (handler runs %d, want 2)", answer.Text, len(seen))
	}
	if seen[1] != (addIn{ID: 7, Qty: 3}) {
		t.Errorf("the action got %+v", seen[1])
	}
}

// secretIn is the input of a tool that answers with a typed result.
type secretIn struct{}

func (secretIn) Pattern() string           { return "POST /keys" }
func (*secretIn) Bind(*http.Request) error { return nil }
func (secretIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{Name: "keys_list", Description: "Lists the keys.", Schema: `{"type":"object"}`}
}

// TestSI_04_SecretMapKeyInToolResult checks that a gx.Secret does not reach
// an agent in a tool result (SI-04: "cannot enter ... tool results ...
// redaction and dev panic at runtime otherwise"). encoding/json writes a map
// key of a string type as its text and does not call MarshalJSON.
func TestSI_04_SecretMapKeyInToolResult(t *testing.T) {
	const key = "sk-live-hunter2"
	list := gx.Action(func(c *gx.Ctx, in secretIn) error {
		gx.ToolResult(c, map[gx.Secret]bool{gx.Secret(key): true})
		return nil
	}).Tool()
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.Collect(list))
	answer := app.CallTool(context.Background(), http.Header{}, "keys_list", nil)
	if strings.Contains(answer.Text, key) || strings.Contains(string(answer.Structured), key) {
		t.Errorf("the tool result holds the secret: text %s, structured %s", answer.Text, answer.Structured)
	}
}

// Two groups mount two methods of one path, each with its own origins.
type shareGet struct{}

func (shareGet) Pattern() string           { return "GET /w/cart" }
func (*shareGet) Bind(*http.Request) error { return nil }

type sharePost struct{}

func (sharePost) Pattern() string           { return "POST /w/cart" }
func (*sharePost) Bind(*http.Request) error { return nil }

// TestREQ_ISL_22_PreflightUsesTheGroupOfTheMethod checks the preflight of a
// path that two groups mount with two methods and two origin lists
// (REQ-ISL-22: "The group answers the CORS preflight and sets the CORS
// headers for a listed origin"). The preflight names the method of the
// request, so the origins of the group of that method decide.
func TestREQ_ISL_22_PreflightUsesTheGroupOfTheMethod(t *testing.T) {
	get := gx.Action(func(c *gx.Ctx, in shareGet) error { return nil })
	post := gx.Action(func(c *gx.Ctx, in sharePost) error { return nil })
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.AllowOrigins("https://reader.example"), gx.Collect(get))
	app.Group("/", gx.AllowOrigins("https://writer.example"), gx.Collect(post))

	send := func(method, origin, asks string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://api.acme.dev/w/cart", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		if asks != "" {
			req.Header.Set("Access-Control-Request-Method", asks)
			req.Header.Set("Access-Control-Request-Headers", "authorization,gx-widget")
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}
	// The request itself: the list of the group of its method decides.
	if rec := send("GET", "https://reader.example", ""); rec.Code == http.StatusForbidden {
		t.Fatalf("the fixture is wrong: GET of the listed origin = %d", rec.Code)
	}
	if rec := send("GET", "https://writer.example", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("the fixture is wrong: GET of an origin of the other group = %d, want 403", rec.Code)
	}
	// The preflight of that request. A widget with a token sends one
	// before its GET.
	rec := send("OPTIONS", "https://reader.example", "GET")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://reader.example" {
		t.Errorf("preflight for GET from the origin that the group of GET lists = %d with Access-Control-Allow-Origin %q; want 204 and the origin",
			rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	rec = send("OPTIONS", "https://writer.example", "GET")
	if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("preflight for GET from an origin that the group of GET does not list = %d with Access-Control-Allow-Origin %q; want 403 and no header",
			rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

// Widget fixtures for the build hash.
type buildWidgetIn struct{}

func (buildWidgetIn) Pattern() string           { return "GET /w/box" }
func (buildWidgetIn) GxWidgetAttrs() []string   { return nil }
func (*buildWidgetIn) Bind(*http.Request) error { return nil }

type quietIn struct{}

func (quietIn) Pattern() string           { return "POST /w/quiet" }
func (*quietIn) Bind(*http.Request) error { return nil }

type ruledIn struct{ Qty int }

func (ruledIn) Pattern() string           { return "POST /w/ruled" }
func (*ruledIn) Bind(*http.Request) error { return nil }
func (in *ruledIn) Rules() gx.Rules       { return gx.Rules{gx.Field(&in.Qty, gx.Min(1))} }
func (in *ruledIn) GxFieldName(ptr any) string {
	if ptr == &in.Qty {
		return "qty"
	}
	return ""
}

// TestREQ_ISL_19_EachAnswerCarriesTheBuild checks that each answer to a
// widget names the build of the server (REQ-ISL-19: "Each answer carries the
// hash of the build"; acceptance: "An open widget whose server changes build
// does a fresh first render at its next action"). The next action of an open
// widget can be an action with no patch, or one that a rule refuses.
func TestREQ_ISL_19_EachAnswerCarriesTheBuild(t *testing.T) {
	box := gx.Widget(func(c *gx.Ctx, in buildWidgetIn) (struct{}, error) { return struct{}{}, nil },
		func(struct{}) gx.Node { return gx.El("p", nil, gx.Text("box")) }).Tag("acme-box")
	quiet := gx.Action(func(c *gx.Ctx, in quietIn) error { return nil })
	ruled := gx.Action(func(c *gx.Ctx, in ruledIn) error { return nil })
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.AllowOrigins("https://host.example"), gx.Collect(box, quiet, ruled))

	send := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://api.acme.dev"+path, strings.NewReader(`{"signals":{}}`))
		req.Header.Set("Origin", "https://host.example")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Gx-Widget", "acme-box")
		req.Header.Set("Accept", "application/json")
		if method != "GET" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec
	}
	var first struct {
		Build string `json:"build"`
	}
	rec := send("GET", "/w/box")
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil || first.Build == "" {
		t.Fatalf("the first answer has no build: status %d, %s", rec.Code, rec.Body.String())
	}
	carries := func(rec *httptest.ResponseRecorder) bool {
		if strings.Contains(rec.Body.String(), first.Build) {
			return true
		}
		for _, values := range rec.Header() {
			for _, v := range values {
				if strings.Contains(v, first.Build) {
					return true
				}
			}
		}
		return false
	}
	if rec := send("POST", "/w/quiet"); !carries(rec) {
		t.Errorf("the answer of an action with no patch (status %d) does not name the build %s: headers %v, body %q", rec.Code, first.Build, rec.Header(), rec.Body.String())
	}
	if rec := send("POST", "/w/ruled"); !carries(rec) {
		t.Errorf("the answer of an action that a rule refuses (status %d) does not name the build %s: headers %v, body %q", rec.Code, first.Build, rec.Header(), rec.Body.String())
	}
}

// TestREQ_AI_07_ToolCallKeepsALargeInteger checks that an integer argument
// of a tool reaches the action with its value (REQ-AI-06: "input schema from
// the input struct"; the schema says "type": "integer" for an int64 field,
// and D-288: "A tool call is the request of its action"). An id above 2^53,
// such as a snowflake id, must not change on the way: the action would run
// on a different record.
func TestREQ_AI_07_ToolCallKeepsALargeInteger(t *testing.T) {
	const id = int64(1234567890123456789)
	var seen []addIn
	add := gx.Action(func(c *gx.Ctx, in addIn) error { seen = append(seen, in); return nil }).Tool()
	app := gx.New(gx.Config{Adapter: noAdapter{}})
	app.Group("/", gx.Collect(add))

	answer := app.CallTool(context.Background(), http.Header{}, "cart_add", json.RawMessage(`{"id":1234567890123456789,"qty":1}`))
	if answer.IsError || len(seen) != 1 {
		t.Fatalf("the call over CallTool did not run the action: %q", answer.Text)
	}
	if seen[0].ID != id {
		t.Errorf("CallTool: the action got the id %d for the argument %d", seen[0].ID, id)
	}

	req := httptest.NewRequest("POST", "https://app.example/_gx/tools/cart_add", strings.NewReader(`{"id":1234567890123456789,"qty":1}`))
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(httptest.NewRecorder(), req)
	if len(seen) != 2 {
		t.Fatalf("the call of the page did not run the action")
	}
	if seen[1].ID != id {
		t.Errorf("POST /_gx/tools/cart_add: the action got the id %d for the argument %d", seen[1].ID, id)
	}
}
