package gx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// A client attribute has two forms (SI-15). For a page of the app it is the
// attribute of the adapter, with the expression as text that the adapter
// runs. For a widget it is data: a small JSON tree that the widget script
// evaluates. A host page gets no code of the app, so it needs no
// 'unsafe-eval' in its policy.
//
// The tree is a list. Its first item names the node:
//
//	["v", value]              a value of the server, or a literal
//	["s", ["cart", "qty"]]    a signal, by its path
//	[op, left, right]         an operator of Go: + - * / % == != < <= > >= && ||
//	["idiv", left, right]     the division of two integers
//	["!", x], ["neg", x]      the unary operators
//	["len", x] and the rest   a helper of package gxc
//	["do", statement...]      the statements of an on: handler
//	["=", path, x]            a signal assignment; ["++", path], ["--", path]
//	["call", method, url, scope]  an action invocation

const (
	// clientKey marks an attribute that Client made. The render writes
	// one of its two forms.
	clientKey = "\x00gx-client"
	// clientSep separates the fields of a client attribute. No field
	// holds it: the adapter text and the tree escape control characters.
	clientSep = "\x1d"
	// widgetHeader is the request header of a widget. Its value is the
	// tag of the widget.
	widgetHeader = "Gx-Widget"
)

// Client returns a client attribute (REQ-ACT-07, SI-15). key and value are
// the attribute of the adapter. tree is the same expression as data, for a
// widget. Generated code calls it.
func Client(key, value, tree string) Attr {
	return Attr{Key: clientKey, Value: key + clientSep + value + clientSep + tree}
}

// ExprValue returns the tree node of a server value (SI-15). The value is
// data in the tree, as it is in gx.JSON.
func ExprValue(v any) string {
	return `["v",` + JSON(v) + `]`
}

// ExprSignal returns the tree node of a signal of a component instance.
func ExprSignal(base string, key Key, name string) string {
	return `["s",` + exprPath(base, key, name) + `]`
}

// ExprPath returns the path of a signal of a component instance, for the
// target of a signal statement.
func ExprPath(base string, key Key, name string) string {
	return exprPath(base, key, name)
}

func exprPath(base string, key Key, name string) string {
	data, err := json.Marshal(append(scopePath(base, key), name))
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ExprRef returns the tree node of a signal that a gx.SignalRef prop names.
// path is the text of the reference.
func ExprRef(path string) string {
	return `["s",` + ExprRefPath(path) + `]`
}

// ExprRefPath returns the path of a signal that a gx.SignalRef prop names.
// The text of a reference is a bracket path, as gx.SignalRefPath writes it.
func ExprRefPath(path string) string {
	parts := []string{}
	for rest := path; strings.HasPrefix(rest, "["); {
		quoted, err := strconv.QuotedPrefix(rest[1:])
		if err != nil {
			break
		}
		part, err := strconv.Unquote(quoted)
		if err != nil {
			break
		}
		parts = append(parts, part)
		rest = strings.TrimPrefix(rest[1+len(quoted):], "]")
	}
	data, err := json.Marshal(parts)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ExprOp returns a tree node with a name and its parts. Each part is a tree
// node or a path.
func ExprOp(op string, parts ...string) string {
	var b strings.Builder
	b.WriteString(`[`)
	b.WriteString(strconv.Quote(op))
	for _, part := range parts {
		b.WriteByte(',')
		b.WriteString(part)
	}
	b.WriteByte(']')
	return b.String()
}

// ExprCall returns the tree node of an action invocation (REQ-ACT-02).
func ExprCall(method, url, scope string) string {
	data, err := json.Marshal([]string{"call", method, externalURL(method, url), scope})
	if err != nil {
		return `["do"]`
	}
	return string(data)
}

// isWidgetRequest reports whether a widget sent the request.
func isWidgetRequest(r *http.Request) bool {
	return r != nil && r.Header.Get(widgetHeader) != ""
}

// forWidget reports whether the render is for a widget.
func (st *renderState) forWidget() bool {
	if st == nil {
		return false
	}
	if !st.widgetSet {
		st.widget, st.widgetSet = isWidgetRequest(st.request), true
	}
	return st.widget
}

// widgetAttrKey returns the name of a client attribute in a widget: the
// name of the adapter attribute, in the namespace of the widget script.
func widgetAttrKey(key string) string {
	return "data-gx-" + strings.TrimPrefix(key, "data-")
}

// resolveClient returns the form of a client attribute for this render.
func (st *renderState) resolveClient(value string) (key, out string, ok bool) {
	fields := strings.SplitN(value, clientSep, 3)
	if len(fields) != 3 {
		return "", "", false
	}
	if st.forWidget() {
		return widgetAttrKey(fields[0]), fields[2], true
	}
	return fields[0], fields[1], true
}

// widgetOn returns the attribute of an On placeholder in a widget: the
// event in the name, and the invocation as data.
func widgetOn(value string) (Attr, bool) {
	fields := strings.Split(value, invokeSep)
	if len(fields) != 4 {
		return Attr{}, false
	}
	event, every, mods := ParseOn(fields[3])
	key := "data-gx-on:" + event
	switch event {
	case "load":
		key = "data-gx-init"
	case "visible":
		key = "data-gx-on-intersect"
	case "interval":
		key = "data-gx-on-interval"
		if every != "" {
			key += "__duration." + every
		}
	}
	for _, mod := range mods {
		key += "__" + mod.Name
		if mod.Value != "" {
			key += "." + mod.Value
		}
	}
	return Attr{Key: key, Value: ExprOp("do", ExprCall(fields[0], fields[1], fields[2]))}, true
}

// widgetInvoke returns the attribute of a whole Invoke placeholder in a
// widget: a click handler with the invocation as data.
func widgetInvoke(value string) (Attr, bool) {
	body, ok := strings.CutPrefix(value, invokeOpen)
	if !ok {
		return Attr{}, false
	}
	body, ok = strings.CutSuffix(body, invokeClose)
	fields := strings.Split(body, invokeSep)
	if !ok || len(fields) != 3 {
		return Attr{}, false
	}
	return Attr{Key: "data-gx-on:click", Value: ExprOp("do", ExprCall(fields[0], fields[1], fields[2]))}, true
}

// widgetOp is one step of the answer to a widget (D-264).
type widgetOp struct {
	// Op is "patch", "signals", "redirect" or "toast".
	Op         string `json:"op"`
	Mode       string `json:"mode,omitempty"`
	Target     string `json:"target,omitempty"`
	HTML       string `json:"html,omitempty"`
	Transition bool   `json:"transition,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Values     any    `json:"values,omitempty"`
	URL        string `json:"url,omitempty"`
}

// widgetOps is the answer of an action to a widget. It does not go through
// the adapter of the app: a widget has one wire form under each adapter.
type widgetOps struct {
	Build string       `json:"build"`
	Ops   []widgetOp   `json:"ops"`
	Error *widgetError `json:"error,omitempty"`
}

var widgetModes = map[PatchMode]string{
	ModeMorph: "morph", ModeInner: "inner", ModeAppend: "append",
	ModePrepend: "prepend", ModeReplace: "replace", ModeRemove: "remove",
}

// respondWidget writes the patches of an action as the answer to a widget.
func respondWidget(w http.ResponseWriter, r *http.Request, res *Response) {
	out := widgetOps{Build: buildID(), Ops: []widgetOp{}}
	for _, p := range res.Patches {
		switch t := p.(type) {
		case ElementPatch:
			op := widgetOp{Op: "patch", Mode: widgetModes[t.Mode], Target: t.Target, Transition: t.Transition}
			if t.Mode != ModeRemove {
				op.HTML = StringRequest(r, t.Node)
			}
			out.Ops = append(out.Ops, op)
		case SignalPatch:
			out.Ops = append(out.Ops, widgetOp{Op: "signals", Scope: t.Scope, Values: t.Signals})
		case RedirectPatch:
			out.Ops = append(out.Ops, widgetOp{Op: "redirect", URL: t.URL})
		case ToastPatch:
			out.Ops = append(out.Ops, widgetOp{Op: "toast", HTML: StringRequest(r, RenderToast(r, t))})
		}
	}
	if res.Err != nil {
		// The host gets a status and a key. The text of the error is
		// in the toast, inside the widget.
		out.Error = &widgetError{Status: http.StatusInternalServerError, Key: "gx.error"}
	}
	body, err := json.Marshal(out)
	if err != nil {
		writeWidgetError(w, http.StatusInternalServerError, "gx.error", "")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

var errTooManySignals = errors.New("gx: the signals of the request are too large")

// widgetSignalsLimit is the most bytes of signals that a widget sends.
const widgetSignalsLimit = 1 << 20

// widgetSignals reads the signals of a widget request: the member "signals"
// of a JSON body, or the query value gx-signals of a request with no body.
// The client controls each value, so the rules of the input check them
// (DR-07).
func widgetSignals(r *http.Request) (map[string]any, error) {
	var raw []byte
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		raw = []byte(r.URL.Query().Get("gx-signals"))
	default:
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") || r.Body == nil {
			return map[string]any{}, nil
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, widgetSignalsLimit+1))
		if err != nil {
			return nil, err
		}
		if len(body) > widgetSignalsLimit {
			return nil, &BindError{Err: errTooManySignals}
		}
		var envelope struct {
			Signals json.RawMessage `json:"signals"`
		}
		if len(body) > 0 {
			if err := json.Unmarshal(body, &envelope); err != nil {
				return nil, &BindError{Err: err}
			}
		}
		raw = envelope.Signals
	}
	m := map[string]any{}
	if len(raw) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, &BindError{Err: err}
	}
	return m, nil
}
