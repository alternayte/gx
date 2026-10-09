package compiler

import (
	"strings"

	gx "github.com/alternayte/gx"
)

// staticNames are the elements that stay a gx.El call with no expression
// below them: the render or a pass over the tree reads the element itself.
// A script gets the nonce of the response (SI-11), and the html element
// says that the page writes its own document (NFR-04).
var staticNames = map[string]bool{
	"html": true, "head": true, "body": true, "script": true, "style": true,
}

// staticNode returns the node of an element whose HTML is the same for each
// render: no expression, no directive and no component at or below it. The
// compiler writes such an element as one pre-escaped string (SDD 3.2).
//
// An element is not static when the render or a pass over the tree reads it:
//   - an id is the target of a patch and of a form re-render;
//   - a data- attribute can be a marker of the runtime (NFR-04), and a
//     widget gets its own names for some of them (SI-15);
//   - a custom element can need a module (REQ-ISL-09).
func (g *gen) staticNode(el *Element) (gx.Node, bool) {
	if el == g.rootEl || el.HasRaw || el.element != nil || staticNames[el.Name] {
		return nil, false
	}
	if _, _, ok := componentTag(el.Name); ok || strings.ContainsAny(el.Name, ":-") {
		return nil, false
	}
	var attrs gx.Attrs
	var classes []string
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if a.Kind != AttrString && a.Kind != AttrBool {
			return nil, false
		}
		switch {
		case isDirective(a.Name), a.Name == "active", a.Name == "id", a.Name == "style",
			strings.HasPrefix(a.Name, "data-"), strings.IndexByte(a.Value, 0) >= 0:
			return nil, false
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
	var children []gx.Node
	for _, n := range el.Children {
		switch t := n.(type) {
		case *Text:
			children = append(children, gx.Text(t.Data))
		case *HTMLComment:
			children = append(children, gx.Raw(gx.SafeHTML("<!--"+t.Data+"-->")))
		case *Comment:
		case *Element:
			child, ok := g.staticNode(t)
			if !ok {
				return nil, false
			}
			children = append(children, child)
		default:
			return nil, false
		}
	}
	return gx.El(el.Name, attrs, children...), true
}
