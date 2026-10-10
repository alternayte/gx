package compiler

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	gx "github.com/alternayte/gx"
)

// tmplBuilder collects one template of generated code (DR-11): the static
// strings, the Go expression of each dynamic value, and the facts that the
// passes of package gx read (REQ-AUT-22).
type tmplBuilder struct {
	static []string
	slots  []tmplSlot
	depth  []int
	els    []gx.TemplateEl
	roots  []gx.TemplateRoot
	// cur is the number of elements of the template around the place
	// that the builder writes now.
	cur int
}

type tmplSlot struct {
	expr string
	at   Pos
}

func newTmpl() *tmplBuilder { return &tmplBuilder{static: []string{""}} }

// text adds HTML to the static string that the builder writes now.
func (tb *tmplBuilder) text(s string) { tb.static[len(tb.static)-1] += s }

// slot adds a dynamic value and returns its index.
func (tb *tmplBuilder) slot(expr string, at Pos) int {
	tb.slots = append(tb.slots, tmplSlot{expr: expr, at: at})
	tb.depth = append(tb.depth, tb.cur)
	tb.static = append(tb.static, "")
	return len(tb.slots) - 1
}

// root records a top-level part of the template.
func (tb *tmplBuilder) root(r gx.TemplateRoot) {
	if tb.cur == 0 {
		tb.roots = append(tb.roots, r)
	}
}

// tmplStore holds the template variables of one generated file.
type tmplStore struct {
	prefix string
	decls  []string
	names  map[string]string
}

// add returns the name of the package variable that holds the value of
// init. Two templates with the same text share one variable.
func (s *tmplStore) add(init string) string {
	if name, ok := s.names[init]; ok {
		return name
	}
	name := fmt.Sprintf("_t%s%d", s.prefix, len(s.decls))
	s.decls = append(s.decls, "var "+name+" = "+init)
	if s.names == nil {
		s.names = map[string]string{}
	}
	s.names[init] = name
	return name
}

// rawNames are the elements that never join a static string: the render
// of a script reads the nonce (SI-11), and the contents of a script and a
// style are raw (REQ-AUT-03). They stay gx.El calls.
var rawNames = map[string]bool{"script": true, "style": true}

// bakedOpen returns the open tag of an element whose attributes are the same
// for each render, and whether the element is void. The open tag then joins
// a static string.
//
// An element is not baked when the render or a pass reads its attributes:
//   - an id is the target of a patch and of a form re-render;
//   - a data- attribute can be a marker of the runtime (NFR-04), and a
//     widget gets its own names for some of them (SI-15);
//   - a custom element can need a module (REQ-ISL-09), and the html element
//     says that the page writes its own document.
func (g *gen) bakedOpen(el *Element) (open string, void, ok bool) {
	if el.element != nil || el.Name == "html" || strings.ContainsAny(el.Name, ":-") {
		return "", false, false
	}
	if el == g.rootEl && len(g.file.Signals) > 0 {
		return "", false, false
	}
	var attrs gx.Attrs
	var classes []string
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if a.Kind != AttrString && a.Kind != AttrBool {
			return "", false, false
		}
		switch {
		case isDirective(a.Name), a.Name == "active", a.Name == "id", a.Name == "style",
			strings.HasPrefix(a.Name, "data-"), strings.IndexByte(a.Value, 0) >= 0:
			return "", false, false
		case a.Kind == AttrBool:
			attrs = append(attrs, gx.Bool(a.Name, true))
		case a.Name == "class":
			classes = append(classes, a.Value)
		case isURLAttr(a.Name):
			attrs = append(attrs, gx.Attr{Key: a.Name, Value: a.Value, Kind: gx.AttrURL})
		default:
			attrs = append(attrs, gx.Attr{Key: a.Name, Value: a.Value, Kind: gx.AttrText})
		}
	}
	if len(classes) > 0 {
		// The class attribute is the first attribute, as attrsExpr
		// writes it.
		class := classes[0]
		if len(classes) > 1 {
			class = gx.Classes(classes...)
		}
		attrs = append(gx.Attrs{{Key: "class", Value: class, Kind: gx.AttrText}}, attrs...)
	}
	// The renderer of package gx writes the tag, so the bytes are the
	// bytes of a tree.
	html := gx.String(gx.El(el.Name, attrs))
	open, closed := strings.CutSuffix(html, "</"+el.Name+">")
	return open, !closed, true
}

// isVoid reports whether the renderer writes no closing tag for the name.
func isVoid(name string) bool {
	return !strings.HasSuffix(gx.String(gx.El(name, nil)), "</"+name+">")
}

// tmplNodes adds the nodes of a list with no statement below it.
func (g *gen) tmplNodes(tb *tmplBuilder, ns []Node) {
	for _, n := range ns {
		switch t := n.(type) {
		case *Text:
			tb.root(gx.TemplateRoot{El: -1, Slot: -1})
			tb.text(gx.String(gx.Text(t.Data)))
		case *HTMLComment:
			tb.root(gx.TemplateRoot{El: -1, Slot: -1})
			tb.text("<!--" + t.Data + "-->")
		case *Comment:
			// A {/* ... */} comment never renders (REQ-AUT-16).
		case *Expr:
			i := tb.slot(g.exprValue(t, t.Data), posOf(t))
			tb.root(gx.TemplateRoot{El: -1, Slot: i})
		case *Element:
			pushed := g.keyAttrExpr(t)
			if pushed != "" {
				g.keyStack = append(g.keyStack, pushed)
			}
			if qual, name, ok := componentTag(t.Name); ok {
				g.tmplValue(tb, g.componentCallExpr(t, qual, name, nil), posOf(t))
			} else {
				g.tmplElement(tb, t, "", false)
			}
			if pushed != "" {
				g.keyStack = g.keyStack[:len(g.keyStack)-1]
			}
		}
	}
}

// tmplValue adds a dynamic value that is a node.
func (g *gen) tmplValue(tb *tmplBuilder, expr string, at Pos) {
	i := tb.slot(expr, at)
	tb.root(gx.TemplateRoot{El: -1, Slot: i})
}

// tmplElement adds an HTML element. With built set, children is the
// expression of its content, which statements before the template made.
func (g *gen) tmplElement(tb *tmplBuilder, el *Element, children string, built bool) {
	if el.HasRaw || rawNames[el.Name] {
		g.tmplValue(tb, g.elementExpr(el), posOf(el))
		return
	}
	open, void, baked := g.bakedOpen(el)
	elIndex := -1
	if baked {
		tb.root(gx.TemplateRoot{El: -1, Slot: -1, Name: el.Name})
		tb.text(open)
	} else {
		start := len(tb.static[len(tb.static)-1])
		tb.text("<" + el.Name)
		slot := tb.slot("gx.Open("+strconv.Quote(el.Name)+", "+g.attrsExpr(el)+")", posOf(el))
		elIndex = len(tb.els)
		// A void element ends after the ">" of its open tag.
		tb.els = append(tb.els, gx.TemplateEl{Slot: slot, Start: start, EndStatic: slot + 1, End: 1})
		tb.root(gx.TemplateRoot{El: elIndex, Slot: -1})
		tb.text(">")
		void = isVoid(el.Name)
	}
	if void {
		return
	}
	tb.cur++
	if built {
		if children != "" {
			tb.slot(children, posOf(el))
		}
	} else {
		g.tmplNodes(tb, el.Children)
	}
	tb.cur--
	tb.text("</" + el.Name + ">")
	if elIndex >= 0 {
		tb.els[elIndex].EndStatic = len(tb.static) - 1
		tb.els[elIndex].End = len(tb.static[len(tb.static)-1])
	}
}

// tmplExpr returns the Go expression of the template value, or "" for a
// template with no content.
func (g *gen) tmplExpr(tb *tmplBuilder) string {
	if len(tb.slots) == 0 && tb.static[0] == "" {
		return ""
	}
	if len(tb.slots) == 1 && tb.static[0] == "" && tb.static[1] == "" {
		// One value and no markup: the value is the node.
		return tb.slots[0].expr
	}
	var b strings.Builder
	b.WriteString("gx.NewTemplate(\n[]string{")
	for i, s := range tb.static {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(s))
	}
	b.WriteString("},\n[]int{")
	for i, d := range tb.depth {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(d))
	}
	b.WriteString("},\n")
	if len(tb.els) == 0 {
		b.WriteString("nil,\n")
	} else {
		b.WriteString("[]gx.TemplateEl{")
		for i, el := range tb.els {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "{Slot: %d, Start: %d, EndStatic: %d, End: %d}", el.Slot, el.Start, el.EndStatic, el.End)
		}
		b.WriteString("},\n")
	}
	b.WriteString("[]gx.TemplateRoot{")
	for i, r := range tb.roots {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "{El: %d, Slot: %d", r.El, r.Slot)
		if r.Name != "" {
			fmt.Fprintf(&b, ", Name: %s", strconv.Quote(r.Name))
		}
		b.WriteString("}")
	}
	b.WriteString("},\n)")
	if len(tb.slots) == 0 {
		// The value is the same for each render: one value for all.
		return g.tmpls.add(b.String() + ".With()")
	}
	name := g.tmpls.add(b.String())
	var call strings.Builder
	call.WriteString(name + ".With(\n")
	for _, s := range tb.slots {
		// Each value has its .gx position (REQ-TLS-02).
		if s.at.Line > 0 {
			fmt.Fprintf(&call, "//line %s:%d:%d\n", filepath.Base(g.file.File), s.at.Line, s.at.Col)
		}
		call.WriteString(s.expr + ",\n")
	}
	call.WriteString(")")
	return call.String()
}
