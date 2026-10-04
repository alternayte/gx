package compiler

import "strings"

// contentNode is one prose chunk or component tag of a content body
// (REQ-CNT-03).
type contentNode struct {
	text string
	tag  *contentTag
	kids []contentNode
}

// contentTag is one component tag in a content body.
type contentTag struct {
	// name is the component name, without the package qualifier.
	name  string
	qual  string
	attrs []contentAttr
	pos   Pos
}

// contentAttr is one attribute of a content tag.
type contentAttr struct {
	name  string
	value string
	// expr marks a {expr} value.
	expr bool
	pos  Pos
}

// parseContentTree parses a Markdown body into prose chunks and nested
// component tags. Fenced code, inline code and HTML comments stay text
// (REQ-CNT-03).
func parseContentTree(src string) ([]contentNode, []Diagnostic) {
	root := &contentNode{}
	stack := []*contentNode{root}
	var prose strings.Builder
	var diags []Diagnostic
	line, col := 1, 1
	i := 0
	atLineStart := true

	advance := func(n int) {
		for k := 0; k < n; k++ {
			if i >= len(src) {
				return
			}
			if src[i] == '\n' {
				line++
				col = 1
				atLineStart = true
			} else {
				col++
				if !isTagSpace(src[i]) {
					atLineStart = false
				}
			}
			i++
		}
	}
	flush := func() {
		if prose.Len() == 0 {
			return
		}
		top := stack[len(stack)-1]
		top.kids = append(top.kids, contentNode{text: prose.String()})
		prose.Reset()
	}
	open := func(tag *contentTag) {
		flush()
		top := stack[len(stack)-1]
		top.kids = append(top.kids, contentNode{tag: tag})
		stack = append(stack, &top.kids[len(top.kids)-1])
	}
	closeTag := func(name string, at Pos) {
		if len(stack) == 1 {
			diags = append(diags, contentParseDiag(at, "closing tag </"+name+"> has no matching open tag"))
			return
		}
		top := stack[len(stack)-1]
		if top.tag == nil || top.tag.name != name {
			diags = append(diags, contentParseDiag(at, "closing tag </"+name+"> does not match <"+openName(top)+">"))
			return
		}
		flush()
		stack = stack[:len(stack)-1]
	}
	for i < len(src) {
		switch {
		case atLineStart && (strings.HasPrefix(src[i:], "```") || strings.HasPrefix(src[i:], "~~~")):
			fence := src[i : i+3]
			// Copy the fence line.
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				prose.WriteString(src[i:])
				advance(len(src) - i)
				continue
			}
			prose.WriteString(src[i : i+end+1])
			advance(end + 1)
			// Copy until the closing fence.
			for i < len(src) {
				lineEnd := strings.IndexByte(src[i:], '\n')
				stop := len(src)
				if lineEnd >= 0 {
					stop = i + lineEnd + 1
				}
				prose.WriteString(src[i:stop])
				trimmed := strings.TrimSpace(src[i:stop])
				advance(stop - i)
				if strings.HasPrefix(trimmed, fence[:1]) && strings.HasPrefix(trimmed, strings.Repeat(fence[:1], 3)) {
					break
				}
			}
		case strings.HasPrefix(src[i:], "<!--"):
			end := strings.Index(src[i:], "-->")
			if end < 0 {
				prose.WriteString(src[i:])
				advance(len(src) - i)
				continue
			}
			prose.WriteString(src[i : i+end+3])
			advance(end + 3)
		case src[i] == '`':
			run := 0
			for i+run < len(src) && src[i+run] == '`' {
				run++
			}
			marker := strings.Repeat("`", run)
			end := strings.Index(src[i+run:], marker)
			if end < 0 {
				prose.WriteString(src[i:])
				advance(len(src) - i)
				continue
			}
			stop := i + run + end + run
			prose.WriteString(src[i:stop])
			advance(stop - i)
		case strings.HasPrefix(src[i:], "</"):
			name, n := readTagName(src[i+2:])
			if name == "" || !isComponentTagName(name) {
				// An HTML closing tag: keep it as text.
				stop := strings.IndexByte(src[i:], '>')
				if stop < 0 {
					stop = len(src) - i
				} else {
					stop++
				}
				prose.WriteString(src[i : i+stop])
				advance(stop)
				continue
			}
			at := Pos{Line: line, Col: col}
			closeTag(baseName(name), at)
			advance(2 + n)
			// Skip to the end of the tag.
			stop := strings.IndexByte(src[i:], '>')
			if stop < 0 {
				stop = len(src) - i
			} else {
				stop++
			}
			advance(stop)
		case src[i] == '<':
			name, _ := readTagName(src[i+1:])
			if name == "" || !isComponentTagName(name) {
				stop := strings.IndexByte(src[i:], '>')
				if stop < 0 {
					stop = len(src) - i
				} else {
					stop++
				}
				prose.WriteString(src[i : i+stop])
				advance(stop)
				continue
			}
			at := Pos{Line: line, Col: col}
			tag, selfClose, stop := scanContentTag(src[i:], name, at)
			advance(stop)
			if tag == nil {
				continue
			}
			if selfClose {
				flush()
				top := stack[len(stack)-1]
				top.kids = append(top.kids, contentNode{tag: tag})
				continue
			}
			open(tag)
		default:
			prose.WriteByte(src[i])
			advance(1)
		}
	}
	flush()
	for _, open := range stack[1:] {
		diags = append(diags, contentParseDiag(open.tag.pos, "unclosed component tag <"+openName(open)+">"))
	}
	return root.kids, diags
}

// openName returns the tag name of an open node.
func openName(n *contentNode) string {
	if n.tag == nil {
		return ""
	}
	if n.tag.qual != "" {
		return n.tag.qual + "." + n.tag.name
	}
	return n.tag.name
}

// contentParseDiag reports a malformed content tag (GX1000).
func contentParseDiag(at Pos, msg string) Diagnostic {
	return Diagnostic{Code: CodeParse, Line: at.Line, Col: at.Col, Msg: msg}
}

// scanContentTag reads one component tag from src starting at "<name".
// It returns the tag, whether it is self-closing, and the bytes consumed.
func scanContentTag(src, name string, at Pos) (*contentTag, bool, int) {
	qual := ""
	base := name
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		qual, base = name[:dot], name[dot+1:]
	}
	tag := &contentTag{name: base, qual: qual, pos: at}
	i := 1 + len(name)
	for i < len(src) {
		for i < len(src) && isTagSpace(src[i]) {
			i++
		}
		if i >= len(src) {
			break
		}
		if src[i] == '>' {
			return tag, false, i + 1
		}
		if src[i] == '/' && i+1 < len(src) && src[i+1] == '>' {
			return tag, true, i + 2
		}
		// Attribute name.
		start := i
		for i < len(src) && !isTagSpace(src[i]) && !strings.ContainsRune("=/>", rune(src[i])) {
			i++
		}
		attrName := src[start:i]
		if attrName == "" {
			i++
			continue
		}
		attr := contentAttr{name: attrName, pos: at}
		for i < len(src) && isTagSpace(src[i]) {
			i++
		}
		if i < len(src) && src[i] == '=' {
			i++
			for i < len(src) && isTagSpace(src[i]) {
				i++
			}
			if i < len(src) && src[i] == '{' {
				end := matchBrace(src, i)
				attr.expr = true
				attr.value = strings.TrimSpace(src[i+1 : end])
				i = end + 1
			} else if i < len(src) && (src[i] == '"' || src[i] == '\'') {
				quote := src[i]
				i++
				start = i
				for i < len(src) && src[i] != quote {
					i++
				}
				attr.value = src[start:i]
				if i < len(src) {
					i++
				}
			}
		}
		tag.attrs = append(tag.attrs, attr)
	}
	return tag, true, len(src)
}

// matchBrace returns the index of the closing brace of the expression that
// starts at open.
func matchBrace(src string, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			quote := src[i]
			i++
			for i < len(src) && src[i] != quote {
				if src[i] == '\\' {
					i++
				}
				i++
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src) - 1
}

// readTagName reads a tag name at the start of s.
func readTagName(s string) (string, int) {
	i := 0
	for i < len(s) && (isTagNameByte(s[i]) || s[i] == '.' || s[i] == '-') {
		i++
	}
	return s[:i], i
}

// baseName returns the component name without its qualifier.
func baseName(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

// isComponentTagName reports whether a tag names a component.
func isComponentTagName(name string) bool {
	base := baseName(name)
	return base != "" && isUpperByte(base[0])
}

func isTagNameByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isTagSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// walkContentTags calls fn for every component tag of a content tree, in
// source order.
func walkContentTags(nodes []contentNode, fn func(*contentTag)) {
	for i := range nodes {
		if nodes[i].tag != nil {
			fn(nodes[i].tag)
		}
		walkContentTags(nodes[i].kids, fn)
	}
}
