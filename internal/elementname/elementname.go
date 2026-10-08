// Package elementname holds the name rule of a custom element. Package gx
// applies it when a widget gets its tag, and the compiler applies it in
// gx check (REQ-ISL-10).
package elementname

import "strings"

// reserved are the names with a hyphen that HTML keeps for itself.
var reserved = map[string]bool{
	"annotation-xml": true, "color-profile": true, "font-face": true, "font-face-src": true,
	"font-face-uri": true, "font-face-format": true, "font-face-name": true, "missing-glyph": true,
}

// Problem returns why a name is not a valid custom element name, or "".
func Problem(name string) string {
	if name == "" {
		return "the name is empty"
	}
	if name[0] < 'a' || name[0] > 'z' {
		return "the name must start with a lowercase letter"
	}
	if !strings.Contains(name, "-") {
		return "the name must have a hyphen"
	}
	if reserved[name] {
		return "HTML keeps this name for itself"
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_', c == 0xB7:
		case c >= 0xC0 && c != 0xD7 && c != 0xF7:
		default:
			return "the name has a character that a custom element name cannot have"
		}
	}
	return ""
}
