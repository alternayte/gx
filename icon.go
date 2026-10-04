package gx

import "strings"

// IconProps are the props of a generated icon component (REQ-STY-06).
type IconProps struct {
	// Label sets role="img" and aria-label. Without it the icon is
	// decorative: aria-hidden="true".
	Label string
	// Class is the class attribute of the svg.
	Class string
	// Attrs adds extra attributes.
	Attrs Attrs
}

// Icon renders one icon as an inline svg with currentColor. body is the
// inner markup of a pinned icon pack, not user input.
func Icon(body string, p IconProps) Node {
	attrs := Attrs{
		{Key: "xmlns", Value: "http://www.w3.org/2000/svg", Kind: AttrText},
		{Key: "viewBox", Value: "0 0 24 24", Kind: AttrText},
		{Key: "fill", Value: "none", Kind: AttrText},
		{Key: "stroke", Value: "currentColor", Kind: AttrText},
		{Key: "stroke-width", Value: "2", Kind: AttrText},
		{Key: "stroke-linecap", Value: "round", Kind: AttrText},
		{Key: "stroke-linejoin", Value: "round", Kind: AttrText},
	}
	if p.Label != "" {
		attrs = append(attrs,
			Attr{Key: "role", Value: "img", Kind: AttrText},
			Attr{Key: "aria-label", Value: p.Label, Kind: AttrText},
		)
	} else {
		attrs = append(attrs, Attr{Key: "aria-hidden", Value: "true", Kind: AttrText})
	}
	if p.Class != "" {
		attrs = append(attrs, Attr{Key: "class", Value: p.Class, Kind: AttrText})
	}
	attrs = append(attrs, p.Attrs...)
	return El("svg", attrs, Raw(SafeHTML(body))) //gx:trusted the body comes from a pinned icon pack (REQ-STY-06)
}

// IsIconBody reports whether a string looks like the inner markup of an
// icon. The icon generator refuses anything else, so a generated icon
// cannot carry markup that breaks out of the svg.
func IsIconBody(body string) bool {
	if body == "" {
		return false
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "<script") || strings.Contains(lower, "javascript:") {
		return false
	}
	for _, tag := range []string{"path", "circle", "line", "rect", "polyline", "polygon", "ellipse", "g", "defs"} {
		if strings.Contains(lower, "<"+tag) {
			return true
		}
	}
	return false
}
