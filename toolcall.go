package gx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// mountedTool is one tool of the app with the mounted route of its action.
type mountedTool struct {
	def *toolDef
	// handler is the action with the middleware of its group, as the
	// routes of the app have it. A tool call runs this handler and no
	// other: no argument of the agent selects a route (SI-07).
	handler http.Handler
	method  string
	// path is the mounted path of the route, with its pattern variables.
	path string
}

// addTool records the tool of a mounted action or form.
func (a *App) addTool(def *toolDef, pattern string, handler http.Handler) {
	name := def.info.Name
	if _, ok := a.tools[name]; ok {
		panic("gx: two tools have the name " + name)
	}
	if a.tools == nil {
		a.tools = map[string]mountedTool{}
	}
	method, path, _ := strings.Cut(pattern, " ")
	if len(a.tools) == 0 {
		// The first tool of the app: serve the tool module and the tool
		// routes of a page. An app with no tool has no such route.
		a.registerAsset("tool.js", toolRuntimeJS)
		a.mux.Handle("GET /_gx/tools/{name}", http.HandlerFunc(a.serveToolInfo))
		a.mux.Handle("POST /_gx/tools/{name}", http.HandlerFunc(a.servePageTool))
	}
	a.tools[name] = mountedTool{def: def, handler: handler, method: method, path: path}
}

// serveToolInfo answers with the description of one tool, for the tool
// module of a page (REQ-AI-06).
func (a *App) serveToolInfo(w http.ResponseWriter, r *http.Request) {
	t, ok := a.tools[r.PathValue("name")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	// The description says what the app can do for a caller with the
	// access of the group of the tool. The request goes through the
	// middleware of that group, and stops before the action: a caller that
	// the group refuses gets the answer of the middleware (SI-07).
	probe, tc := t.toolRequest(r.Context(), r, r.Header, nil, false)
	tc.describe = true
	rec := &statusRecorder{header: http.Header{}}
	t.handler.ServeHTTP(rec, probe.WithContext(context.WithValue(probe.Context(), csrfCheckedKey{}, true)))
	if !tc.done {
		http.Error(w, http.StatusText(rec.status()), rec.status())
		return
	}
	data, err := json.Marshal(struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
		Confirm     bool            `json:"confirm,omitempty"`
		ReadOnly    bool            `json:"readOnly,omitempty"`
	}{t.def.info.Name, t.def.info.Description, json.RawMessage(t.def.info.Schema), t.def.confirm, t.readOnly()})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	// The answer depends on the caller.
	h.Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// readOnly reports whether the tool reads and does not change: its action
// has the method GET.
func (t mountedTool) readOnly() bool {
	return t.method == http.MethodGet || t.method == http.MethodHead
}

// servePageTool runs one tool for the tool module of a page (REQ-AI-06).
// The body is the JSON object of the arguments. The request of the action
// has the headers of this request, so it is the request of the user of the
// page: the middleware of the group of the action runs (SI-07). The answer
// is the answer of the action for the page, with the answer for the agent in
// a header.
func (a *App) servePageTool(w http.ResponseWriter, r *http.Request) {
	t, ok := a.tools[r.PathValue("name")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	// A page of a different site cannot send application/json with no
	// preflight. Each form encoding, text/plain too, is refused here, also
	// for a browser with no Fetch Metadata and no Origin header (SI-03).
	if ct, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";"); !strings.EqualFold(strings.TrimSpace(ct), "application/json") {
		http.Error(w, "gx: the arguments of a tool are application/json", http.StatusUnsupportedMediaType)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	var args map[string]any
	if err == nil {
		args, err = toolArgs(body)
	}
	if err != nil {
		http.Error(w, "gx: the arguments of a tool are a JSON object", http.StatusBadRequest)
		return
	}
	// This request passed the cross-origin check of the app (SI-03).
	in, _ := t.toolRequest(r.Context(), r, r.Header, args, true)
	// The handler of the tool runs, and no other route of the app.
	t.handler.ServeHTTP(w, in)
}

// toolCall is one call of a tool. The request of the call carries it, and
// the handler of the action puts the outcome into it and writes no answer
// for a browser.
type toolCall struct {
	signals map[string]any
	// done is true when the handler of the action got the request.
	done bool
	res  *Response
	// bad is true for arguments that the binder cannot read.
	bad bool
	// errs holds the field errors of the rules, by field name.
	errs map[string]string
	// describe is true for a read of the description of the tool: the
	// request goes through the middleware of the group and stops before
	// the action.
	describe bool
	// browser is true for a call of the tool module of a page. The handler
	// then writes its answer for the page as for a click, and the answer
	// for the agent goes into a header (REQ-AI-06).
	browser bool
}

// toolAttr marks an element that invokes a tool: its value is the name of
// the tool. The tool module of the runtime registers the tool while such an
// element is in the document (REQ-AI-06).
const toolAttr = "data-gx-tool"

// toolAnswerHeader carries the answer for the agent next to the answer for
// the page.
const toolAnswerHeader = "Gx-Tool-Answer"

// toolAnswerLimit is the size of an answer that fits a header.
const toolAnswerLimit = 6000

// finish ends the tool part of a handler. It reports whether the handler
// stops: a call of an MCP client needs no answer for a page. For a call of
// the tool module it sets the answer for the agent as a header, and the
// handler goes on with its answer for the page.
func (tc *toolCall) finish(w http.ResponseWriter) (stop bool) {
	if !tc.browser {
		return true
	}
	answer := toolAnswer(tc, nil)
	wire := struct {
		Text       string          `json:"text"`
		Structured json.RawMessage `json:"structured,omitempty"`
		IsError    bool            `json:"isError,omitempty"`
	}{answer.Text, answer.Structured, answer.IsError}
	data, _ := json.Marshal(wire)
	if len(data) > toolAnswerLimit {
		// A header has a size limit. The page has the full answer.
		wire.Structured = nil
		wire.Text = "the result has " + strconv.Itoa(len(answer.Text)) + " bytes, which is too large for the answer of a tool in the browser; the page shows the change"
		data, _ = json.Marshal(wire)
	}
	w.Header().Set(toolAnswerHeader, url.PathEscape(string(data)))
	return false
}

type toolCallKey struct{}

// toolCallOf returns the tool call of a request, or nil. Only this package
// can put one into a request, so a client cannot ask for the answer of a
// tool.
func toolCallOf(r *http.Request) *toolCall {
	tc, _ := r.Context().Value(toolCallKey{}).(*toolCall)
	return tc
}

// ToolResult sets the structured output of the tool call that runs the
// action (REQ-AI-08). An agent gets v as JSON. With no ToolResult the agent
// gets a summary of the patches of the action. For a request of a browser
// it does nothing.
func ToolResult(c *Ctx, v any) {
	if c == nil || c.res == nil {
		return
	}
	c.res.tool, c.res.hasTool = v, true
}

// argText writes one JSON value of a tool argument as the text of a path,
// query or form value.
func argText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case nil:
		return ""
	}
	data, _ := json.Marshal(v)
	return string(data)
}

// flattenArg adds a JSON value to form values under the names that the
// form binder reads: "address.street" for a field of an object, "tags[0]"
// for a list element and "lines[0].sku" for a field of a list element.
func flattenArg(out url.Values, name string, v any) {
	switch t := v.(type) {
	case map[string]any:
		for key, sub := range t {
			flattenArg(out, name+"."+key, sub)
		}
	case []any:
		for i, sub := range t {
			flattenArg(out, name+"["+strconv.Itoa(i)+"]", sub)
		}
	default:
		out.Set(name, argText(v))
	}
}

// toolRequest makes the request of the action for one tool call: each
// argument goes to the part of the request that the binder reads it from.
// The request has the headers of the call, so the middleware of the group
// and the cross-origin check see the caller (SI-07).
func (t mountedTool) toolRequest(ctx context.Context, caller *http.Request, header http.Header, args map[string]any, browser bool) (*http.Request, *toolCall) {
	tc := &toolCall{signals: map[string]any{}, browser: browser}
	path := t.path
	// The path values go to the request by name: the handler of the tool
	// runs with no route match, so a value cannot name a different route.
	pathValues := map[string]string{}
	query, form := url.Values{}, url.Values{}
	for _, f := range t.def.info.Fields {
		v, ok := args[f.Name]
		if !ok {
			continue
		}
		switch f.In {
		case "path":
			pathValues[f.Name] = argText(v)
			text := url.PathEscape(argText(v))
			path = strings.NewReplacer("{"+f.Name+"}", text, "{"+f.Name+"...}", text).Replace(path)
		case "query":
			query.Set(f.Name, argText(v))
		case "signal":
			tc.signals[f.Name] = v
		default:
			flattenArg(form, f.Name, v)
		}
	}
	var body io.Reader
	switch t.method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		body = strings.NewReader(form.Encode())
	default:
		// net/http reads a form body only for POST, PUT and PATCH. For
		// each other method the binder reads the form values from the
		// query, as it does for the request of a browser.
		for name, values := range form {
			query[name] = values
		}
	}
	// The routes of the app have no base path: a mount strips it.
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	// The address has a host only so that it is a full address: the routes
	// of the app read the path.
	r, err := http.NewRequestWithContext(context.WithValue(ctx, toolCallKey{}, tc), t.method, "http://tool"+target, body)
	if err != nil {
		tc.bad = true
		r, _ = http.NewRequestWithContext(context.WithValue(ctx, toolCallKey{}, tc), t.method, "http://tool/", nil)
	}
	r.RequestURI = target
	for name, value := range pathValues {
		r.SetPathValue(name, value)
	}
	for name, values := range header {
		switch name {
		case "Content-Type", "Content-Length", "Mcp-Session-Id", "Mcp-Protocol-Version", "Last-Event-Id":
			// These describe the message of the agent, not the call.
		case "Accept":
			// The tool module of a page asks for the answer of a click.
			if browser {
				r.Header[name] = values
			}
		default:
			r.Header[name] = values
		}
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if caller != nil {
		// The request of the action is the request of the caller: a
		// middleware of the group reads the host, the client address and
		// the TLS state as it does for a user request (SI-07).
		r.Host = caller.Host
		r.URL.Host = caller.Host
		r.RemoteAddr = caller.RemoteAddr
		r.TLS = caller.TLS
		if caller.TLS != nil {
			r.URL.Scheme = "https"
		}
	}
	return r, tc
}

// patchSummary says in one line what a patch does to the page.
func patchSummary(p Patch) string {
	switch t := p.(type) {
	case ElementPatch:
		mode := "morphed"
		switch t.Mode {
		case ModeInner:
			mode = "set the content of"
		case ModeAppend:
			mode = "appended to"
		case ModePrepend:
			mode = "prepended to"
		case ModeReplace:
			mode = "replaced"
		case ModeRemove:
			mode = "removed"
		}
		return mode + " " + t.Target
	case SignalPatch:
		data, _ := json.Marshal(t.Signals)
		return "set signals " + string(data)
	case RedirectPatch:
		return "redirect to " + t.URL
	case ToastPatch:
		return "toast: " + t.Text
	case EventPatch:
		data, _ := json.Marshal(t.Detail)
		return "event " + t.Name + " " + string(data)
	}
	return "patch"
}

// statusRecorder keeps the status of an answer that a middleware writes in
// the place of the action. A tool call needs no body of such an answer.
type statusRecorder struct {
	header http.Header
	code   int
}

func (s *statusRecorder) Header() http.Header { return s.header }

func (s *statusRecorder) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
	}
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.WriteHeader(http.StatusOK)
	return len(b), nil
}

func (s *statusRecorder) status() int {
	if s.code == 0 {
		return http.StatusOK
	}
	return s.code
}

// ToolAnswer is the answer of one tool call for an agent (REQ-AI-08).
type ToolAnswer struct {
	// Text says what the call did. For a typed result it is the JSON of
	// the result.
	Text string
	// Structured is the JSON of the value that ToolResult set, or nil.
	Structured json.RawMessage
	// IsError is true when the call did not run the action to its end.
	IsError bool
}

// toolAnswer makes the answer of a tool call from its outcome (REQ-AI-08).
func toolAnswer(tc *toolCall, rec *statusRecorder) ToolAnswer {
	fail := func(text string) ToolAnswer { return ToolAnswer{Text: text, IsError: true} }
	switch {
	case !tc.done && rec != nil:
		// The middleware of the group, or the cross-origin check,
		// answered: the app refuses this caller.
		return fail("the app refused the call with status " + strconv.Itoa(rec.status()) + " " + http.StatusText(rec.status()))
	case tc.bad:
		return fail("the arguments do not have the form of the input of the tool")
	case len(tc.errs) > 0:
		names := make([]string, 0, len(tc.errs))
		for name := range tc.errs {
			names = append(names, name)
		}
		sort.Strings(names)
		lines := make([]string, len(names))
		for i, name := range names {
			lines[i] = "field " + name + ": " + Translate(tc.errs[name], DefaultMessage(tc.errs[name])) + " (" + tc.errs[name] + ")"
		}
		return fail(strings.Join(lines, "\n"))
	case tc.res == nil:
		return fail("the action gave no answer")
	case tc.res.Err != nil:
		return fail(tc.res.Err.Error())
	case tc.res.hasTool && hasSecretKey(tc.res.tool):
		// The JSON of a map has its keys as text, so this secret has no
		// redacted form: the agent gets no result (SI-04).
		return fail("the result of the tool has a gx.Secret as the key of a map; a secret cannot cross to an agent")
	case tc.res.hasTool:
		data, err := json.Marshal(tc.res.tool)
		if err != nil {
			return fail("the result of the tool is not JSON: " + err.Error())
		}
		return ToolAnswer{Text: string(data), Structured: data}
	case len(tc.res.Patches) == 0:
		return ToolAnswer{Text: "done; the action changed nothing on the page"}
	}
	lines := make([]string, len(tc.res.Patches))
	for i, p := range tc.res.Patches {
		lines[i] = patchSummary(p)
	}
	return ToolAnswer{Text: strings.Join(lines, "\n")}
}

// Tools returns the description of each tool of the app, by name. A tool
// is a mounted action or form with Tool; no other route is a tool (SI-07).
func (a *App) Tools() []ToolInfo {
	out := make([]ToolInfo, 0, len(a.tools))
	for _, t := range a.tools {
		info := t.def.info
		info.Confirm = t.def.confirm
		info.ReadOnly = t.readOnly()
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CallTool runs the tool with the given name for an agent (REQ-AI-07). args
// is the JSON object of the arguments. header holds the headers of the
// request of the agent: the request of the action gets them, so the
// middleware of the group of the action sees the caller as it sees a user
// (SI-07). The caller of CallTool makes the cross-origin check of the request
// of the agent; a handler that the app mounts has it.
//
// Package gxmcp serves the tools over MCP with CallToolFor.
func (a *App) CallTool(ctx context.Context, header http.Header, name string, args json.RawMessage) ToolAnswer {
	return a.callTool(ctx, nil, header, name, args)
}

// CallToolFor runs a tool as CallTool does, for the request of an agent that
// the caller has in hand. The request of the action then has the host, the
// client address and the TLS state of that request too. A middleware that
// reads them sees the caller (SI-07).
func (a *App) CallToolFor(caller *http.Request, name string, args json.RawMessage) ToolAnswer {
	return a.callTool(caller.Context(), caller, caller.Header, name, args)
}

func (a *App) callTool(ctx context.Context, caller *http.Request, header http.Header, name string, args json.RawMessage) ToolAnswer {
	t, ok := a.tools[name]
	if !ok {
		return ToolAnswer{Text: "the app has no tool " + strconv.Quote(name), IsError: true}
	}
	values, err := toolArgs(args)
	if err != nil {
		return ToolAnswer{Text: "the arguments are not a JSON object", IsError: true}
	}
	r, tc := t.toolRequest(ctx, caller, header, values, false)
	rec := &statusRecorder{header: http.Header{}}
	// The request of the agent passed the cross-origin check of the app
	// with these headers (SI-03). The request of the action has a form
	// body, which the token rule for old browsers reads as a form of a
	// page; the check is not made a second time on that form.
	r = r.WithContext(context.WithValue(r.Context(), csrfCheckedKey{}, true))
	t.handler.ServeHTTP(rec, r)
	return toolAnswer(tc, rec)
}

// toolArgs reads the JSON object of the arguments of a tool call. A number
// keeps its text, so an integer above 2^53 reaches the binder with its
// value.
func toolArgs(data []byte) (map[string]any, error) {
	args := map[string]any{}
	if len(bytes.TrimSpace(data)) == 0 {
		return args, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&args); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("more than one JSON value")
	}
	return args, nil
}
