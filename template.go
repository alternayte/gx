package gx

import (
	"bytes"
	"errors"
	"fmt"
)

// Template is the constant part of a template value (DR-11): the static
// strings of one piece of generated markup, and the facts that a pass over a
// page needs (REQ-AUT-22). Generated code makes each Template one time, in a
// package variable. Code that a person writes uses gx.El.
type Template struct {
	// static holds the HTML between the dynamic values: one string
	// before each value and one after the last.
	static []string
	// depth holds, for each dynamic value, the number of elements of
	// the template around it. The deepest title of a page wins
	// (REQ-RTE-11).
	depth []int
	// els holds each element whose open tag is a dynamic value, in the
	// order of the document. Only such an element can have an id.
	els []TemplateEl
	// roots holds the top-level parts of the template, for a patch
	// (REQ-ACT-04).
	roots []TemplateRoot
}

// TemplateEl is one element of a template whose attributes are a dynamic
// value. Slot is the index of that value. The element starts at the byte
// Start of the static string before the value, and ends at the byte End of
// the static string EndStatic.
type TemplateEl struct {
	Slot, Start, EndStatic, End int
}

// TemplateRoot is one top-level part of a template: an element of els (El,
// or -1), an element with no dynamic attribute (Name), a dynamic value (Slot,
// or -1) or text.
type TemplateRoot struct {
	El   int
	Slot int
	Name string
}

// NewTemplate returns the constant part of a template value. Generated code
// calls it; the compiler gives the facts.
func NewTemplate(static []string, depth []int, els []TemplateEl, roots []TemplateRoot) *Template {
	if len(static) != len(depth)+1 {
		panic("gx: a template has one static string more than it has dynamic values")
	}
	return &Template{static: static, depth: depth, els: els, roots: roots}
}

// With returns the template value of one render: the template and its
// dynamic values, in the order of the document.
func (t *Template) With(dyn ...Node) Node {
	if len(dyn) != len(t.depth) {
		panic("gx: a template value has the wrong number of dynamic values")
	}
	n := &tmplNode{t: t, n: len(dyn)}
	if len(dyn) <= len(n.few) {
		copy(n.few[:], dyn)
	} else {
		n.many = append([]Node(nil), dyn...)
	}
	return n
}

// Open returns the attributes of one element as a dynamic value. The static
// string before it ends with the name of the element.
func Open(name string, attrs Attrs) Node { return &openNode{name: name, attrs: attrs} }

// tmplNode is a template value. A value with few dynamic values holds them
// in itself, so it is one allocation.
type tmplNode struct {
	t    *Template
	n    int
	few  [3]Node
	many []Node
}

// openNode is the open tag of an element with a dynamic attribute.
type openNode struct {
	name  string
	attrs Attrs
}

// tmplElNode is one element of a template value: the target of a patch
// (REQ-ACT-04) or the form of a re-render (REQ-FRM-05).
type tmplElNode struct {
	n  *tmplNode
	el int
}

func (*tmplNode) node()   {}
func (*openNode) node()   {}
func (*tmplElNode) node() {}

// dyn returns the dynamic values of the template value.
func (n *tmplNode) dyn() []Node {
	if n.many != nil {
		return n.many
	}
	return n.few[:n.n]
}

func (n *tmplNode) render(b *bytes.Buffer, st *renderState) {
	dyn := n.dyn()
	for i, s := range n.t.static {
		b.WriteString(s)
		if i < len(dyn) {
			renderNode(b, dyn[i], st)
		}
	}
}

// open returns the open tag of the element.
func (e *tmplElNode) open() *openNode {
	o, _ := e.n.dyn()[e.n.t.els[e.el].Slot].(*openNode)
	return o
}

// inner returns the dynamic values of the element, its open tag first.
func (e *tmplElNode) inner() []Node {
	el := e.n.t.els[e.el]
	return e.n.dyn()[el.Slot:el.EndStatic]
}

// depthOf returns the number of elements around the dynamic value i of the
// element, the element included.
func (e *tmplElNode) depthOf(i int) int {
	el := e.n.t.els[e.el]
	return e.n.t.depth[el.Slot+i] - e.n.t.depth[el.Slot]
}

func (e *tmplElNode) render(b *bytes.Buffer, st *renderState) {
	t, el, dyn := e.n.t, e.n.t.els[e.el], e.n.dyn()
	b.WriteString(t.static[el.Slot][el.Start:])
	for j := el.Slot; j < el.EndStatic; j++ {
		renderNode(b, dyn[j], st)
		if j+1 < el.EndStatic {
			b.WriteString(t.static[j+1])
		} else {
			b.WriteString(t.static[el.EndStatic][:el.End])
		}
	}
}

// patchRoot is one top-level element of a patch node.
type patchRoot struct {
	name string
	id   string
	node Node
}

// patchRoots returns the top-level elements of a patch node.
func patchRoots(n Node) ([]patchRoot, error) {
	switch t := n.(type) {
	case *elNode:
		return []patchRoot{{name: t.name, id: attrValue(t.attrs, "id"), node: t}}, nil
	case *tmplElNode:
		o := t.open()
		return []patchRoot{{name: o.name, id: attrValue(o.attrs, "id"), node: t}}, nil
	case fragNode:
		var out []patchRoot
		for _, child := range t {
			roots, err := patchRoots(child)
			if err != nil {
				return nil, err
			}
			out = append(out, roots...)
		}
		if len(out) == 0 {
			return nil, errors.New("gx: patch node is empty")
		}
		return out, nil
	case *tmplNode:
		var out []patchRoot
		dyn := t.dyn()
		for _, r := range t.t.roots {
			switch {
			case r.El >= 0:
				roots, _ := patchRoots(&tmplElNode{n: t, el: r.El})
				out = append(out, roots...)
			case r.Slot >= 0:
				roots, err := patchRoots(dyn[r.Slot])
				if err != nil {
					return nil, err
				}
				out = append(out, roots...)
			case r.Name != "":
				// An element with no dynamic attribute has no id.
				out = append(out, patchRoot{name: r.Name})
			default:
				return nil, fmt.Errorf("gx: patch node %T is not an element", textNode(""))
			}
		}
		if len(out) == 0 {
			return nil, errors.New("gx: patch node is empty")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("gx: patch node %T is not an element", n)
	}
}

// findElementByID returns the first element with the given id, or nil.
func findElementByID(n Node, id string) Node {
	switch t := n.(type) {
	case *elNode:
		if attrValue(t.attrs, "id") == id {
			return t
		}
		for _, child := range t.children {
			if el := findElementByID(child, id); el != nil {
				return el
			}
		}
	case fragNode:
		for _, child := range t {
			if el := findElementByID(child, id); el != nil {
				return el
			}
		}
	case *tmplNode:
		dyn := t.dyn()
		for i, el := range t.t.els {
			if o, ok := dyn[el.Slot].(*openNode); ok && attrValue(o.attrs, "id") == id {
				return &tmplElNode{n: t, el: i}
			}
		}
		for _, d := range dyn {
			if el := findElementByID(d, id); el != nil {
				return el
			}
		}
	case *tmplElNode:
		if attrValue(t.open().attrs, "id") == id {
			return t
		}
		for _, d := range t.inner() {
			if el := findElementByID(d, id); el != nil {
				return el
			}
		}
	}
	return nil
}
