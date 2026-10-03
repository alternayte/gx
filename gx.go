// Package gx is the runtime of the Gx web framework. It holds the public API
// for templates, routes, actions, forms and client signals.
package gx

import (
	"fmt"
	"io"
	"strconv"
	"strings"
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
)

// Attr is one attribute of an element.
type Attr struct {
	Key   string
	Value string
	Kind  AttrKind
}

// Bool returns a boolean attribute that is omitted when present is false
// (REQ-AUT-08).
func Bool(key string, present bool) Attr {
	if present {
		return Attr{Key: key, Value: "true", Kind: AttrBool}
	}
	return Attr{Key: key, Value: "false", Kind: AttrBool}
}

// Render writes n as HTML (REQ-AUT-03, REQ-AUT-12).
func Render(w io.Writer, n Node) error {
	_, err := io.WriteString(w, String(n))
	return err
}

// String returns the HTML of n.
func String(n Node) string {
	var b strings.Builder
	renderNode(&b, n)
	return b.String()
}

func renderNode(b *strings.Builder, n Node) {
	switch t := n.(type) {
	case nil:
		return
	case textNode:
		b.WriteString(escapeText(string(t)))
	case rawNode:
		b.WriteString(string(t))
	case fragNode:
		for _, child := range t {
			renderNode(b, child)
		}
	case *elNode:
		b.WriteByte('<')
		b.WriteString(t.name)
		for _, a := range t.attrs {
			if a.Kind == AttrBool {
				if a.Value == "true" {
					b.WriteByte(' ')
					b.WriteString(a.Key)
				}
				continue
			}
			b.WriteByte(' ')
			b.WriteString(a.Key)
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
		}
		b.WriteByte('>')
		if voidElements[t.name] {
			return
		}
		for _, child := range t.children {
			renderNode(b, child)
		}
		b.WriteString("</")
		b.WriteString(t.name)
		b.WriteByte('>')
	}
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
func escapeURL(s string) string { return attrEscaper.Replace(urlFilter(s)) }

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
