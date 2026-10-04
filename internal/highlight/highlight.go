// Package highlight renders syntax-highlighted code frames (REQ-CNT-04).
// Chroma runs at build time; the browser gets HTML and CSS only.
package highlight

import (
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Options are the options of one highlighted code block.
type Options struct {
	Lang  string
	Title string
	// Frame is "" (none), "code" or "terminal".
	Frame string
	// Marks, Ins, Del and Words hold 1-based line numbers.
	Marks []int
	Ins   []int
	Del   []int
	Words []int
	Wrap  bool
}

// RenderCode renders one highlighted code block.
func RenderCode(lang, info, code string) string {
	opt := ParseInfo(lang, info)
	body := highlightLines(opt.Lang, code, opt)
	var b strings.Builder
	class := "gx-code"
	if opt.Frame == "terminal" {
		class += " terminal"
	}
	fmt.Fprintf(&b, `<figure class="%s" data-lang="%s">`, class, html.EscapeString(opt.Lang))
	if opt.Frame != "" || opt.Title != "" {
		b.WriteString(`<figcaption class="gx-code-bar">`)
		if opt.Frame == "terminal" {
			b.WriteString(`<i class="gx-dot"></i><i class="gx-dot"></i><i class="gx-dot"></i>`)
		}
		if opt.Title != "" {
			fmt.Fprintf(&b, `<span class="gx-code-title">%s</span>`, html.EscapeString(opt.Title))
		}
		b.WriteString(`<button class="gx-copy" type="button" data-gx-copy>Copy</button></figcaption>`)
	} else {
		b.WriteString(`<button class="gx-copy" type="button" data-gx-copy>Copy</button>`)
	}
	preClass := "gx-code-pre"
	if opt.Wrap {
		preClass += " wrap"
	}
	fmt.Fprintf(&b, `<pre class="%s"><code>%s</code></pre></figure>`, preClass, body)
	return b.String()
}

// ParseInfo parses the fence info string (REQ-CNT-04):
//
//	```go title="main.go" frame="code" {3-5} ins={7} del={9} word={2} wrap
func ParseInfo(lang, info string) Options {
	opt := Options{Lang: strings.ToLower(lang)}
	switch opt.Lang {
	case "sh", "bash", "zsh", "shell", "console", "fish":
		opt.Frame = "terminal"
	}
	rest := strings.TrimSpace(info)
	if opt.Lang != "" {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, lang))
	}
	for rest != "" {
		switch {
		case strings.HasPrefix(rest, "{"):
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return opt
			}
			opt.Marks = append(opt.Marks, ParseLines(rest[1:end])...)
			rest = strings.TrimSpace(rest[end+1:])
		case strings.HasPrefix(rest, "title="):
			value, n := parseValue(rest[len("title="):])
			opt.Title = value
			rest = strings.TrimSpace(rest[len("title=")+n:])
		case strings.HasPrefix(rest, "frame="):
			value, n := parseValue(rest[len("frame="):])
			opt.Frame = value
			rest = strings.TrimSpace(rest[len("frame=")+n:])
		case strings.HasPrefix(rest, "ins="):
			value, n := parseValue(rest[len("ins="):])
			opt.Ins = append(opt.Ins, ParseLines(value)...)
			rest = strings.TrimSpace(rest[len("ins=")+n:])
		case strings.HasPrefix(rest, "del="):
			value, n := parseValue(rest[len("del="):])
			opt.Del = append(opt.Del, ParseLines(value)...)
			rest = strings.TrimSpace(rest[len("del=")+n:])
		case strings.HasPrefix(rest, "word="):
			value, n := parseValue(rest[len("word="):])
			opt.Words = append(opt.Words, ParseLines(value)...)
			rest = strings.TrimSpace(rest[len("word=")+n:])
		case strings.HasPrefix(rest, "wrap"):
			opt.Wrap = true
			rest = strings.TrimSpace(rest[len("wrap"):])
		default:
			// An unknown word ends the options.
			return opt
		}
	}
	return opt
}

// parseValue reads a quoted or braced option value.
func parseValue(s string) (string, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	if s[0] == '"' {
		end := strings.IndexByte(s[1:], '"')
		if end < 0 {
			return s[1:], len(s)
		}
		return s[1 : 1+end], end + 2
	}
	if s[0] == '{' {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return s[1:], len(s)
		}
		return s[1:end], end + 1
	}
	i := 0
	for i < len(s) && s[i] != ' ' {
		i++
	}
	return s[:i], i
}

// ParseLines parses "1,3-5" into line numbers.
func ParseLines(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			start, err1 := strconv.Atoi(strings.TrimSpace(lo))
			end, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 == nil && err2 == nil {
				for n := start; n <= end; n++ {
					out = append(out, n)
				}
			}
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// highlightLines highlights code and wraps every line with its mark
// classes.
func highlightLines(lang, code string, opt Options) string {
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return `<span class="line">` + html.EscapeString(code) + `</span>`
	}
	var lines []string
	var line strings.Builder
	flush := func() {
		lines = append(lines, line.String())
		line.Reset()
	}
	for _, token := range iterator.Tokens() {
		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				flush()
			}
			if part == "" {
				continue
			}
			fmt.Fprintf(&line, `<span class="%s">%s</span>`, tokenClass(token.Type), html.EscapeString(part))
		}
	}
	if line.Len() > 0 {
		flush()
	}
	// The trailing newline leaves one empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, text := range lines {
		classes := lineClasses(i+1, opt)
		if classes != "" {
			lines[i] = `<span class="line ` + classes + `">` + text + `</span>`
		} else {
			lines[i] = `<span class="line">` + text + `</span>`
		}
	}
	return strings.Join(lines, "\n")
}

// lineClasses returns the mark classes of one line.
func lineClasses(n int, opt Options) string {
	var classes []string
	if contains(opt.Marks, n) {
		classes = append(classes, "mark")
	}
	if contains(opt.Ins, n) {
		classes = append(classes, "ins")
	}
	if contains(opt.Del, n) {
		classes = append(classes, "del")
	}
	if contains(opt.Words, n) {
		classes = append(classes, "word")
	}
	return strings.Join(classes, " ")
}

func contains(xs []int, n int) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

// LangForFile guesses the chroma lexer name from a file path.
func LangForFile(path string) string {
	base := strings.ToLower(path)
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	switch base {
	case "go.mod", "go.sum":
		return "go"
	case "dockerfile", "makefile":
		return "bash"
	}
	ext := base
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		ext = base[i+1:]
	}
	switch ext {
	case "go":
		return "go"
	case "ts", "tsx", "mts", "cts":
		return "typescript"
	case "js", "jsx", "mjs", "cjs":
		return "javascript"
	case "md", "markdown", "mdx":
		return "markdown"
	case "html", "htm", "gx":
		return "html"
	case "css":
		return "css"
	case "json", "jsonc", "json5":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "toml":
		return "toml"
	case "sh", "bash", "zsh":
		return "bash"
	case "sql":
		return "sql"
	case "diff", "patch":
		return "diff"
	case "xml", "svg":
		return "xml"
	case "mod":
		return "go"
	}
	return "text"
}

// tokenClass turns a chroma token type into a CSS class.
func tokenClass(t chroma.TokenType) string {
	name := t.String()
	var b strings.Builder
	b.WriteString("tok-")
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// CodeCSS returns the light and dark stylesheet of the highlighted code
// blocks (REQ-CNT-04). Token colours are CSS variables.
func CodeCSS() string {
	light := styles.Get("github")
	dark := styles.Get("github-dark")
	if light == nil {
		light = styles.Fallback
	}
	if dark == nil {
		dark = light
	}
	seen := map[string]bool{}
	var types []chroma.TokenType
	for t := range chroma.StandardTypes {
		name := tokenClassName(t)
		if seen[name] {
			continue
		}
		seen[name] = true
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return tokenClassName(types[i]) < tokenClassName(types[j]) })
	var b strings.Builder
	writeVars := func(style *chroma.Style) {
		for _, t := range types {
			entry := style.Get(t)
			if entry.Colour == 0 {
				continue
			}
			fmt.Fprintf(&b, "--gx-code-%s: %s;\n", tokenClassName(t), entry.Colour)
		}
	}
	b.WriteString(":root {\n--gx-code-bg: #ffffff;\n--gx-code-border: #e5e5e5;\n")
	writeVars(light)
	b.WriteString("}\n.dark {\n--gx-code-bg: #0d1117;\n--gx-code-border: #30363d;\n")
	writeVars(dark)
	b.WriteString("}\n@media (prefers-color-scheme: dark) {\n:root:not(.light) {\n--gx-code-bg: #0d1117;\n--gx-code-border: #30363d;\n")
	writeVars(dark)
	b.WriteString("}\n}\n")
	b.WriteString(`.gx-code { position: relative; border: 1px solid var(--gx-code-border); border-radius: 8px; overflow: hidden; background: var(--gx-code-bg); margin: 1rem 0; }
.gx-code-bar { display: flex; align-items: center; gap: .375rem; padding: .375rem .75rem; border-bottom: 1px solid var(--gx-code-border); font-size: .75rem; }
.gx-dot { display: inline-block; width: .625rem; height: .625rem; border-radius: 50%; background: var(--gx-code-border); }
.gx-code-title { margin-left: .5rem; opacity: .8; }
.gx-code-pre { margin: 0; padding: .75rem 1rem; overflow-x: auto; }
.gx-code-pre.wrap { white-space: pre-wrap; overflow-wrap: anywhere; }
.line { display: block; }
.line.mark { background: color-mix(in srgb, currentColor 8%, transparent); }
.line.ins { background: rgba(46, 160, 67, .15); }
.line.del { background: rgba(248, 81, 73, .15); }
.line.word { outline: 1px solid var(--gx-code-border); outline-offset: -1px; }
.gx-copy { position: absolute; top: .5rem; right: .5rem; font: inherit; font-size: .75rem; padding: .125rem .5rem; border: 1px solid var(--gx-code-border); border-radius: 4px; background: var(--gx-code-bg); color: inherit; cursor: pointer; }
`)
	for _, t := range types {
		entry := light.Get(t)
		name := tokenClassName(t)
		fmt.Fprintf(&b, ".tok-%s { color: var(--gx-code-%s);", name, name)
		if entry.Bold == chroma.Yes {
			b.WriteString(" font-weight: 600;")
		}
		if entry.Italic == chroma.Yes {
			b.WriteString(" font-style: italic;")
		}
		b.WriteString(" }\n")
	}
	return b.String()
}

// tokenClassName turns a token type into the variable suffix.
func tokenClassName(t chroma.TokenType) string {
	return strings.TrimPrefix(tokenClass(t), "tok-")
}
