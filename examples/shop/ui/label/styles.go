package label

import "github.com/alternayte/gx"

// attrs returns the for attribute when the label names a control, then the
// caller's attributes.
func (p LabelProps) attrs() gx.Attrs {
	if p.For == "" {
		return p.Attrs
	}
	return append(gx.Attrs{{Key: "for", Value: p.For}}, p.Attrs...)
}
