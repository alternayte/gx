package gx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// mountedTool is one tool of the app with the mounted route of its action.
type mountedTool struct {
	def    *toolDef
	method string
	// path is the mounted path of the route, with its pattern variables.
	path string
}

// addTool records the tool of a mounted action or form.
func (a *App) addTool(def *toolDef, pattern string) {
	name := def.info.Name
	if _, ok := a.tools[name]; ok {
		panic("gx: two tools have the name " + name)
	}
	if a.tools == nil {
		a.tools = map[string]mountedTool{}
	}
	method, path, _ := strings.Cut(pattern, " ")
	a.tools[name] = mountedTool{def: def, method: method, path: path}
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
func (t mountedTool) toolRequest(ctx context.Context, header http.Header, args map[string]any) (*http.Request, *toolCall) {
	tc := &toolCall{signals: map[string]any{}}
	path := t.path
	query, form := url.Values{}, url.Values{}
	for _, f := range t.def.info.Fields {
		v, ok := args[f.Name]
		if !ok {
			continue
		}
		switch f.In {
		case "path":
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
	if t.method == http.MethodGet || t.method == http.MethodHead {
		// A request with no body has its form values in the query.
		for name, values := range form {
			query[name] = values
		}
	} else {
		body = strings.NewReader(form.Encode())
	}
	target := BasePath() + path
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
	for name, values := range header {
		switch name {
		case "Content-Type", "Content-Length", "Accept", "Mcp-Session-Id", "Mcp-Protocol-Version", "Last-Event-Id":
			// These describe the MCP message, not the call.
		default:
			r.Header[name] = values
		}
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if host := header.Get("Host"); host != "" {
		r.Host = host
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
	case !tc.done:
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
		out = append(out, t.def.info)
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
// Package gxmcp serves the tools over MCP with this method.
func (a *App) CallTool(ctx context.Context, header http.Header, name string, args json.RawMessage) ToolAnswer {
	t, ok := a.tools[name]
	if !ok {
		return ToolAnswer{Text: "the app has no tool " + strconv.Quote(name), IsError: true}
	}
	values := map[string]any{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &values); err != nil {
			return ToolAnswer{Text: "the arguments are not a JSON object", IsError: true}
		}
	}
	r, tc := t.toolRequest(ctx, header, values)
	rec := &statusRecorder{header: http.Header{}}
	// The request of the agent passed the cross-origin check of the app
	// with these headers (SI-03). The request of the action has a form
	// body, which the token rule for old browsers reads as a form of a
	// page; the check is not made a second time on that form.
	r = r.WithContext(context.WithValue(r.Context(), csrfCheckedKey{}, true))
	a.mux.ServeHTTP(rec, r)
	return toolAnswer(tc, rec)
}
