package gx

import "strings"

// Cx merges class strings with tailwind-merge semantics for Tailwind v4
// (REQ-STY-04). Later classes win inside one conflict group, modifiers scope
// the conflict, and the surviving classes keep their input order.
func Cx(parts ...string) string {
	records := make([]cxClass, 0, 16)
	for _, part := range parts {
		for i := 0; i < len(part); {
			for i < len(part) && (part[i] == ' ' || part[i] == '\t' || part[i] == '\n' || part[i] == '\r') {
				i++
			}
			if i >= len(part) {
				break
			}
			start := i
			for i < len(part) && part[i] != ' ' && part[i] != '\t' && part[i] != '\n' && part[i] != '\r' {
				i++
			}
			records = append(records, parseClass(part[start:i]))
		}
	}
	kept := make([]cxClass, 0, len(records))
	for _, rec := range records {
		if rec.group == "" {
			kept = append(kept, rec)
			continue
		}
		for i := 0; i < len(kept); {
			other := kept[i]
			if other.group != "" && other.modifiers == rec.modifiers && conflicts(rec.group, other.group) {
				kept = append(kept[:i], kept[i+1:]...)
				continue
			}
			i++
		}
		kept = append(kept, rec)
	}
	if len(kept) == 0 {
		return ""
	}
	var b strings.Builder
	size := 0
	for _, rec := range kept {
		size += len(rec.raw) + 1
	}
	b.Grow(size)
	for i, rec := range kept {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(rec.raw)
	}
	return b.String()
}

// cxClass is one parsed class.
type cxClass struct {
	raw       string
	modifiers string // the variant prefix, "" for none
	group     string // the conflict group, "" when unknown
}

// parseClass splits a class into modifiers and base and finds its group.
func parseClass(class string) cxClass {
	rec := cxClass{raw: class}
	base := class
	depth := 0
	last := -1
	for i := 0; i < len(base); i++ {
		switch base[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				last = i
			}
		}
	}
	if last >= 0 {
		rec.modifiers = base[:last]
		base = base[last+1:]
	}
	// An important class only replaces an important class. Tailwind v4
	// writes the mark at the end; the v3 prefix still parses.
	if strings.HasPrefix(base, "!") || strings.HasSuffix(base, "!") {
		base = strings.TrimSuffix(strings.TrimPrefix(base, "!"), "!")
		rec.modifiers += "!"
	}
	base = strings.TrimPrefix(base, "-")
	rec.group = classGroup(base)
	return rec
}

// conflicts reports whether a class of group next replaces one of group prev.
func conflicts(next, prev string) bool {
	if next == prev {
		return true
	}
	for _, g := range conflictGroups[next] {
		if g == prev {
			return true
		}
	}
	return false
}

// classGroup returns the conflict group of a class, or "" when the class
// does not participate in merging.
func classGroup(class string) string {
	if g, ok := staticGroups[class]; ok {
		return g
	}
	i := indexByte(class, '-')
	if i < 0 {
		return ""
	}
	head, rest := class[:i], class[i+1:]
	switch head {
	case "p", "px", "py", "ps", "pe", "pt", "pr", "pb", "pl":
		return head
	case "m", "mx", "my", "ms", "me", "mt", "mr", "mb", "ml":
		return head
	case "gap":
		if strings.HasPrefix(rest, "x-") {
			return "gap-x"
		}
		if strings.HasPrefix(rest, "y-") {
			return "gap-y"
		}
		return "gap"
	case "w", "h", "size", "z", "leading", "tracking", "opacity":
		return head
	case "shadow":
		if shadowSizes[rest] || (rest != "" && rest[0] == '[') {
			return "shadow"
		}
		return "shadow-color"
	case "ring":
		switch {
		case rest == "inset":
			return ""
		case numberLike(rest) || arbitraryLength(rest):
			return "ring-w"
		default:
			return "ring-color"
		}
	case "min", "max":
		if strings.HasPrefix(rest, "w-") {
			return head + "-w"
		}
		if strings.HasPrefix(rest, "h-") {
			return head + "-h"
		}
		return ""
	case "text":
		switch rest {
		case "left", "center", "right", "justify", "start", "end":
			return "text-align"
		case "wrap", "nowrap", "balance", "pretty":
			return "text-wrap"
		}
		// text-sm/relaxed sets the size with a line height.
		size := rest
		if j := indexByte(size, '/'); j >= 0 {
			size = size[:j]
		}
		if textSizes[size] || isFontSizeNumber(size) || arbitraryLength(size) {
			return "text-size"
		}
		return "text-color"
	case "font":
		switch rest {
		case "thin", "extralight", "light", "normal", "medium", "semibold", "bold", "extrabold", "black":
			return "font-weight"
		default:
			return "font-family"
		}
	case "bg":
		switch {
		case strings.HasPrefix(rest, "repeat"), strings.HasPrefix(rest, "no-repeat"),
			rest == "cover", rest == "contain", rest == "fixed", rest == "local",
			strings.HasPrefix(rest, "bottom"), strings.HasPrefix(rest, "top"),
			strings.HasPrefix(rest, "left"), strings.HasPrefix(rest, "right"),
			strings.HasPrefix(rest, "center"):
			return "bg-position"
		default:
			return "bg-color"
		}
	case "border":
		switch rest {
		case "x", "y", "t", "r", "b", "l", "s", "e":
			// A bare side is a 1px width.
			return "border-" + rest + "-w"
		}
		if len(rest) > 1 && rest[1] == '-' {
			side := rest[:1]
			switch side {
			case "x", "y", "t", "r", "b", "l", "s", "e":
				value := rest[2:]
				if numberLike(value) || arbitraryLength(value) {
					return "border-" + side + "-w"
				}
				return "border-" + side + "-color"
			}
		}
		switch {
		case numberLike(rest) || arbitraryLength(rest):
			return "border-w"
		case rest == "solid" || rest == "dashed" || rest == "dotted" || rest == "double" || rest == "none":
			return "border-style"
		default:
			return "border-color"
		}
	case "rounded":
		return roundGroup(rest)
	case "justify", "items", "content", "self", "cursor", "select", "resize", "object":
		return head
	case "place":
		switch {
		case strings.HasPrefix(rest, "items-"):
			return "place-items"
		case strings.HasPrefix(rest, "content-"):
			return "place-content"
		case strings.HasPrefix(rest, "self-"):
			return "place-self"
		}
		return ""
	case "overflow", "overscroll":
		if strings.HasPrefix(rest, "x-") {
			return head + "-x"
		}
		if strings.HasPrefix(rest, "y-") {
			return head + "-y"
		}
		return head
	case "inset":
		return "inset"
	case "top", "right", "bottom", "left", "start", "end":
		return head
	case "flex":
		return "flex-direction"
	case "grid":
		switch {
		case strings.HasPrefix(rest, "cols-"):
			return "grid-cols"
		case strings.HasPrefix(rest, "rows-"):
			return "grid-rows"
		default:
			return "grid-flow"
		}
	}
	return ""
}

// roundGroup maps a rounded- value to its corner group.
func roundGroup(rest string) string {
	if len(rest) > 1 && rest[1] == '-' {
		switch rest[:1] {
		case "t", "r", "b", "l", "s", "e":
			return "rounded-" + rest[:1]
		}
	}
	if len(rest) > 2 && rest[2] == '-' {
		switch rest[:2] {
		case "tl", "tr", "br", "bl", "ss", "se", "ee", "es":
			return "rounded-" + rest[:2]
		}
	}
	return "rounded"
}

// indexByte returns the index of the first c in s, or -1.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// numberLike reports whether a value is a number, an arbitrary value or a
// named spacing value.
func numberLike(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '[' || value == "px" || value == "auto" || value == "full" {
		return true
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= '0' && c <= '9') || c == '.' {
			continue
		}
		return false
	}
	return true
}

// isFontSizeNumber reports whether a text- value is a size number.
func isFontSizeNumber(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= '0' && c <= '9') || c == '.' {
			continue
		}
		return false
	}
	return true
}

// arbitraryLength reports whether a value is an arbitrary length such as
// [3px] or [0.5rem], and not an arbitrary colour or variable.
func arbitraryLength(value string) bool {
	if len(value) < 3 || value[0] != '[' || value[len(value)-1] != ']' {
		return false
	}
	inner := value[1 : len(value)-1]
	if strings.HasPrefix(inner, "length:") {
		return true
	}
	c := inner[0]
	return (c >= '0' && c <= '9') || c == '.' || strings.HasPrefix(inner, "calc(")
}

// shadowSizes are the box shadow sizes of Tailwind v4. Anything else after
// shadow- is a color.
var shadowSizes = map[string]bool{
	"2xs": true, "xs": true, "sm": true, "md": true, "lg": true, "xl": true,
	"2xl": true, "none": true, "inner": true,
}

// textSizes are the text sizes of Tailwind v4. Anything else after text- is
// a color.
var textSizes = map[string]bool{
	"xs": true, "sm": true, "base": true, "lg": true, "xl": true,
	"2xl": true, "3xl": true, "4xl": true, "5xl": true, "6xl": true,
	"7xl": true, "8xl": true, "9xl": true,
}

// staticGroups maps exact class names to their conflict group.
var staticGroups = map[string]string{
	"block": "display", "inline-block": "display", "inline": "display",
	"flex": "display", "inline-flex": "display", "table": "display",
	"grid": "display", "inline-grid": "display", "contents": "display",
	"hidden": "display", "flow-root": "display",

	"static": "position", "fixed": "position", "absolute": "position",
	"relative": "position", "sticky": "position",

	"visible": "visibility", "invisible": "visibility",

	"underline": "text-decoration", "overline": "text-decoration",
	"line-through": "text-decoration", "no-underline": "text-decoration",

	"italic": "font-style", "not-italic": "font-style",

	"flex-row": "flex-direction", "flex-row-reverse": "flex-direction",
	"flex-col": "flex-direction", "flex-col-reverse": "flex-direction",

	"flex-wrap": "flex-wrap", "flex-wrap-reverse": "flex-wrap", "flex-nowrap": "flex-wrap",

	"flex-1": "flex", "flex-auto": "flex", "flex-initial": "flex", "flex-none": "flex",

	"grow": "grow", "grow-0": "grow", "shrink": "shrink", "shrink-0": "shrink",

	"uppercase": "text-transform", "lowercase": "text-transform",
	"capitalize": "text-transform", "normal-case": "text-transform",

	"truncate": "text-overflow", "text-ellipsis": "text-overflow",
	"text-clip": "text-overflow",

	"antialiased": "font-smoothing", "subpixel-antialiased": "font-smoothing",

	"outline-none": "outline", "outline": "outline",
	"ring": "ring-w", "ring-0": "ring-w", "ring-1": "ring-w", "ring-2": "ring-w",

	"sr-only": "sr", "not-sr-only": "sr",

	"rounded": "rounded", "border": "border-w", "shadow": "shadow",
	"opacity": "opacity", "font": "font-weight",
}

// conflictGroups lists the groups a class of the key group replaces.
var conflictGroups = map[string][]string{
	"p":          {"px", "py", "ps", "pe", "pt", "pr", "pb", "pl"},
	"px":         {"pr", "pl"},
	"py":         {"pt", "pb"},
	"m":          {"mx", "my", "ms", "me", "mt", "mr", "mb", "ml"},
	"mx":         {"mr", "ml"},
	"my":         {"mt", "mb"},
	"border-w":   {"border-x-w", "border-y-w", "border-t-w", "border-r-w", "border-b-w", "border-l-w"},
	"border-x-w": {"border-l-w", "border-r-w"},
	"border-y-w": {"border-t-w", "border-b-w"},
	"rounded":    {"rounded-t", "rounded-r", "rounded-b", "rounded-l", "rounded-tl", "rounded-tr", "rounded-br", "rounded-bl"},
	"rounded-t":  {"rounded-tl", "rounded-tr"},
	"rounded-r":  {"rounded-tr", "rounded-br"},
	"rounded-b":  {"rounded-bl", "rounded-br"},
	"rounded-l":  {"rounded-tl", "rounded-bl"},
	"inset":      {"top", "right", "bottom", "left", "start", "end"},
	"size":       {"w", "h"},
	"overflow":   {"overflow-x", "overflow-y"},
	"overscroll": {"overscroll-x", "overscroll-y"},
}
