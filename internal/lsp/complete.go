package lsp

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// completionItem is one LSP completion item.
type completionItem struct {
	Label      string    `json:"label"`
	Kind       int       `json:"kind,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	Doc        string    `json:"documentation,omitempty"`
	InsertText string    `json:"insertText,omitempty"`
	TextEdit   *textEdit `json:"textEdit,omitempty"`
	SortText   string    `json:"sortText,omitempty"`
}

// textEdit is one LSP text edit.
type textEdit struct {
	Range   rng    `json:"range"`
	NewText string `json:"newText"`
}

// LSP completion item kinds.
const (
	kindFunction = 3
	kindField    = 5
	kindVariable = 6
	kindClass    = 7
	kindModule   = 9
	kindProperty = 10
	kindKeyword  = 14
)

// htmlTags are the HTML elements completion offers.
var htmlTags = []string{
	"a", "abbr", "address", "area", "article", "aside", "audio", "b", "base",
	"bdi", "bdo", "blockquote", "body", "br", "button", "canvas", "caption",
	"cite", "code", "col", "colgroup", "data", "datalist", "dd", "del",
	"details", "dfn", "dialog", "div", "dl", "dt", "em", "embed", "fieldset",
	"figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5",
	"h6", "head", "header", "hgroup", "hr", "html", "i", "iframe", "img",
	"input", "ins", "kbd", "label", "legend", "li", "link", "main", "map",
	"mark", "menu", "meta", "meter", "nav", "noscript", "object", "ol",
	"optgroup", "option", "output", "p", "picture", "pre", "progress", "q",
	"rp", "rt", "ruby", "s", "samp", "script", "search", "section", "select",
	"slot", "small", "source", "span", "strong", "style", "sub", "summary",
	"sup", "table", "tbody", "td", "template", "textarea", "tfoot", "th",
	"thead", "time", "title", "tr", "track", "u", "ul", "var", "video", "wbr",
}

// globalAttrs are the HTML attributes completion offers on any element.
var globalAttrs = []string{
	"id", "class", "style", "title", "hidden", "lang", "dir", "tabindex",
	"role", "aria-label", "aria-hidden", "aria-describedby", "aria-expanded",
	"data-", "name", "value", "type", "href", "src", "alt", "placeholder",
	"disabled", "checked", "required", "readonly", "autocomplete", "method",
	"action", "target", "rel", "for", "min", "max", "step", "multiple",
	"accept", "pattern", "maxlength", "minlength", "rows", "cols", "wrap",
}

// directives are the gx attribute directives (SDD §6.1).
var directives = []string{
	"show", "text", "bind:value", "bind:checked", "class:", "attr:",
	"on:click", "on:input", "on:change", "on:submit", "on:keydown", "on:load",
	"key", "transition",
}

// documentState is the lexical state around one cursor.
type documentState struct {
	inTag       bool
	tagName     string
	tagDone     bool // whitespace follows the tag name
	inAttr      bool
	attrName    string
	inValue     bool
	valueAttr   string
	inExpr      bool
	exprFromTag bool
	afterDollar bool
	wordStart   int
	word        string
	qualified   string
}

// scanState computes the lexical state of text[:offset].
func scanState(text string, offset int) documentState {
	st := documentState{wordStart: offset}
	i := 0
	for i < offset {
		c := text[i]
		switch {
		case st.inExpr:
			switch c {
			case '"', '\'', '`':
				q := c
				i++
				for i < offset {
					if text[i] == '\\' {
						i += 2
						continue
					}
					if text[i] == q {
						break
					}
					i++
				}
			case '}':
				st.inExpr = false
				if st.exprFromTag {
					st.exprFromTag = false
				}
			}
			i++
		case st.inValue:
			if c == '"' || c == '\'' {
				st.inValue = false
			}
			i++
		case st.inTag:
			switch c {
			case '>':
				st.inTag = false
				st.inAttr = false
				st.attrName = ""
				st.tagName = ""
				st.tagDone = false
				i++
			case '{':
				st.inExpr = true
				st.exprFromTag = true
				st.valueAttr = st.attrName
				i++
			case '"', '\'':
				st.inValue = true
				st.valueAttr = st.attrName
				i++
			case '=':
				st.inAttr = true
				i++
				if i < offset && (text[i] == '"' || text[i] == '\'') {
					i++ // consume the opening quote
					st.inValue = true
					st.valueAttr = st.attrName
				}
			case ' ', '\t', '\r', '\n':
				if st.tagName != "" {
					st.tagDone = true
				}
				i++
			default:
				start := i
				for i < offset && !strings.ContainsRune(" \t\r\n=>'\"{", rune(text[i])) {
					i++
				}
				word := text[start:i]
				if st.tagName == "" {
					st.tagName = word
				} else {
					st.attrName = word
				}
			}
		default:
			switch c {
			case '<':
				st.inTag = true
				st.tagName = ""
				st.tagDone = false
				st.attrName = ""
				i++
				continue
			case '{':
				st.inExpr = true
				i++
				continue
			}
			i++
		}
	}
	st.wordStart = offset
	for st.wordStart > 0 && isWordByte(text[st.wordStart-1]) {
		st.wordStart--
	}
	st.word = text[st.wordStart:offset]
	if st.wordStart > 0 && text[st.wordStart-1] == '.' {
		q := st.wordStart - 1
		for q > 0 && isWordByte(text[q-1]) && text[q-1] != '$' {
			q--
		}
		st.qualified = text[q : st.wordStart-1]
	}
	st.afterDollar = st.inExpr && offset > 0 && text[offset-1] == '$'
	return st
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c == '#' || c == '-' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// complete returns the completion items of one position (REQ-DEV-08).
func complete(doc *document, m *compiler.Model, pos position) any {
	offset := doc.offsetAt(pos)
	st := scanState(doc.Text, offset)
	f := fileFor(m, doc.Path)
	items := []completionItem{}
	edit := func(text string) *textEdit {
		return &textEdit{
			Range:   rng{Start: doc.positionAt(st.wordStart), End: pos},
			NewText: text,
		}
	}
	item := func(label string, kind int, detail string, text string) completionItem {
		return completionItem{Label: label, Kind: kind, Detail: detail, TextEdit: edit(text)}
	}
	switch {
	case st.inExpr:
		switch {
		case strings.HasPrefix(st.word, "$") || st.afterDollar:
			if f != nil {
				for _, s := range f.Signals {
					if strings.HasPrefix("$"+s.Name, st.word) {
						items = append(items, item("$"+s.Name, kindVariable, s.Type, "$"+s.Name))
					}
				}
			}
		case st.qualified != "":
			items = append(items, memberCompletions(m, f, st, item)...)
		default:
			items = append(items, localCompletions(m, f, st, item)...)
			items = append(items, valueCompletions(m, f, st, item)...)
		}
	case st.inTag:
		if !st.tagDone {
			if st.qualified != "" {
				items = append(items, componentCompletions(m, f, st.qualified, st.word, item)...)
			} else {
				items = append(items, componentCompletions(m, f, "", st.word, item)...)
				for _, tag := range htmlTags {
					if strings.HasPrefix(tag, st.word) {
						items = append(items, item(tag, kindKeyword, "html element", tag))
					}
				}
			}
		} else {
			if comp := resolveTag(m, f, st.tagName); comp != nil {
				for _, prop := range comp.Props {
					attr := lowerFirst(prop.Name)
					if strings.HasPrefix(attr, strings.ToLower(st.word)) {
						detail := prop.Type
						if !prop.HasDefault {
							detail += " (required)"
						}
						it := item(attr, kindProperty, detail, attr)
						it.Doc = prop.Doc
						items = append(items, it)
					}
				}
			}
			for _, attr := range globalAttrs {
				if strings.HasPrefix(attr, st.word) {
					items = append(items, item(attr, kindProperty, "attribute", attr))
				}
			}
			for _, d := range directives {
				if strings.HasPrefix(d, st.word) {
					kind := kindKeyword
					if strings.HasPrefix(d, "on:") {
						kind = kindFunction
					}
					items = append(items, item(d, kind, "gx directive", d))
				}
			}
		}
	}
	return map[string]any{"isIncomplete": false, "items": items}
}

// memberCompletions completes a qualified name (pkg.Member, p.Field).
func memberCompletions(m *compiler.Model, f *compiler.File, st documentState, item func(string, int, string, string) completionItem) []completionItem {
	var out []completionItem
	if f == nil {
		return out
	}
	if path := importPathOf(f, st.qualified); path != "" {
		for _, name := range m.Packages[path] {
			if strings.HasPrefix(name, st.word) {
				out = append(out, item(name, kindFunction, path, name))
			}
		}
		return out
	}
	if st.qualified == "p" {
		if comp := componentOfFile(m, f); comp != nil {
			for _, prop := range comp.Props {
				if strings.HasPrefix(prop.Name, st.word) {
					it := item(prop.Name, kindField, prop.Type, prop.Name)
					it.Doc = prop.Doc
					out = append(out, it)
				}
			}
		}
	}
	return out
}

// localCompletions completes local names inside a Go expression.
func localCompletions(m *compiler.Model, f *compiler.File, st documentState, item func(string, int, string, string) completionItem) []completionItem {
	var out []completionItem
	if strings.HasPrefix("p", st.word) {
		out = append(out, item("p", kindVariable, "component props", "p"))
	}
	if f != nil {
		for _, imp := range f.Imports {
			qual := compiler.ImportQualifier(imp.Raw)
			if qual != "" && strings.HasPrefix(qual, st.word) {
				out = append(out, item(qual, kindModule, compiler.ImportPath(imp.Raw), qual))
			}
		}
		for _, s := range f.Signals {
			label := "$" + s.Name
			if strings.HasPrefix(label, st.word) {
				out = append(out, item(label, kindVariable, s.Type, label))
			}
		}
		for _, el := range compiler.Fragments(f) {
			for _, a := range el.Attrs {
				if a.Kind == compiler.AttrFragment && strings.HasPrefix(a.Name, st.word) {
					out = append(out, item(a.Name, kindFunction, "fragment", a.Name))
				}
			}
		}
	}
	return out
}

// valueCompletions completes route and action literals.
func valueCompletions(m *compiler.Model, f *compiler.File, st documentState, item func(string, int, string, string) completionItem) []completionItem {
	attr := st.valueAttr
	if attr == "" {
		attr = st.attrName
	}
	var out []completionItem
	own := ""
	if comp := componentOfFile(m, f); comp != nil {
		own = comp.PkgPath
	}
	for _, r := range m.Routes {
		qual := qualifierFor(f, r.PkgPath)
		label := qual + "." + r.Name + "{}"
		if r.PkgPath == own {
			label = r.Name + "{}"
		} else if qual == "" {
			continue
		}
		if !strings.HasPrefix(r.Name, st.word) && !strings.HasPrefix(label, st.word) {
			continue
		}
		switch {
		case attr == "href" || attr == "src" || attr == "action" || attr == "formaction":
			if r.Method != "GET" && r.Method != "HEAD" {
				continue
			}
			out = append(out, item(label, kindClass, r.Pattern, label))
		case strings.HasPrefix(attr, "on:"):
			if !r.Registered {
				continue
			}
			out = append(out, item(label, kindFunction, r.Pattern, label))
		}
	}
	return out
}

// componentCompletions returns the components completion offers.
func componentCompletions(m *compiler.Model, f *compiler.File, qual, prefix string, item func(string, int, string, string) completionItem) []completionItem {
	var out []completionItem
	if f == nil {
		return out
	}
	dir := filepath.Dir(f.File)
	for _, c := range m.Components {
		if qual != "" {
			if c.PkgPath != importPathOf(f, qual) {
				continue
			}
			if strings.HasPrefix(c.Name, prefix) {
				out = append(out, item(c.Name, kindClass, c.PkgPath, c.Name))
			}
			continue
		}
		if c.Dir == dir {
			if strings.HasPrefix(c.Name, prefix) {
				out = append(out, item(c.Name, kindClass, "component", c.Name))
			}
			continue
		}
		// Components of imported packages complete as dotted tags.
		for _, imp := range f.Imports {
			if compiler.ImportPath(imp.Raw) != c.PkgPath {
				continue
			}
			label := compiler.ImportQualifier(imp.Raw) + "." + c.Name
			if strings.HasPrefix(label, prefix) {
				out = append(out, item(label, kindClass, c.PkgPath, label))
			}
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// resolveTag returns the component a tag name refers to in one file.
func resolveTag(m *compiler.Model, f *compiler.File, tag string) *compiler.ComponentRef {
	if m == nil || f == nil || tag == "" {
		return nil
	}
	qual, name := splitTag(tag)
	if qual != "" {
		path := importPathOf(f, qual)
		if path == "" {
			return nil
		}
		for i := range m.Components {
			if m.Components[i].PkgPath == path && m.Components[i].Name == name {
				return &m.Components[i]
			}
		}
		return nil
	}
	dir := filepath.Dir(f.File)
	for i := range m.Components {
		if m.Components[i].Dir == dir && m.Components[i].Name == name {
			return &m.Components[i]
		}
	}
	return nil
}

// componentOfFile returns the component a file declares.
func componentOfFile(m *compiler.Model, f *compiler.File) *compiler.ComponentRef {
	if m == nil || f == nil {
		return nil
	}
	base := strings.TrimSuffix(filepath.Base(f.File), ".gx")
	for i := range m.Components {
		if m.Components[i].Name == base && filepath.Clean(m.Components[i].File.File) == filepath.Clean(f.File) {
			return &m.Components[i]
		}
	}
	return nil
}

// splitTag splits a dotted tag name.
func splitTag(tag string) (qual, name string) {
	if i := strings.IndexByte(tag, '.'); i >= 0 {
		return tag[:i], tag[i+1:]
	}
	return "", tag
}

// importPathOf returns the import path of one qualifier in a file.
func importPathOf(f *compiler.File, qual string) string {
	if f == nil || qual == "" {
		return ""
	}
	for _, imp := range f.Imports {
		if compiler.ImportQualifier(imp.Raw) == qual {
			return compiler.ImportPath(imp.Raw)
		}
	}
	return ""
}

// qualifierFor returns the qualifier a file uses for an import path.
func qualifierFor(f *compiler.File, path string) string {
	if f == nil || path == "" {
		return ""
	}
	for _, imp := range f.Imports {
		if compiler.ImportPath(imp.Raw) == path {
			return compiler.ImportQualifier(imp.Raw)
		}
	}
	return ""
}

// lowerFirst lowercases the first byte.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
