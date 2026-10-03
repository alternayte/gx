// Package compiler parses .gx files into an AST. The AST is the input of the
// formatter, the type checker and the code generator.
package compiler

import (
	"path/filepath"
	"strings"
)

type parser struct {
	file     string
	src      string
	off      int
	diags    []Diagnostic
	fragment bool
}

type bailout struct{}

// ParseFile parses a .gx file. file may be empty for input with no file name,
// such as stdin; the component name check is then skipped.
func ParseFile(file string, src []byte) (*File, []Diagnostic) {
	p := &parser{file: file, src: string(src)}
	f := p.run()
	return f, p.diags
}

// ParseFragment parses markup with no package clause, imports, props or
// signals. It is used by the HTML corpus tests.
func ParseFragment(src []byte) (*File, []Diagnostic) {
	p := &parser{src: string(src), fragment: true}
	f := p.run()
	return f, p.diags
}

func (p *parser) run() (f *File) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); !ok {
				panic(r)
			}
			f = nil
		}
	}()
	return p.parseFile()
}

func (p *parser) fail(off int, code, msg string) {
	p.errAt(off, code, msg)
	panic(bailout{})
}

func (p *parser) errAt(off int, code, msg string) {
	if off > len(p.src) {
		off = len(p.src)
	}
	if off < 0 {
		off = 0
	}
	line, col := 1, 1
	for i := 0; i < off; i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	p.diags = append(p.diags, Diagnostic{Code: code, File: p.file, Line: line, Col: col, Msg: msg})
}

func (p *parser) posAt(off int) Pos {
	line, col := 1, 1
	if off > len(p.src) {
		off = len(p.src)
	}
	for i := 0; i < off; i++ {
		if p.src[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return Pos{Line: line, Col: col}
}

func (p *parser) eof() bool         { return p.off >= len(p.src) }
func (p *parser) rest() string      { return p.src[p.off:] }
func (p *parser) has(s string) bool { return strings.HasPrefix(p.rest(), s) }

func (p *parser) take(s string) bool {
	if p.has(s) {
		p.off += len(s)
		return true
	}
	return false
}

func (p *parser) parseFile() *File {
	f := &File{File: p.file}
	if p.fragment {
		f.Body = p.parseNodes(p.eof)
		return f
	}
	p.skipSpaceNewlinesAndComments()
	if !p.takeKeyword("package") {
		p.fail(p.off, CodeParse, "expected a package clause")
	}
	p.skipSpaces()
	name := p.readIdent()
	if name == "" {
		p.fail(p.off, CodeParse, "expected a package name")
	}
	f.Package = name
	if !p.skipLine() {
		p.fail(p.off, CodeParse, "unexpected text after the package clause")
	}
	if base := strings.TrimSuffix(filepath.Base(p.file), ".gx"); p.file != "" && !isExportedIdent(base) {
		p.errAt(0, CodeFileName, "component file name "+Quoted(base)+" must be an exported Go identifier")
	}
	for {
		p.skipSpaceNewlinesAndComments()
		if !p.atKeyword("import") {
			break
		}
		f.Imports = append(f.Imports, p.parseImport()...)
	}
	if p.atKeyword("props") {
		p.off += len("props")
		p.skipSpaces()
		if !p.take("{") {
			p.fail(p.off, CodeParse, "props must be followed by {")
		}
		f.HasProps = true
		f.Props = p.parseFields()
	}
	p.skipSpaceNewlinesAndComments()
	if p.atKeyword("signals") {
		p.off += len("signals")
		p.skipSpaces()
		if !p.take("{") {
			p.fail(p.off, CodeParse, "signals must be followed by {")
		}
		f.HasSignals = true
		f.Signals = p.parseFields()
	}
	f.Body = p.parseNodes(p.eof)
	return f
}

func (p *parser) parseImport() []Import {
	start := p.off
	p.off += len("import")
	p.skipSpaces()
	if p.take("(") {
		var out []Import
		for {
			p.skipSpaceNewlinesAndComments()
			if p.take(")") {
				return out
			}
			if p.eof() {
				p.fail(start, CodeParse, "unclosed import block")
			}
			lineStart := p.off
			p.skipToLineEnd()
			if raw := strings.TrimSpace(p.src[lineStart:p.off]); raw != "" {
				out = append(out, Import{Raw: raw})
			}
		}
	}
	lineStart := p.off
	p.skipToLineEnd()
	return []Import{{Raw: strings.TrimSpace(p.src[lineStart:p.off])}}
}

func (p *parser) parseFields() []Field {
	var fields []Field
	for {
		p.skipSpaceNewlinesAndComments()
		if p.take("}") {
			return fields
		}
		if p.eof() {
			p.fail(p.off, CodeParse, "unclosed block: expected }")
		}
		name := p.readIdent()
		if name == "" {
			p.fail(p.off, CodeParse, "expected a field name")
		}
		typeStart := p.off
		depth := 0
		for !p.eof() {
			c := p.src[p.off]
			if c == '\n' && depth == 0 {
				break
			}
			if c == '}' && depth == 0 {
				break
			}
			if c == '=' && depth == 0 && !p.isDoubleEquals() {
				break
			}
			if c == '(' || c == '[' || c == '{' {
				depth++
			}
			if c == ')' || c == ']' || c == '}' {
				depth--
			}
			p.off++
		}
		field := Field{Name: name, Type: canonExpr(p.src[typeStart:p.off])}
		if p.has("=") {
			p.off++
			defStart := p.off
			depth = 0
			for !p.eof() {
				c := p.src[p.off]
				if c == '\n' && depth == 0 {
					break
				}
				if c == '}' && depth == 0 {
					break
				}
				if c == '(' || c == '[' || c == '{' {
					depth++
				}
				if c == ')' || c == ']' || c == '}' {
					depth--
				}
				p.off++
			}
			field.Default = canonExpr(p.src[defStart:p.off])
			field.HasDefault = true
		}
		fields = append(fields, field)
	}
}

func (p *parser) isDoubleEquals() bool {
	if p.off == 0 || p.off+1 >= len(p.src) {
		return false
	}
	return p.src[p.off-1] == '=' || p.src[p.off+1] == '='
}

func (p *parser) parseNodes(stop func() bool) []Node {
	var nodes []Node
	for {
		if p.eof() || stop() {
			return nodes
		}
		start := p.off
		n := p.parseNode()
		if n == nil {
			p.fail(start, CodeParse, "unexpected "+Quoted(p.src[start:min(start+1, len(p.src))]))
		}
		nodes = append(nodes, n)
		if p.off == start {
			p.fail(start, CodeParse, "parser made no progress")
		}
	}
}

func (p *parser) parseNode() Node {
	switch {
	case p.has("<!--"):
		return p.parseHTMLComment()
	case p.has("{/*"):
		return p.parseGxComment()
	case p.has("{"):
		return p.parseExprNode()
	case p.has("<") && p.off+1 < len(p.src) && isTagStartByte(p.src[p.off+1]):
		return p.parseElement()
	}
	if p.atLineStart() {
		if n := p.tryControl(); n != nil {
			return n
		}
		if n := p.tryLet(); n != nil {
			return n
		}
	}
	start := p.off
	txt := p.parseText()
	if txt == "" {
		return nil
	}
	return &Text{At: p.posAt(start), Data: txt}
}

func (p *parser) parseText() string {
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if c == '<' && p.off+1 < len(p.src) && (isTagStartByte(p.src[p.off+1]) || p.src[p.off+1] == '/') {
			break
		}
		if c == '{' {
			break
		}
		if c == '}' && p.atLineStart() {
			break
		}
		if c == '\n' {
			next := p.off + 1
			for next < len(p.src) && (p.src[next] == ' ' || p.src[next] == '\t') {
				next++
			}
			if p.startsNode(next) {
				p.off = next
				return p.src[start:p.off]
			}
		}
		p.off++
	}
	return p.src[start:p.off]
}

// startsNode reports whether offset begins a construct that ends the current
// text node: markup, an expression, a control block or a statement line.
func (p *parser) startsNode(off int) bool {
	if off >= len(p.src) {
		return false
	}
	switch p.src[off] {
	case '<':
		return off+1 < len(p.src) && (isTagStartByte(p.src[off+1]) || p.src[off+1] == '/')
	case '{':
		return true
	}
	if off != 0 && !p.onlySpaceBeforeOnLine(off) {
		return false
	}
	if p.keywordAt(off, "if") || p.keywordAt(off, "for") || p.keywordAt(off, "switch") {
		return p.lineEndsWithBrace(off)
	}
	if p.keywordAt(off, "case") || strings.HasPrefix(p.src[off:], "default:") {
		return true
	}
	return p.letAt(off)
}

func (p *parser) parseElement() *Element {
	start := p.off
	p.off++ // <
	name := p.readTagName()
	el := &Element{At: p.posAt(start), Name: name}
	for {
		p.skipSpaceNewlines()
		if p.take("/>") {
			el.SelfClose = true
			return el
		}
		if p.take(">") {
			break
		}
		if p.eof() {
			p.fail(start, CodeParse, "unclosed element <"+name+">")
		}
		el.Attrs = append(el.Attrs, p.parseAttr())
	}
	if voidElements[strings.ToLower(name)] {
		return el
	}
	if rawElements[strings.ToLower(name)] {
		el.RawText = p.consumeRawText(name)
		el.HasRaw = true
	} else {
		el.Children = p.parseNodes(func() bool { return p.has("</") })
	}
	if !p.has("</") {
		p.fail(p.off, CodeParse, "unclosed element <"+name+">")
	}
	p.off += 2
	closeName := p.readTagName()
	if closeName != name {
		p.fail(p.off, CodeParse, "closing tag </"+closeName+"> does not match <"+name+">")
	}
	p.skipSpaces()
	if !p.take(">") {
		p.fail(p.off, CodeParse, "expected > to close </"+name+">")
	}
	return el
}

func (p *parser) consumeRawText(name string) string {
	closeTag := "</" + name
	idx := strings.Index(strings.ToLower(p.src[p.off:]), strings.ToLower(closeTag))
	if idx < 0 {
		p.fail(p.off, CodeParse, "unclosed <"+name+">")
	}
	raw := p.src[p.off : p.off+idx]
	p.off += idx
	return raw
}

func (p *parser) parseAttr() Attr {
	start := p.posAt(p.off)
	if p.has("{...") {
		p.off += len("{...")
		inner := p.scanBraceBody()
		return Attr{At: start, Kind: AttrSpread, Name: "...", Value: inner}
	}
	if p.has("#") {
		p.off++
		name := p.readFragmentName()
		if name == "" {
			p.fail(p.off, CodeParse, "expected a fragment name after #")
		}
		value := ""
		if p.has("(") {
			value = p.scanParens()
		}
		return Attr{At: start, Kind: AttrFragment, Name: name, Value: value}
	}
	name := p.readAttrName()
	if name == "" {
		p.fail(p.off, CodeParse, "expected an attribute name")
	}
	a := Attr{At: start, Kind: AttrBool, Name: name}
	if !p.take("=") {
		return a
	}
	switch {
	case p.has("{"):
		a.Kind = AttrExpr
		a.Value = p.scanBraces()
	case p.has(`"`):
		a.Kind = AttrString
		a.Value = p.readQuoted('"')
	case p.has("'"):
		a.Kind = AttrString
		a.Value = p.readQuoted('\'')
	default:
		a.Kind = AttrString
		a.Value = p.readUnquoted()
	}
	return a
}

func (p *parser) parseHTMLComment() Node {
	start := p.posAt(p.off)
	p.off += len("<!--")
	idx := strings.Index(p.src[p.off:], "-->")
	if idx < 0 {
		p.fail(p.off, CodeParse, "unclosed HTML comment")
	}
	data := p.src[p.off : p.off+idx]
	p.off += idx + len("-->")
	return &HTMLComment{At: start, Data: data}
}

func (p *parser) parseGxComment() Node {
	start := p.posAt(p.off)
	p.off += len("{/*")
	idx := strings.Index(p.src[p.off:], "*/}")
	if idx < 0 {
		p.fail(p.off, CodeParse, "unclosed {/* */} comment")
	}
	data := p.src[p.off : p.off+idx]
	p.off += idx + len("*/}")
	return &Comment{At: start, Data: data}
}

func (p *parser) parseExprNode() Node {
	start := p.posAt(p.off)
	return &Expr{At: start, Data: p.scanBraces()}
}

func (p *parser) tryControl() Node {
	for _, kw := range []string{"if", "for", "switch"} {
		if p.atKeyword(kw) {
			if !p.lineEndsWithBrace(p.off) {
				return nil
			}
			return p.parseControl(kw)
		}
	}
	return nil
}

func (p *parser) parseControl(kind string) *Control {
	start := p.posAt(p.off)
	p.off += len(kind)
	headerStart := p.off
	depth := 0
	for !p.eof() {
		c := p.src[p.off]
		if c == '{' && depth == 0 {
			break
		}
		if c == '(' || c == '[' {
			depth++
		}
		if c == ')' || c == ']' {
			depth--
		}
		p.off++
	}
	c := &Control{At: start, Kind: kind, Header: canonExpr(p.src[headerStart:p.off])}
	if !p.take("{") {
		p.fail(p.off, CodeParse, kind+" must be followed by {")
	}
	if kind == "switch" {
		c.Cases = p.parseCases()
	} else {
		c.Body = p.parseNodes(func() bool { return p.has("}") })
	}
	if !p.take("}") {
		p.fail(p.off, CodeParse, "unclosed "+kind+" block")
	}
	if kind == "if" {
		p.parseElse(c)
	}
	return c
}

func (p *parser) parseElse(c *Control) {
	save := p.off
	p.skipSpaceNewlines()
	if !p.atKeyword("else") {
		p.off = save
		return
	}
	p.off += len("else")
	p.skipSpaceNewlines()
	if p.atKeyword("if") {
		c.Else = []Node{p.parseControl("if")}
		return
	}
	if !p.take("{") {
		p.fail(p.off, CodeParse, "else must be followed by if or {")
	}
	c.Else = p.parseNodes(func() bool { return p.has("}") })
	if !p.take("}") {
		p.fail(p.off, CodeParse, "unclosed else block")
	}
}

func (p *parser) parseCases() []Case {
	var cases []Case
	for {
		p.skipSpaceNewlinesAndComments()
		if p.has("}") || p.eof() {
			return cases
		}
		start := p.posAt(p.off)
		switch {
		case p.atKeyword("case"):
			p.off += len("case")
			headerStart := p.off
			for !p.eof() && p.src[p.off] != ':' {
				p.off++
			}
			header := canonExpr(p.src[headerStart:p.off])
			p.take(":")
			cases = append(cases, Case{At: start, Header: header, Body: p.parseNodes(p.caseBoundary)})
		case p.has("default:"):
			p.off += len("default")
			p.take(":")
			cases = append(cases, Case{At: start, IsDefault: true, Body: p.parseNodes(p.caseBoundary)})
		default:
			p.fail(p.off, CodeParse, "expected case or default in switch")
		}
	}
}

func (p *parser) caseBoundary() bool {
	if p.has("}") {
		return true
	}
	if !p.atLineStart() {
		return false
	}
	return p.has("case ") || p.has("case\t") || p.has("default:")
}

func (p *parser) tryLet() Node {
	save := p.off
	start := p.posAt(p.off)
	first := p.readIdent()
	if first == "" {
		return nil
	}
	names := []string{first}
	for {
		q := p.off
		p.skipSpaces()
		if !p.take(",") {
			p.off = q
			break
		}
		p.skipSpaces()
		n := p.readIdent()
		if n == "" {
			p.off = save
			return nil
		}
		names = append(names, n)
	}
	p.skipSpaces()
	if !p.take(":=") {
		p.off = save
		return nil
	}
	exprStart := p.off
	depth := 0
	for !p.eof() {
		c := p.src[p.off]
		if c == '\n' && depth == 0 {
			break
		}
		if c == '(' || c == '[' || c == '{' {
			depth++
		}
		if c == ')' || c == ']' || c == '}' {
			depth--
			if depth < 0 {
				break
			}
		}
		p.off++
	}
	return &Let{At: start, Name: strings.Join(names, ", "), Expr: canonExpr(p.src[exprStart:p.off])}
}

func (p *parser) letAt(off int) bool {
	q := &parser{src: p.src, off: off}
	id := q.readIdent()
	if id == "" {
		return false
	}
	for {
		q.skipSpaces()
		if !q.take(",") {
			break
		}
		q.skipSpaces()
		if q.readIdent() == "" {
			return false
		}
	}
	q.skipSpaces()
	return q.take(":=")
}

// scanBraces consumes { ... } and returns the inner text.
func (p *parser) scanBraces() string {
	if !p.take("{") {
		p.fail(p.off, CodeParse, "expected {")
	}
	return p.scanBraceBody()
}

// scanBraceBody returns the text up to the matching close brace and consumes
// it. Strings and comments do not nest braces.
func (p *parser) scanBraceBody() string {
	start := p.off
	depth := 0
	for !p.eof() {
		c := p.src[p.off]
		switch {
		case c == '"' || c == '\'' || c == '`':
			p.skipQuoted(c)
			continue
		case c == '/' && p.off+1 < len(p.src) && p.src[p.off+1] == '/':
			for !p.eof() && p.src[p.off] != '\n' {
				p.off++
			}
			continue
		case c == '/' && p.off+1 < len(p.src) && p.src[p.off+1] == '*':
			idx := strings.Index(p.src[p.off+2:], "*/")
			if idx < 0 {
				p.fail(p.off, CodeParse, "unclosed comment")
			}
			p.off += 2 + idx + 2
			continue
		case c == '{':
			depth++
		case c == '}':
			if depth == 0 {
				inner := p.src[start:p.off]
				p.off++
				return inner
			}
			depth--
		}
		p.off++
	}
	p.fail(p.off, CodeParse, "unclosed {")
	return ""
}

func (p *parser) scanParens() string {
	if !p.take("(") {
		p.fail(p.off, CodeParse, "expected (")
	}
	start := p.off
	depth := 1
	for !p.eof() {
		c := p.src[p.off]
		switch {
		case c == '"' || c == '\'' || c == '`':
			p.skipQuoted(c)
			continue
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				inner := p.src[start:p.off]
				p.off++
				return inner
			}
		}
		p.off++
	}
	p.fail(p.off, CodeParse, "unclosed (")
	return ""
}

func (p *parser) skipQuoted(q byte) {
	p.off++
	for !p.eof() {
		c := p.src[p.off]
		if c == '\\' && q != '`' {
			p.off += 2
			continue
		}
		p.off++
		if c == q {
			return
		}
	}
	p.fail(p.off, CodeParse, "unclosed string")
}

func (p *parser) readQuoted(q byte) string {
	if !p.take(string(q)) {
		p.fail(p.off, CodeParse, "expected "+string(q))
	}
	start := p.off
	for !p.eof() && p.src[p.off] != q {
		p.off++
	}
	if p.eof() {
		p.fail(start, CodeParse, "unclosed attribute value")
	}
	value := p.src[start:p.off]
	p.off++
	return value
}

func (p *parser) readUnquoted() string {
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if isSpaceByte(c) || c == '>' {
			break
		}
		p.off++
	}
	return p.src[start:p.off]
}

func (p *parser) readIdent() string {
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if c == '_' || isLetter(c) || (p.off > start && isDigit(c)) {
			p.off++
			continue
		}
		break
	}
	return p.src[start:p.off]
}

func (p *parser) readTagName() string {
	if p.has(":") {
		p.off++
		start := p.off
		for !p.eof() {
			c := p.src[p.off]
			if c == '_' || c == '-' || c == '.' || isLetter(c) || isDigit(c) {
				p.off++
				continue
			}
			break
		}
		return ":" + p.src[start:p.off]
	}
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if c == '_' || c == '-' || c == '.' || isLetter(c) || isDigit(c) {
			p.off++
			continue
		}
		break
	}
	return p.src[start:p.off]
}

func (p *parser) readFragmentName() string {
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if c == '_' || c == '-' || isLetter(c) || isDigit(c) {
			p.off++
			continue
		}
		break
	}
	return p.src[start:p.off]
}

func (p *parser) readAttrName() string {
	start := p.off
	for !p.eof() {
		c := p.src[p.off]
		if isSpaceByte(c) || c == '=' || c == '>' || c == '/' || c == '{' || c == '}' || c == '<' || c == '"' || c == '\'' {
			break
		}
		p.off++
	}
	return p.src[start:p.off]
}

func (p *parser) skipSpaces() {
	for !p.eof() && (p.src[p.off] == ' ' || p.src[p.off] == '\t') {
		p.off++
	}
}

func (p *parser) skipSpaceNewlines() {
	for !p.eof() && isSpaceByte(p.src[p.off]) {
		p.off++
	}
}

func (p *parser) skipSpaceNewlinesAndComments() {
	for !p.eof() {
		if isSpaceByte(p.src[p.off]) {
			p.off++
			continue
		}
		if p.has("//") {
			p.skipToLineEnd()
			continue
		}
		if p.has("/*") {
			idx := strings.Index(p.src[p.off+2:], "*/")
			if idx < 0 {
				p.off = len(p.src)
				return
			}
			p.off += 2 + idx + 2
			continue
		}
		return
	}
}

// skipLine advances past the rest of the line. It reports false when the line
// has non-space text before its end.
func (p *parser) skipLine() bool {
	ok := true
	for !p.eof() && p.src[p.off] != '\n' {
		if !isSpaceByte(p.src[p.off]) {
			ok = false
		}
		p.off++
	}
	p.take("\n")
	return ok
}

func (p *parser) skipToLineEnd() {
	for !p.eof() && p.src[p.off] != '\n' {
		p.off++
	}
	p.take("\n")
}

func (p *parser) atKeyword(kw string) bool {
	if !p.has(kw) {
		return false
	}
	next := p.off + len(kw)
	if next >= len(p.src) {
		return true
	}
	c := p.src[next]
	return !(c == '_' || isLetter(c) || isDigit(c))
}

func (p *parser) keywordAt(off int, kw string) bool {
	if !strings.HasPrefix(p.src[off:], kw) {
		return false
	}
	next := off + len(kw)
	if next >= len(p.src) {
		return true
	}
	c := p.src[next]
	return !(c == '_' || isLetter(c) || isDigit(c))
}

func (p *parser) takeKeyword(kw string) bool {
	if p.atKeyword(kw) {
		p.off += len(kw)
		return true
	}
	return false
}

func (p *parser) atLineStart() bool {
	return p.onlySpaceBeforeOnLine(p.off)
}

func (p *parser) onlySpaceBeforeOnLine(off int) bool {
	for i := off - 1; i >= 0; i-- {
		switch p.src[i] {
		case '\n':
			return true
		case ' ', '\t', '\r':
			continue
		default:
			return false
		}
	}
	return true
}

func (p *parser) lineEndsWithBrace(off int) bool {
	end := off
	for end < len(p.src) && p.src[end] != '\n' {
		end++
	}
	line := strings.TrimRight(p.src[off:end], " \t\r")
	return strings.HasSuffix(line, "{")
}

func isExportedIdent(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c == '_' || isLetter(c) || isDigit(c)) {
			return false
		}
	}
	return true
}

func isTagStartByte(c byte) bool {
	return c == ':' || isLetter(c)
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
