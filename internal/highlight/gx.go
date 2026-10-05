package highlight

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// gxLexer is the chroma lexer of .gx source (REQ-CNT-04). It follows
// docs/grammar.md: the header, the props and signals blocks, tags,
// attributes, directives, fragments, Go expressions, control lines and
// comments. It never fails: text it does not know stays plain text, so a
// part of a file highlights too.
type gxLexer struct{}

func (gxLexer) Config() *chroma.Config {
	return &chroma.Config{Name: "Gx", Aliases: []string{"gx"}, Filenames: []string{"*.gx"}}
}

func (gxLexer) Tokenise(_ *chroma.TokeniseOptions, text string) (chroma.Iterator, error) {
	s := &gxScanner{src: text}
	s.run()
	return chroma.Literator(s.out...), nil
}

func (l gxLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer     { return l }
func (l gxLexer) SetAnalyser(func(text string) float32) chroma.Lexer { return l }
func (gxLexer) AnalyseText(string) float32                           { return 0 }

// lexerFor returns the lexer of a language name. The .gx lexer is not in
// the chroma registry.
func lexerFor(lang string) chroma.Lexer {
	if lang == "gx" {
		return gxLexer{}
	}
	if lexer := lexers.Get(lang); lexer != nil {
		return lexer
	}
	return lexers.Fallback
}

var (
	// gxControlLine matches the lines that are Go: a block opener, a block
	// end with an optional else, a switch clause and a statement.
	gxControlLine = regexp.MustCompile(`^(?:(?:if|for|switch)\b.*\{|\}(?:\s*else\b.*\{)?|case\b.*:|default\s*:|[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*\s*:=\s*\S.*)$`)
	gxBlockStart  = regexp.MustCompile(`^(props|signals)(\s*)\{`)
	gxDirective   = regexp.MustCompile(`^(?:show|text|key|transition|let|(?:bind|class|attr|on):.*)$`)
	gxSignal      = regexp.MustCompile(`\$[A-Za-z_]\w*`)
	gxEntity      = regexp.MustCompile(`^&(?:#[xX][0-9a-fA-F]{1,6}|#[0-9]{1,7}|[A-Za-z][A-Za-z0-9]{0,30});`)
	gxIdent       = regexp.MustCompile(`^[A-Za-z_]\w*`)
)

// gxAttribute is the token type of an attribute name and of a prop name.
// The code themes give a variable a colour of its own and an attribute the
// text colour, so the lexer uses the variable type.
const gxAttribute = chroma.NameVariable

// gxScanner turns .gx source into tokens. The values of the tokens, joined,
// are the source.
type gxScanner struct {
	src string
	pos int
	out []chroma.Token
	// markup is true after the first tag or control line: the header is
	// over.
	markup bool
}

func (s *gxScanner) emit(t chroma.TokenType, v string) {
	if v != "" {
		s.out = append(s.out, chroma.Token{Type: t, Value: v})
	}
}

func (s *gxScanner) has(prefix string) bool { return strings.HasPrefix(s.src[s.pos:], prefix) }

// take emits the next n bytes as one token.
func (s *gxScanner) take(t chroma.TokenType, n int) {
	s.emit(t, s.src[s.pos:s.pos+n])
	s.pos += n
}

// space emits the run of spaces, tabs and line ends at the current
// position.
func (s *gxScanner) space() {
	n := 0
	for s.pos+n < len(s.src) && strings.IndexByte(" \t\r\n", s.src[s.pos+n]) >= 0 {
		n++
	}
	s.take(chroma.Text, n)
}

// sub emits text in another language. A lexer that adds a line end to the
// text loses it again.
func (s *gxScanner) sub(lang, text string) {
	if text == "" {
		return
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		s.emit(chroma.Text, text)
		return
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		s.emit(chroma.Text, text)
		return
	}
	left := len(text)
	for _, tok := range it.Tokens() {
		if len(tok.Value) > left {
			tok.Value = tok.Value[:left]
		}
		left -= len(tok.Value)
		s.emit(tok.Type, tok.Value)
	}
	s.emit(chroma.Text, text[len(text)-left:])
}

// goCode emits Go code. A $Name is a signal, which Go does not have.
func (s *gxScanner) goCode(text string) {
	for text != "" {
		loc := gxSignal.FindStringIndex(text)
		if loc == nil {
			s.sub("go", text)
			return
		}
		s.sub("go", text[:loc[0]])
		s.emit(chroma.NameEntity, text[loc[0]:loc[1]])
		text = text[loc[1]:]
	}
}

func (s *gxScanner) run() {
	for s.pos < len(s.src) {
		if (s.pos == 0 || s.src[s.pos-1] == '\n') && s.line() {
			continue
		}
		s.content()
	}
}

// line handles a line that is Go as a whole: a header line, a props or
// signals block, a control line or a statement. It reports false for a
// line of markup.
func (s *gxScanner) line() bool {
	end := strings.IndexByte(s.src[s.pos:], '\n')
	if end < 0 {
		end = len(s.src) - s.pos
	}
	line := s.src[s.pos : s.pos+end]
	trimmed := strings.TrimSpace(line)
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	if !s.markup {
		switch {
		case strings.HasPrefix(trimmed, "//"):
			s.take(chroma.Text, indent)
			s.take(chroma.CommentSingle, end-indent)
			return true
		case strings.HasPrefix(trimmed, "/*"):
			s.take(chroma.Text, indent)
			n := len(s.src) - s.pos
			if stop := strings.Index(s.src[s.pos:], "*/"); stop >= 0 {
				n = stop + 2
			}
			s.take(chroma.CommentMultiline, n)
			return true
		case strings.HasPrefix(trimmed, "package ") || trimmed == "package":
			s.goLine(end)
			return true
		case strings.HasPrefix(trimmed, "import"):
			n := end
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, "import")), "(") {
				if stop := strings.IndexByte(s.src[s.pos:], ')'); stop >= 0 {
					n = stop + 1
				}
			}
			s.goLine(n)
			return true
		case gxBlockStart.MatchString(trimmed):
			s.take(chroma.Text, indent)
			s.block()
			return true
		}
	}
	if !gxControlLine.MatchString(trimmed) {
		return false
	}
	s.markup = true
	s.goLine(end)
	return true
}

// goLine emits the next n bytes as Go, with the leading space as text.
func (s *gxScanner) goLine(n int) {
	text := s.src[s.pos : s.pos+n]
	indent := len(text) - len(strings.TrimLeft(text, " \t"))
	s.take(chroma.Text, indent)
	s.goCode(text[indent:])
	s.pos += n - indent
}

// block emits a props or signals block: the keyword, then one field or one
// comment per line.
func (s *gxScanner) block() {
	m := gxBlockStart.FindStringSubmatch(s.src[s.pos:])
	s.take(chroma.KeywordDeclaration, len(m[1]))
	s.take(chroma.Text, len(m[2]))
	end := s.match(s.pos, '{', '}')
	s.take(chroma.Punctuation, 1)
	stop := end
	if stop < 0 {
		stop = len(s.src)
	}
	depth := 0
	for s.pos < stop {
		s.space()
		if s.pos >= stop {
			break
		}
		n := strings.IndexByte(s.src[s.pos:stop], '\n')
		if n < 0 {
			n = stop - s.pos
		}
		line := s.src[s.pos : s.pos+n]
		switch name := gxIdent.FindString(line); {
		case strings.HasPrefix(line, "//"):
			s.take(chroma.CommentSingle, n)
			continue
		case depth == 0 && name != "":
			// The prop name is the attribute name of the tag.
			s.take(gxAttribute, len(name))
			s.goCode(line[len(name):])
			s.pos += n - len(name)
		default:
			s.goCode(line)
			s.pos += n
		}
		depth += strings.Count(line, "{") + strings.Count(line, "(") + strings.Count(line, "[")
		depth -= strings.Count(line, "}") + strings.Count(line, ")") + strings.Count(line, "]")
	}
	if end >= 0 {
		s.take(chroma.Punctuation, 1)
	}
}

// match returns the offset of the bracket that closes the one at start, or
// -1. A string and a block comment hold any byte.
func (s *gxScanner) match(start int, open, shut byte) int {
	depth := 0
	for i := start; i < len(s.src); i++ {
		switch c := s.src[i]; c {
		case open:
			depth++
		case shut:
			depth--
			if depth == 0 {
				return i
			}
		case '"', '\'', '`':
			for i++; i < len(s.src) && s.src[i] != c; i++ {
				if s.src[i] == '\n' && c != '`' {
					break
				}
				if s.src[i] == '\\' && c != '`' {
					i++
				}
			}
		case '/':
			if strings.HasPrefix(s.src[i:], "/*") {
				stop := strings.Index(s.src[i+2:], "*/")
				if stop < 0 {
					return -1
				}
				i += 2 + stop + 1
			}
		}
	}
	return -1
}

// content emits one piece of markup at the current position.
func (s *gxScanner) content() {
	c := s.src[s.pos]
	switch {
	case s.has("<!--"):
		s.comment("-->")
	case s.has("{/*"):
		s.comment("*/}")
	case c == '{':
		s.expr()
	case s.has("</"):
		s.take(chroma.Punctuation, 2)
		s.tagName()
		s.space()
		if s.has(">") {
			s.take(chroma.Punctuation, 1)
		}
	case c == '<' && s.pos+1 < len(s.src) && (isNameStart(s.src[s.pos+1]) || s.src[s.pos+1] == ':'):
		s.tag()
	case c == '&' && gxEntity.MatchString(s.src[s.pos:]):
		s.take(chroma.NameEntity, len(gxEntity.FindString(s.src[s.pos:])))
	case c == '\n':
		s.take(chroma.Text, 1)
	default:
		n := 1
		for s.pos+n < len(s.src) && strings.IndexByte("<{&\n", s.src[s.pos+n]) < 0 {
			n++
		}
		s.take(chroma.Text, n)
	}
}

// comment emits a comment that ends at stop, or at the end of the text.
func (s *gxScanner) comment(stop string) {
	n := len(s.src) - s.pos
	if end := strings.Index(s.src[s.pos:], stop); end >= 0 {
		n = end + len(stop)
	}
	s.take(chroma.CommentMultiline, n)
}

// expr emits {expr}: the braces, and Go between them.
func (s *gxScanner) expr() {
	end := s.match(s.pos, '{', '}')
	if end < 0 {
		s.take(chroma.Text, len(s.src)-s.pos)
		return
	}
	s.take(chroma.Punctuation, 1)
	if s.has("...") {
		s.take(chroma.Operator, 3)
	}
	s.goCode(s.src[s.pos:end])
	s.pos = end
	s.take(chroma.Punctuation, 1)
}

// tagName emits a tag name and returns it. A component is a function, a
// slot is a label, and any other name is an HTML tag.
func (s *gxScanner) tagName() string {
	n := 0
	for s.pos+n < len(s.src) && strings.IndexByte(" \t\r\n/>", s.src[s.pos+n]) < 0 {
		n++
	}
	name := s.src[s.pos : s.pos+n]
	local := name[strings.LastIndexByte(name, '.')+1:]
	switch {
	case strings.HasPrefix(name, ":"):
		s.take(chroma.NameLabel, n)
	case local != "" && local[0] >= 'A' && local[0] <= 'Z':
		s.take(chroma.NameFunction, n)
	default:
		s.take(chroma.NameTag, n)
	}
	return name
}

// tag emits a start tag with its attributes, and the raw body of a script
// or style element.
func (s *gxScanner) tag() {
	s.markup = true
	start := s.pos
	s.take(chroma.Punctuation, 1)
	name := s.tagName()
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case strings.IndexByte(" \t\r\n", c) >= 0:
			s.space()
		case s.has("/>"):
			s.take(chroma.Punctuation, 2)
			return
		case c == '>':
			s.take(chroma.Punctuation, 1)
			if name == "script" || name == "style" {
				s.raw(name, s.src[start:s.pos])
			}
			return
		case c == '{':
			s.expr()
		case c == '#':
			n := 1 + len(gxIdent.FindString(s.src[s.pos+1:]))
			s.take(chroma.NameFunction, n)
			if s.has("(") {
				if end := s.match(s.pos, '(', ')'); end >= 0 {
					s.take(chroma.Punctuation, 1)
					s.goCode(s.src[s.pos:end])
					s.pos = end
					s.take(chroma.Punctuation, 1)
				}
			}
		case c == '=':
			s.take(chroma.Operator, 1)
			s.attrValue()
		case c == '"' || c == '\'' || c == '<':
			s.take(chroma.Text, 1)
		default:
			n := 0
			for s.pos+n < len(s.src) && strings.IndexByte(" \t\r\n=>{\"'", s.src[s.pos+n]) < 0 && !strings.HasPrefix(s.src[s.pos+n:], "/>") {
				n++
			}
			if n == 0 {
				s.take(chroma.Text, 1)
				continue
			}
			if gxDirective.MatchString(s.src[s.pos : s.pos+n]) {
				s.take(chroma.Keyword, n)
			} else {
				s.take(gxAttribute, n)
			}
		}
	}
}

// attrValue emits the value after the = of an attribute: a quoted string,
// an expression or a bare word.
func (s *gxScanner) attrValue() {
	if s.pos >= len(s.src) {
		return
	}
	switch c := s.src[s.pos]; c {
	case '"', '\'':
		n := len(s.src) - s.pos
		if end := strings.IndexByte(s.src[s.pos+1:], c); end >= 0 {
			n = end + 2
		}
		s.take(chroma.LiteralString, n)
	case '{':
		s.expr()
	default:
		n := 0
		for s.pos+n < len(s.src) && strings.IndexByte(" \t\r\n>", s.src[s.pos+n]) < 0 {
			n++
		}
		s.take(chroma.LiteralString, n)
	}
}

// raw emits the body of a script or style element in its own language.
func (s *gxScanner) raw(name, open string) {
	n := len(s.src) - s.pos
	if end := strings.Index(s.src[s.pos:], "</"+name); end >= 0 {
		n = end
	}
	lang := "css"
	if name == "script" {
		lang = "javascript"
		if strings.Contains(open, `lang="ts"`) || strings.Contains(open, `lang='ts'`) {
			lang = "typescript"
		}
	}
	s.sub(lang, s.src[s.pos:s.pos+n])
	s.pos += n
}

func isNameStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
