// Package gx is the runtime of the Gx web framework. It holds the public API
// for templates, routes, actions, forms and client signals.
package gx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Node is one node of a render tree. Only this package implements Node.
type Node interface {
	node()
}

// SafeHTML is HTML that needs no escaping.
type SafeHTML string

// Text returns a node that escapes s as HTML text.
func Text(s string) Node { return textNode(s) }

// Raw returns a node that writes s without escaping. Use it only for
// gx.SafeHTML values (SI-01).
func Raw(s SafeHTML) Node { return rawNode(s) }

// Frag returns a node that renders its children in order.
func Frag(children ...Node) Node { return fragNode(children) }

// El returns an element node. An empty attribute value means a boolean
// attribute; AttrBool with the value "false" is omitted.
func El(name string, attrs Attrs, children ...Node) Node {
	return &elNode{name: name, attrs: attrs, children: children}
}

type textNode string
type rawNode string
type fragNode []Node

type elNode struct {
	name     string
	attrs    Attrs
	children []Node
}

func (textNode) node() {}
func (rawNode) node()  {}
func (fragNode) node() {}
func (*elNode) node()  {}

// Builder collects nodes while a generated component runs.
type Builder struct {
	nodes []Node
}

// Add appends nodes to the builder.
func (b *Builder) Add(n ...Node) { b.nodes = append(b.nodes, n...) }

// Node returns the built node.
func (b *Builder) Node() Node { return fragNode(b.nodes) }

// Int returns a text node for an integer. Generated code uses it for an
// expression of an integer type: the render writes the digits with no
// string (NFR-03).
func Int(v int64) Node { return intNode(v) }

type intNode int64

func (intNode) node() {}

// Value returns a text node for a renderable value: bool, integer, float,
// string, fmt.Stringer or error.
func Value(v any) Node { return textNode(TextValue(v)) }

// TextValue returns the text form of a renderable value.
func TextValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case SafeHTML:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case int8:
		return strconv.FormatInt(int64(x), 10)
	case int16:
		return strconv.FormatInt(int64(x), 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint8:
		return strconv.FormatUint(uint64(x), 10)
	case uint16:
		return strconv.FormatUint(uint64(x), 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}

// JSON returns the JSON form of a server value inlined into a client
// expression (SI-05). It panics when the value has no JSON form: the
// compiler must not inline it.
func JSON(v any) string {
	checkSecret(v)
	if devMode.Load() {
		checkSafeInt(v)
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic("gx: cannot inline a value into a client expression: " + err.Error())
	}
	return string(data)
}

// maxSafeInt is the largest integer a JavaScript number holds exactly.
const maxSafeInt = 1<<53 - 1

// checkSafeInt panics in dev when an integer is outside the 53-bit safe
// range: the browser rounds it, so the client expression differs from Go
// (REQ-ACT-13).
func checkSafeInt(v any) {
	var n int64
	switch x := v.(type) {
	case int:
		n = int64(x)
	case int64:
		n = x
	case uint:
		if uint64(x) <= maxSafeInt {
			return
		}
		n = maxSafeInt + 1
	case uint64:
		if x <= maxSafeInt {
			return
		}
		n = maxSafeInt + 1
	default:
		return
	}
	if n > maxSafeInt || n < -maxSafeInt {
		panic("gx: an integer in a client expression is outside the 53-bit safe range of JavaScript: " + TextValue(v))
	}
}

// JoinAttrs concatenates attribute lists in order (REQ-AUT-09).
func JoinAttrs(parts ...Attrs) Attrs {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	if n == 0 {
		return nil
	}
	out := make(Attrs, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Slot is a typed slot: a function that renders one value (REQ-AUT-11).
type Slot[T any] func(T) Node

// When returns name when on is true, and the empty string otherwise.
func When(name string, on bool) string {
	if on {
		return name
	}
	return ""
}

// Classes joins the non-empty parts with single spaces, in source order
// (REQ-AUT-10). Resolve conflicts with gx.Cx.
func Classes(parts ...string) string {
	n := 0
	for _, p := range parts {
		if p != "" {
			n += len(p) + 1
		}
	}
	if n == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(p)
	}
	return b.String()
}

// Attrs is an ordered attribute list.
type Attrs []Attr

// Style is a dynamic style attribute value.
type Style string

// AttrKind selects the escaping rule of an attribute value.
type AttrKind uint8

const (
	// AttrText is a plain attribute value.
	AttrText AttrKind = iota
	// AttrURL is a URL attribute value.
	AttrURL
	// AttrBool is a boolean attribute: present when Value is "true".
	AttrBool
	// AttrStyle is a style attribute value.
	AttrStyle
	// attrBundle is an attribute whose value names an entry of the island
	// bundle. The render writes the URL of the entry file. Only this
	// package sets the kind, so a value from user data cannot name an
	// entry (SI-01).
	attrBundle
)

// Attr is one attribute of an element. Active marks a typed link for the
// aria-current and data-active rules of REQ-RTE-13: "page" (exact match) or
// "section" (path prefix).
type Attr struct {
	Key    string
	Value  string
	Kind   AttrKind
	Active string
}

// Bool returns a boolean attribute that is omitted when present is false
// (REQ-AUT-08).
func Bool(key string, present bool) Attr {
	if present {
		return Attr{Key: key, Value: "true", Kind: AttrBool}
	}
	return Attr{Key: key, Value: "false", Kind: AttrBool}
}

// Render renders n inside any http.Handler (REQ-RTE-16). It sets the content
// type, marks active links for r and writes the merged head.
func Render(w http.ResponseWriter, r *http.Request, n Node) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return RenderRequest(w, r, n)
}

// RenderNode writes n to a writer with no request in scope.
func RenderNode(w io.Writer, n Node) error {
	return writeRequest(w, nil, n)
}

// RenderRequest renders n with the request in scope, so typed links mark the
// active page (REQ-RTE-13). The rendered markers feed the runtime script
// decision of the app (NFR-04).
func RenderRequest(w io.Writer, r *http.Request, n Node) error {
	needs := runtimeNeedsOf(r)
	if needs == nil {
		return writeRequest(w, r, n)
	}
	*needs = scanRuntimeNeeds(n)
	needs.nonce = Nonce(r)
	if needs.ownDocument {
		// The node writes its own html element, so the head stays where
		// the node put it.
		return writeRequest(w, r, n)
	}
	// The app writes the document shell around this fragment, and the
	// gx.Head output moves into its head.
	st := &renderState{request: r, requestURI: activeURI(r), headWritten: true, nonce: needs.nonce, keepSignals: devKeepSignals(r)}
	collectHead(n, st, 1)
	b := getBuffer()
	defer putBuffer(b)
	renderHead(b, st)
	needs.shell = &shellParts{head: b.String(), props: st.head}
	b.Reset()
	renderNode(b, n, st)
	_, err := w.Write(b.Bytes())
	return err
}

// bufferPool holds the buffers of the render. A render writes into one
// buffer and gives its bytes to the writer, so a page costs no buffer of
// its own (NFR-03).
var bufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func getBuffer() *bytes.Buffer {
	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

// putBuffer gives a buffer back. A very large page does not keep its
// buffer in the pool.
func putBuffer(b *bytes.Buffer) {
	if b.Cap() <= 1<<18 {
		bufferPool.Put(b)
	}
}

// writeRequest renders n into a buffer of the pool and writes it.
func writeRequest(w io.Writer, r *http.Request, n Node) error {
	b := getBuffer()
	defer putBuffer(b)
	renderRequest(b, r, n)
	_, err := w.Write(b.Bytes())
	return err
}

// renderRequest renders n with the request in scope.
func renderRequest(b *bytes.Buffer, r *http.Request, n Node) {
	st := &renderState{request: r, nonce: Nonce(r), keepSignals: devKeepSignals(r)}
	if r != nil && r.URL != nil {
		st.requestURI = activeURI(r)
	}
	collectHead(n, st, 1)
	renderNode(b, n, st)
}

// String returns the HTML of n with no request in scope.
func String(n Node) string { return StringRequest(nil, n) }

// StringRequest returns the HTML of n with the request in scope.
func StringRequest(r *http.Request, n Node) string {
	b := getBuffer()
	defer putBuffer(b)
	renderRequest(b, r, n)
	return b.String()
}

func renderNode(b *bytes.Buffer, n Node, st *renderState) {
	switch t := n.(type) {
	case nil:
		return
	case headNode:
		if !st.headWritten {
			st.headWritten = true
			renderHead(b, st)
		}
	case textNode:
		_, _ = textEscaper.WriteString(b, string(t))
	case intNode:
		var digits [20]byte
		b.Write(strconv.AppendInt(digits[:0], int64(t), 10))
	case rawNode:
		b.WriteString(string(t))
	case fragNode:
		for _, child := range t {
			renderNode(b, child, st)
		}
	case *elNode:
		b.WriteByte('<')
		b.WriteString(t.name)
		writeAttrs(b, t.name, t.attrs, st)
		b.WriteByte('>')
		if voidElements[t.name] {
			return
		}
		for _, child := range t.children {
			renderNode(b, child, st)
		}
		b.WriteString("</")
		b.WriteString(t.name)
		b.WriteByte('>')
	case *tmplNode:
		t.render(b, st)
	case *tmplElNode:
		t.render(b, st)
	case *openNode:
		writeAttrs(b, t.name, t.attrs, st)
	}
}

// writeAttrs writes the attributes of one element, and the nonce of a
// script. The element of a tree and the open tag of a template value both
// use it, so the two give the same bytes (REQ-AUT-21).
func writeAttrs(b *bytes.Buffer, name string, attrs Attrs, st *renderState) {
	for _, a := range attrs {
		if a.Kind == AttrBool {
			if a.Value == "true" {
				b.WriteByte(' ')
				b.WriteString(a.Key)
			}
			continue
		}
		if a.Kind == attrBundle {
			// The file of an island or of a web component module:
			// the installed bundle names it at render time.
			a.Value, a.Kind = bundleURL(a.Value), AttrURL
			if a.Value == "" {
				continue
			}
		}
		if a.Key == clientKey {
			// A client expression: text for the adapter, or
			// data for a widget (SI-15).
			key, value, ok := st.resolveClient(a.Value)
			if !ok {
				continue
			}
			a.Key, a.Value, a.Kind = key, value, AttrText
		}
		if st.forWidget() {
			// A widget has its own names for the attributes of
			// signals, and an invocation is data.
			switch a.Key {
			case "data-signals", "data-bind":
				a.Key = widgetAttrKey(a.Key)
			case onKey:
				out, ok := widgetOn(a.Value)
				if !ok {
					continue
				}
				a = out
			case invokeKey:
				out, ok := widgetInvoke(a.Value)
				if !ok {
					continue
				}
				a = out
			}
		}
		if a.Key == onKey {
			// An action invocation on an event: the adapter
			// writes its own attributes (REQ-PLG-04).
			for _, out := range st.resolveOn(a.Value) {
				b.WriteByte(' ')
				b.WriteString(out.Key)
				b.WriteString(`="`)
				b.WriteString(escapeAttr(out.Value))
				b.WriteByte('"')
			}
			continue
		}
		if a.Kind == AttrText && strings.IndexByte(a.Value, 0) >= 0 {
			// An action invocation: the adapter writes it
			// (REQ-PLG-04).
			a.Key, a.Value = st.resolveInvoke(a.Key, a.Value)
			if a.Key == "" {
				continue
			}
		}
		b.WriteByte(' ')
		b.WriteString(a.Key)
		if st != nil && st.keepSignals && a.Key == "data-signals" {
			// A dev reload keeps the state of the page: a signal
			// that the browser holds keeps its value.
			b.WriteString("__ifmissing")
		}
		b.WriteString(`="`)
		switch a.Kind {
		case AttrURL:
			b.WriteString(escapeURL(a.Value))
		case AttrStyle:
			b.WriteString(escapeStyle(a.Value))
		default:
			b.WriteString(escapeAttr(a.Value))
		}
		b.WriteByte('"')
		if a.Active != "" {
			b.WriteString(` data-gx-active="`)
			b.WriteString(escapeAttr(a.Active))
			b.WriteByte('"')
		}
		if a.Active != "" && st != nil {
			switch {
			case a.Active == "section" && sectionMatch(st.requestURI, a.Value):
				b.WriteString(" data-active")
			case a.Active == "page" && st.requestURI == a.Value:
				b.WriteString(` aria-current="page"`)
			}
		}
	}
	if name == "script" && st != nil && st.nonce != "" && !hasAttr(attrs, "nonce") {
		// Every script carries the nonce of the policy (SI-11).
		b.WriteString(` nonce="`)
		b.WriteString(escapeAttr(st.nonce))
		b.WriteByte('"')
	}
}

// hasAttr reports whether attrs holds key.
func hasAttr(attrs Attrs, key string) bool {
	for _, a := range attrs {
		if a.Key == key {
			return true
		}
	}
	return false
}

// sectionMatch reports whether the current URI is the link path or below it.
func sectionMatch(currentURI, href string) bool {
	current := currentURI
	if i := strings.IndexByte(current, '?'); i >= 0 {
		current = current[:i]
	}
	base := href
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	if base == "" {
		return false
	}
	return current == base || strings.HasPrefix(current, strings.TrimSuffix(base, "/")+"/")
}

var textEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func escapeText(s string) string { return textEscaper.Replace(s) }

var attrEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&#34;",
	"'", "&#39;",
)

func escapeAttr(s string) string { return attrEscaper.Replace(s) }

// escapeStyle escapes a dynamic style attribute value (REQ-AUT-12).
func escapeStyle(s string) string { return attrEscaper.Replace(s) }

// escapeURL filters a URL and then escapes it for an attribute (REQ-AUT-12).
// A URL with a script scheme renders as "#" (SI-02).
func escapeURL(s string) string {
	if scriptScheme(s) {
		return "#"
	}
	return attrEscaper.Replace(urlFilter(s))
}

// activeURI returns the address that a typed link has for the current
// request. A mounted app sees the path without its prefix, and a typed
// link holds the prefix (REQ-RTE-13, REQ-RTE-18).
func activeURI(r *http.Request) string {
	return BasePath() + r.URL.RequestURI()
}

// scriptScheme reports whether a browser reads the URL as javascript: or
// vbscript:. A browser drops leading spaces and control characters, and
// every tab and newline, before it reads the scheme.
func scriptScheme(s string) bool {
	const longest = len("javascript:")
	var scheme [longest]byte
	n := 0
	lead := true
	for i := 0; i < len(s) && n < longest; i++ {
		c := s[i]
		if c == '\t' || c == '\n' || c == '\r' || (lead && c <= ' ') {
			continue
		}
		lead = false
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		scheme[n] = c
		n++
		if c == ':' {
			break
		}
	}
	got := string(scheme[:n])
	return got == "javascript:" || got == "vbscript:"
}

// urlFilter percent-encodes bytes that are not safe in a URL.
func urlFilter(s string) string {
	// Straight from html/template: keep unreserved and gen-delims, plus
	// the sub-delims that carry URL syntax.
	const safe = "!#$&*+,-./:;=?@[]_~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '%':
			b.WriteByte(c)
		case strings.IndexByte(safe, c) >= 0:
			b.WriteByte(c)
		default:
			const hex = "0123456789ABCDEF"
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

// voidElements are the HTML elements with no closing tag.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}
