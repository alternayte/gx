package gx

import "strings"

// TransitionName is the rendered value of a typed transition (REQ-STY-07).
type TransitionName struct {
	Name string
	Key  string
}

// Transition returns a typed transition. Call it with a key to pair one
// element across two pages:
//
//	var ProductImage = gx.Transition[int64]("product-image")
//	<img transition={ProductImage(p.Product.ID)} src=... />
func Transition[K any](name string) func(K) TransitionName {
	return func(k K) TransitionName {
		return TransitionName{Name: name, Key: TextValue(k)}
	}
}

// TransitionStyle renders the sanitized view-transition-name and
// view-transition-class of a transition (REQ-STY-07).
func TransitionStyle(t TransitionName) Style {
	base := sanitizeIdent(t.Name)
	name := base
	if key := sanitizeIdent(t.Key); key != "" {
		name = base + "-" + key
	}
	return Style("view-transition-name: " + name + "; view-transition-class: " + base)
}

// StyleJoin joins style parts with a semicolon (REQ-AUT-12).
func StyleJoin(parts ...Style) Style {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(string(p)); s != "" {
			out = append(out, strings.TrimSuffix(s, ";"))
		}
	}
	return Style(strings.Join(out, "; "))
}

// sanitizeIdent keeps only the characters of a CSS identifier.
func sanitizeIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return out
}
