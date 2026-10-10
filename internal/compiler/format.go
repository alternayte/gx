package compiler

import (
	"bytes"
	"fmt"
	"go/format"
	goparser "go/parser"
	goprinter "go/printer"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// FormatSource parses src and returns its canonical form (REQ-AUT-17). It
// returns the parse diagnostics instead when src does not parse.
func FormatSource(file string, src []byte) ([]byte, []Diagnostic) {
	f, diags := ParseFile(file, src)
	if len(diags) > 0 {
		return nil, diags
	}
	return Format(f), nil
}

// Format returns the canonical source of f (REQ-AUT-17).
func Format(f *File) []byte {
	pr := &printer{}
	started := false
	if f.Package != "" {
		pr.write("package " + f.Package + "\n")
		started = true
	}
	if len(f.Imports) > 0 {
		if started {
			pr.write("\n")
		}
		for _, im := range formatImports(f.Imports) {
			pr.write("import " + im.Raw + "\n")
		}
		started = true
	}
	if f.HasProps {
		if started {
			pr.write("\n")
		}
		pr.write("props {\n")
		pr.writeFields(f.Props)
		pr.write("}\n")
		started = true
	}
	if f.HasSignals {
		if started {
			pr.write("\n")
		}
		pr.write("signals {\n")
		pr.writeFields(f.Signals)
		pr.write("}\n")
		started = true
	}
	if len(f.Body) > 0 {
		if body := trimWhitespaceEdges(f.Body); len(body) > 0 {
			if started {
				pr.write("\n")
			}
			pr.writeNodes(body, 0)
			pr.write("\n")
		}
	}
	out := strings.TrimRight(pr.b.String(), " \t\n") + "\n"
	return []byte(out)
}

type printer struct {
	b strings.Builder
	// keep counts the open elements that show white space as written,
	// such as pre. The printer does not indent inside them: the indent
	// is text in the rendered page (REQ-AUT-17).
	keep int
}

// keepsWhitespace reports whether an element shows its white space.
func keepsWhitespace(name string) bool {
	switch strings.ToLower(name) {
	case "pre", "textarea":
		return true
	}
	return false
}

func (pr *printer) write(s string) { pr.b.WriteString(s) }

func indent(depth int) string {
	if depth <= 0 {
		return ""
	}
	return strings.Repeat("  ", depth)
}

func (pr *printer) writeFields(fields []Field) {
	// label is the name of a field with the word of a shared signal.
	label := func(f Field) string {
		if f.Shared {
			return "shared " + f.Name
		}
		return f.Name
	}
	width := 0
	for _, f := range fields {
		if len(label(f)) > width {
			width = len(label(f))
		}
	}
	for _, f := range fields {
		for _, line := range docLines(f.Doc) {
			pr.write(strings.TrimRight("  // "+line, " ") + "\n")
		}
		pr.write("  " + label(f) + strings.Repeat(" ", width-len(label(f))+1) + formatGoExpr(f.Type))
		if f.HasDefault {
			pr.write(" = " + formatGoExpr(f.Default))
		}
		pr.write("\n")
	}
}

// docLines returns the comment lines of a field description.
func docLines(doc string) []string {
	if doc == "" {
		return nil
	}
	return strings.Split(doc, "\n")
}

// formatImports sorts and dedupes import specs, like gofmt does for imports.
func formatImports(imports []Import) []Import {
	type spec struct{ alias, path string }
	var list []spec
	seen := map[string]bool{}
	for _, im := range imports {
		alias, path, ok := splitImport(im.Raw)
		if !ok || path == "" {
			continue
		}
		key := alias + " " + path
		if seen[key] {
			continue
		}
		seen[key] = true
		list = append(list, spec{alias: alias, path: path})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].path != list[j].path {
			return list[i].path < list[j].path
		}
		return list[i].alias < list[j].alias
	})
	out := make([]Import, 0, len(list))
	for _, s := range list {
		if s.alias == pathBase(s.path) {
			out = append(out, Import{Raw: strconv.Quote(s.path)})
			continue
		}
		out = append(out, Import{Raw: s.alias + " " + strconv.Quote(s.path)})
	}
	return out
}

// formatGoExpr formats a Go expression with go/format (REQ-TLS-01). Client
// expressions keep their $signal names.
func formatGoExpr(src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return src
	}
	masked, names := maskSignals(src)
	node, err := goparser.ParseExpr(masked)
	if err != nil {
		return canonExpr(src)
	}
	var b bytes.Buffer
	if err := goprinter.Fprint(&b, token.NewFileSet(), node); err != nil {
		return canonExpr(src)
	}
	return unmaskSignals(b.String(), names)
}

// formatGoHeader formats the header of an if, for or switch block.
func formatGoHeader(keyword, header string) string {
	header = strings.TrimSpace(header)
	masked, names := maskSignals(header)
	src := "package p\n\nfunc _() {\n\t" + keyword + " " + masked + " {}\n}\n"
	out, err := format.Source([]byte(src))
	if err != nil {
		return canonExpr(header)
	}
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, keyword) {
			continue
		}
		h := strings.TrimSpace(strings.TrimPrefix(t, keyword))
		h = strings.TrimSpace(strings.TrimSuffix(h, "{}"))
		h = strings.TrimSpace(strings.TrimSuffix(h, "{"))
		return unmaskSignals(h, names)
	}
	return canonExpr(header)
}

func maskSignals(src string) (string, []string) {
	var names []string
	var b strings.Builder
	for i := 0; i < len(src); {
		if src[i] != '$' || i+1 >= len(src) || !isIdentStart(src[i+1]) {
			b.WriteByte(src[i])
			i++
			continue
		}
		j := i + 1
		for j < len(src) && isIdentByte(src[j]) {
			j++
		}
		names = append(names, src[i+1:j])
		fmt.Fprintf(&b, "gxSig_%d", len(names)-1)
		i = j
	}
	return b.String(), names
}

func unmaskSignals(s string, names []string) string {
	for i, n := range names {
		s = strings.ReplaceAll(s, fmt.Sprintf("gxSig_%d", i), "$"+n)
	}
	return s
}

func isIdentStart(c byte) bool { return c == '_' || isLetter(c) }

func isWhitespaceText(n Node) bool {
	t, ok := n.(*Text)
	return ok && strings.TrimSpace(t.Data) == ""
}

func (pr *printer) writeNodes(ns []Node, depth int) {
	for i, n := range ns {
		if t, ok := n.(*Text); ok && strings.TrimSpace(t.Data) == "" {
			switch {
			case pr.keep > 0:
				pr.write(t.Data)
			case i == len(ns)-1 && strings.Contains(t.Data, "\n"):
				pr.write("\n" + indent(depth-1))
			case strings.Contains(t.Data, "\n"):
				pr.write("\n" + indent(depth))
			default:
				pr.write(" ")
			}
			continue
		}
		pr.writeNode(n, depth)
	}
}

func (pr *printer) writeNode(n Node, depth int) {
	switch t := n.(type) {
	case *Text:
		pr.write(t.Data)
	case *Expr:
		pr.write("{" + formatGoExpr(t.Data) + "}")
	case *Comment:
		pr.write("{/*" + t.Data + "*/}")
	case *HTMLComment:
		pr.write("<!--" + t.Data + "-->")
	case *Let:
		pr.write(t.Name + " := " + formatGoExpr(t.Expr))
	case *Element:
		pr.writeElement(t, depth)
	case *Control:
		pr.writeControl(t, depth)
	}
}

// trimWhitespaceEdges drops whitespace-only text nodes at both ends.
func trimWhitespaceEdges(ns []Node) []Node {
	start, end := 0, len(ns)
	for start < end && isWhitespaceText(ns[start]) {
		start++
	}
	for end > start && isWhitespaceText(ns[end-1]) {
		end--
	}
	return ns[start:end]
}

// trimBlockEdges removes the layout whitespace at the start and end of a
// block body: a leading or trailing whitespace run that contains a newline is
// replaced by the block indentation the printer writes itself.
func trimBlockEdges(ns []Node) []Node {
	if len(ns) == 0 {
		return ns
	}
	out := make([]Node, len(ns))
	copy(out, ns)
	if t, ok := out[0].(*Text); ok {
		nt := *t
		trimTextEdge(&nt, true)
		if strings.TrimSpace(nt.Data) == "" {
			out = out[1:]
		} else {
			out[0] = &nt
		}
	}
	if len(out) > 0 {
		if t, ok := out[len(out)-1].(*Text); ok {
			nt := *t
			trimTextEdge(&nt, false)
			if strings.TrimSpace(nt.Data) == "" {
				out = out[:len(out)-1]
			} else {
				out[len(out)-1] = &nt
			}
		}
	}
	return out
}

func trimTextEdge(t *Text, leading bool) {
	if leading {
		j := 0
		for j < len(t.Data) && isSpaceByte(t.Data[j]) {
			j++
		}
		t.Data = t.Data[j:]
		return
	}
	j := len(t.Data)
	for j > 0 && isSpaceByte(t.Data[j-1]) {
		j--
	}
	t.Data = t.Data[:j]
}

// writeBlock writes a block body with one newline and one indent level.
func (pr *printer) writeBlock(ns []Node, depth int) {
	body := trimBlockEdges(trimWhitespaceEdges(ns))
	if len(body) == 0 {
		pr.write("\n" + indent(depth-1))
		return
	}
	pr.write("\n" + indent(depth))
	pr.writeNodes(body, depth)
	pr.write("\n" + indent(depth-1))
}

func (pr *printer) writeControl(c *Control, depth int) {
	switch c.Kind {
	case "switch":
		header := strings.TrimSpace(c.Header)
		if header == "" {
			pr.write("switch {")
		} else {
			pr.write("switch " + formatGoHeader("switch", header) + " {")
		}
		for _, cs := range c.Cases {
			pr.write("\n" + indent(depth+1))
			if cs.IsDefault {
				pr.write("default:")
			} else {
				pr.write("case " + canonExpr(cs.Header) + ":")
			}
			body := trimWhitespaceEdges(cs.Body)
			if len(body) == 0 {
				continue
			}
			pr.write("\n" + indent(depth+2))
			pr.writeNodes(body, depth+2)
		}
		pr.write("\n" + indent(depth) + "}")
	default:
		pr.writeIfChain(c, depth)
	}
}

func (pr *printer) writeIfChain(c *Control, depth int) {
	header := strings.TrimSpace(c.Header)
	if header == "" {
		pr.write(c.Kind + " {")
	} else {
		pr.write(c.Kind + " " + formatGoHeader(c.Kind, header) + " {")
	}
	pr.writeBlock(c.Body, depth+1)
	pr.write("}")
	if len(c.Else) == 0 {
		return
	}
	pr.write(" else ")
	if ec, ok := c.Else[0].(*Control); ok && ec.Kind == "if" {
		pr.writeIfChain(ec, depth)
		return
	}
	pr.write("{")
	pr.writeBlock(c.Else, depth+1)
	pr.write("}")
}

func (pr *printer) writeElement(el *Element, depth int) {
	pr.write("<" + el.Name)
	for _, a := range el.Attrs {
		pr.write(" " + formatAttr(a))
	}
	if el.SelfClose {
		pr.write(" />")
		return
	}
	pr.write(">")
	if voidElements[strings.ToLower(el.Name)] {
		return
	}
	if el.HasRaw {
		pr.write(el.RawText)
	} else {
		if keepsWhitespace(el.Name) {
			pr.keep++
			defer func() { pr.keep-- }()
		}
		pr.writeNodes(el.Children, depth+1)
	}
	pr.write("</" + el.Name + ">")
}

func formatAttr(a Attr) string {
	switch a.Kind {
	case AttrString:
		if strings.Contains(a.Value, `"`) {
			return a.Name + "='" + a.Value + "'"
		}
		return a.Name + `="` + a.Value + `"`
	case AttrExpr:
		return a.Name + "={" + formatGoExpr(a.Value) + "}"
	case AttrSpread:
		return "{..." + canonExpr(a.Value) + "}"
	case AttrFragment:
		if a.Value == "" {
			return "#" + a.Name
		}
		return "#" + a.Name + "(" + canonExpr(a.Value) + ")"
	default:
		return a.Name
	}
}

// canonExpr is the canonical form of a Go or client expression: whitespace
// runs outside strings and comments become one space (REQ-AUT-17).
func canonExpr(s string) string {
	var b strings.Builder
	i := 0
	pending := false
	for i < len(s) {
		c := s[i]
		switch {
		case c == '"' || c == '\'' || c == '`':
			j := i + 1
			for j < len(s) {
				if s[j] == '\\' && c != '`' {
					j += 2
					continue
				}
				if s[j] == c {
					j++
					break
				}
				j++
			}
			if j > len(s) {
				j = len(s)
			}
			if pending && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pending = false
			b.WriteString(s[i:j])
			i = j
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			j := strings.IndexByte(s[i:], '\n')
			if pending && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pending = false
			if j < 0 {
				b.WriteString(s[i:])
				i = len(s)
				continue
			}
			j = i + j + 1
			b.WriteString(s[i:j])
			i = j
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			j := strings.Index(s[i+2:], "*/")
			if j < 0 {
				j = len(s)
			} else {
				j = i + 2 + j + 2
			}
			if pending && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pending = false
			b.WriteString(s[i:j])
			i = j
		case isSpaceByte(c):
			pending = true
			i++
		default:
			if pending && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pending = false
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}
