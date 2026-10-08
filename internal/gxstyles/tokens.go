package gxstyles

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

// varRef is one read of a custom property.
var varRef = regexp.MustCompile(`var\(\s*(--[A-Za-z0-9_-]+)`)

// WidgetTokens returns the CSS variables of a widget (REQ-ISL-11,
// REQ-ISL-13): the tokens that a host page can set on the element and that
// the stylesheet of the widget reads. A token is a custom property that a
// rule of the host element declares outside each layer: after ShadowCSS that
// is each token of the :root and .dark rules of the theme. A token counts
// when a rule of the stylesheet reads it, or when a token that counts reads
// it. The input is a stylesheet from ShadowCSS.
func WidgetTokens(css []byte) []string {
	values := map[string][]string{}
	var rest bytes.Buffer
	scanTokens(css, values, &rest)
	used := map[string]bool{}
	var visit func(text string)
	visit = func(text string) {
		for _, m := range varRef.FindAllStringSubmatch(text, -1) {
			name := m[1]
			vals, ok := values[name]
			if !ok || used[name] {
				continue
			}
			used[name] = true
			for _, v := range vals {
				visit(v)
			}
		}
	}
	visit(string(dropStrings(rest.Bytes())))
	out := make([]string, 0, len(used))
	for name := range used {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// dropStrings returns css with no string: the text of a string is not a
// read of a token.
func dropStrings(css []byte) []byte {
	out := make([]byte, 0, len(css))
	for i := 0; i < len(css); i++ {
		if css[i] == '"' || css[i] == '\'' {
			i = stringEnd(css, i)
			continue
		}
		out = append(out, css[i])
	}
	return out
}

// scanTokens walks the rules of one level of a stylesheet. It puts the
// custom properties of the rules of the host element into values, and each
// other part of the text into rest.
func scanTokens(css []byte, values map[string][]string, rest *bytes.Buffer) {
	for i := 0; i < len(css); {
		open := indexOutsideStrings(css, i, "{;}")
		if open < 0 {
			rest.Write(css[i:])
			return
		}
		if css[open] != '{' {
			// A statement with no block, such as @import.
			rest.Write(css[i : open+1])
			i = open + 1
			continue
		}
		end := closeOf(css, open)
		prelude := strings.TrimSpace(string(css[i:open]))
		body := css[open+1 : end]
		switch {
		case strings.HasPrefix(prelude, "@media") || strings.HasPrefix(prelude, "@supports"):
			// A condition around rules: the rules keep their level.
			rest.WriteString(prelude + "{")
			scanTokens(body, values, rest)
			rest.WriteString("}")
		case !strings.HasPrefix(prelude, "@") && hostOnly(prelude):
			for _, decl := range splitOutside(body, ';') {
				name, value, ok := strings.Cut(decl, ":")
				name = strings.TrimSpace(name)
				if ok && strings.HasPrefix(name, "--") {
					values[name] = append(values[name], value)
					continue
				}
				// A property of the host element can read a token.
				rest.WriteString(decl + ";")
			}
		default:
			rest.Write(css[i:min(end+1, len(css))])
		}
		i = end + 1
	}
}

// hostOnly reports whether each selector of a rule is the host element
// itself: ":host" or ":host(...)".
func hostOnly(prelude string) bool {
	for _, sel := range splitOutside([]byte(prelude), ',') {
		sel = strings.TrimSpace(sel)
		arg, ok := strings.CutPrefix(sel, ":host")
		if !ok {
			return false
		}
		if arg == "" {
			continue
		}
		if arg[0] != '(' || closeParen(arg) != len(arg)-1 {
			return false
		}
	}
	return true
}

// closeParen returns the index of the parenthesis that closes the one at
// the start of s, or -1.
func closeParen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// indexOutsideStrings returns the index of the first byte of chars at or
// after from that is not in a string, or -1.
func indexOutsideStrings(css []byte, from int, chars string) int {
	for i := from; i < len(css); i++ {
		switch c := css[i]; {
		case c == '"' || c == '\'':
			i = stringEnd(css, i)
		case strings.IndexByte(chars, c) >= 0:
			return i
		}
	}
	return -1
}

// stringEnd returns the index of the quote that closes the string at i, or
// the last index of css.
func stringEnd(css []byte, i int) int {
	quote := css[i]
	for j := i + 1; j < len(css); j++ {
		switch css[j] {
		case '\\':
			j++
		case quote:
			return j
		}
	}
	return len(css) - 1
}

// closeOf returns the index of the brace that closes the block at open, or
// the length of css for a block with no end.
func closeOf(css []byte, open int) int {
	depth := 0
	for i := open; i < len(css); i++ {
		switch c := css[i]; c {
		case '"', '\'':
			i = stringEnd(css, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(css)
}

// splitOutside splits text at sep, outside strings and parentheses.
func splitOutside(text []byte, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case c == '"' || c == '\'':
			i = stringEnd(text, i)
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			out = append(out, string(text[start:i]))
			start = i + 1
		}
	}
	if part := string(text[start:]); strings.TrimSpace(part) != "" {
		out = append(out, part)
	}
	return out
}
