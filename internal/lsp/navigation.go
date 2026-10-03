package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// tagHover returns the hover content for a component or HTML tag.
func (s *Server) tagHover(doc *document, m *compiler.Model, pos position) (string, rng, bool) {
	name, r, ok := tagAt(doc, pos)
	if !ok {
		return "", rng{}, false
	}
	f := fileFor(m, doc.Path)
	if comp := resolveTag(m, f, name); comp != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "**%s** — component `%s`\n\n", comp.Name, comp.PkgPath)
		if len(comp.Props) > 0 {
			b.WriteString("```go\ntype " + comp.Name + "Props struct {\n")
			for _, p := range comp.Props {
				def := ""
				if p.HasDefault {
					def = " = " + p.Default
				}
				fmt.Fprintf(&b, "\t%s %s%s\n", p.Name, p.Type, def)
			}
			b.WriteString("}\n```")
		}
		return b.String(), r, true
	}
	for _, tag := range htmlTags {
		if tag == name {
			return "HTML element `<" + name + ">`", r, true
		}
	}
	return "", rng{}, false
}

// tagDefinition returns the definition location of a component tag.
func (s *Server) tagDefinition(doc *document, m *compiler.Model, pos position) *location {
	name, _, ok := tagAt(doc, pos)
	if !ok {
		return nil
	}
	f := fileFor(m, doc.Path)
	if comp := resolveTag(m, f, name); comp != nil {
		return &location{URI: pathToURI(comp.File.File), Range: rng{Start: position{}, End: position{}}}
	}
	// An island .ts file next to the .gx file is the definition of an
	// island tag (REQ-DEV-08).
	_, tagName := splitTag(name)
	if f != nil {
		ts := filepath.Join(filepath.Dir(f.File), tagName+".ts")
		if _, err := os.Stat(ts); err == nil {
			return &location{URI: pathToURI(ts), Range: rng{Start: position{}, End: position{}}}
		}
	}
	return nil
}

// renameAt returns the workspace edits of a rename (REQ-DEV-08).
func renameAt(doc *document, m *compiler.Model, pos position, newName string) map[string][]map[string]any {
	if newName == "" || strings.ContainsAny(newName, " \t\n.$#") {
		return nil
	}
	f := fileFor(m, doc.Path)
	if f == nil {
		return nil
	}
	if name := signalAt(doc.Text, doc.offsetAt(pos)); name != "" && signalNamed(f, name) {
		return signalRename(doc, f, name, newName)
	}
	if name, ok := attrFragmentAt(f, pos); ok {
		return fragmentRename(m, f, name, newName, doc)
	}
	if name := fragmentCallAt(f, doc, pos); name != "" {
		return fragmentRename(m, f, name, newName, doc)
	}
	if prop, comp := propAt(m, doc, f, pos); comp != nil && prop != "" {
		return propRename(m, comp, prop, newName)
	}
	return nil
}

// signalAt returns the signal name of a $name occurrence at an offset.
func signalAt(text string, offset int) string {
	start := offset
	for start > 0 && isWordByte(text[start-1]) && text[start-1] != '$' {
		start--
	}
	if start == 0 || text[start-1] != '$' {
		return ""
	}
	end := offset
	for end < len(text) && isWordByte(text[end]) && text[end] != '$' {
		end++
	}
	return text[start:end]
}

// signalNamed reports whether a file declares signal name.
func signalNamed(f *compiler.File, name string) bool {
	for _, s := range f.Signals {
		if s.Name == name {
			return true
		}
	}
	return false
}

// signalRename renames one signal in its file.
func signalRename(doc *document, f *compiler.File, name, newName string) map[string][]map[string]any {
	edits := textEdits(doc.Text, "$"+name, "$"+newName, wordBoundary)
	for _, s := range f.Signals {
		if s.Name != name {
			continue
		}
		edits = append(edits, positionEdit(doc, s.At.Line, s.At.Col, len(name), newName))
	}
	return map[string][]map[string]any{pathToURI(doc.Path): sortEdits(edits)}
}

// attrFragmentAt returns the fragment name of the #name attribute at a
// position.
func attrFragmentAt(f *compiler.File, pos position) (string, bool) {
	line := pos.Line + 1
	for _, el := range compiler.Fragments(f) {
		for _, a := range el.Attrs {
			if a.Kind != compiler.AttrFragment {
				continue
			}
			if a.At.Line == line && pos.Character+1 >= a.At.Col && pos.Character+1 <= a.At.Col+len(a.Name)+1 {
				return a.Name, true
			}
		}
	}
	return "", false
}

// fragmentCallAt returns the fragment name of a p.Name reference at a
// position.
func fragmentCallAt(f *compiler.File, doc *document, pos position) string {
	for _, el := range compiler.Fragments(f) {
		for _, a := range el.Attrs {
			if a.Kind != compiler.AttrFragment {
				continue
			}
			for _, idx := range allIndexes(doc.Text, "p."+a.Name) {
				line, col := positionAt(doc.Text, idx+2)
				if line == pos.Line && pos.Character >= col && pos.Character <= col+len(a.Name) {
					return a.Name
				}
			}
		}
	}
	return ""
}

// fragmentRename renames one fragment of a component in every file that
// names it.
func fragmentRename(m *compiler.Model, f *compiler.File, name, newName string, doc *document) map[string][]map[string]any {
	changes := map[string][]map[string]any{}
	comp := componentOfFile(m, f)
	for _, symFile := range m.Files {
		d := doc
		if symFile.File != doc.Path {
			d = &document{Path: symFile.File, Text: readText(symFile.File)}
			if d.Text == "" {
				continue
			}
		}
		var edits []map[string]any
		edits = append(edits, textEdits(d.Text, "#"+name, "#"+newName, prefixBoundary("#"))...)
		edits = append(edits, textEdits(d.Text, "p."+name, "p."+newName, wordBoundary)...)
		if comp != nil {
			oldFn := comp.Name + upperFirst(name)
			newFn := comp.Name + upperFirst(newName)
			edits = append(edits, textEdits(d.Text, oldFn, newFn, wordBoundary)...)
		}
		if len(edits) > 0 {
			changes[pathToURI(symFile.File)] = sortEdits(edits)
		}
	}
	return changes
}

// propAt returns the prop name and component at a position.
func propAt(m *compiler.Model, doc *document, f *compiler.File, pos position) (string, *compiler.ComponentRef) {
	comp := componentOfFile(m, f)
	if comp == nil {
		return "", nil
	}
	for _, prop := range f.Props {
		for _, idx := range allIndexes(doc.Text, "p."+prop.Name) {
			line, col := positionAt(doc.Text, idx+2)
			if line == pos.Line && pos.Character >= col && pos.Character <= col+len(prop.Name) {
				return prop.Name, comp
			}
		}
		attr := lowerFirst(prop.Name)
		for _, idx := range allIndexes(doc.Text, attr+"=") {
			line, col := positionAt(doc.Text, idx)
			if line != pos.Line || pos.Character < col || pos.Character > col+len(attr) {
				continue
			}
			if inComponentTag(doc.Text, idx, comp.Name) {
				return prop.Name, comp
			}
		}
	}
	return "", nil
}

// propRename renames one prop in its component and in every caller.
func propRename(m *compiler.Model, comp *compiler.ComponentRef, prop, newName string) map[string][]map[string]any {
	changes := map[string][]map[string]any{}
	attr := lowerFirst(prop)
	newAttr := lowerFirst(newName)
	for _, f := range m.Files {
		d := &document{Path: f.File, Text: readText(f.File)}
		if d.Text == "" {
			continue
		}
		var edits []map[string]any
		if filepath.Clean(f.File) == filepath.Clean(comp.File.File) {
			for _, p := range f.Props {
				if p.Name == prop {
					edits = append(edits, positionEdit(d, p.At.Line, p.At.Col, len(prop), newName))
				}
			}
			edits = append(edits, textEdits(d.Text, "p."+prop, "p."+newName, wordBoundary)...)
		}
		for _, idx := range allIndexes(d.Text, attr+"=") {
			if inComponentTag(d.Text, idx, comp.Name) {
				edits = append(edits, replaceAt(d, idx, len(attr), newAttr))
			}
		}
		if len(edits) > 0 {
			changes[pathToURI(f.File)] = sortEdits(edits)
		}
	}
	return changes
}

// inComponentTag reports whether the attribute at an offset belongs to a
// tag whose name matches comp.
func inComponentTag(text string, attrOffset int, comp string) bool {
	start := strings.LastIndexByte(text[:attrOffset], '<')
	if start < 0 {
		return false
	}
	rest := text[start+1:]
	end := strings.IndexAny(rest, " \t\r\n>")
	if end < 0 {
		return false
	}
	_, name := splitTag(rest[:end])
	return name == comp
}

// textEdits finds every occurrence of old in text and returns a replacement
// edit when boundary accepts it.
func textEdits(text, old, newText string, boundary func(string, int, int) bool) []map[string]any {
	d := &document{Path: "", Text: text}
	var out []map[string]any
	for _, idx := range allIndexes(text, old) {
		if boundary != nil && !boundary(text, idx, len(old)) {
			continue
		}
		out = append(out, replaceAt(d, idx, len(old), newText))
	}
	return out
}

// wordBoundary rejects a match whose neighbors are word bytes.
func wordBoundary(text string, idx, n int) bool {
	if idx > 0 && isWordByte(text[idx-1]) {
		return false
	}
	end := idx + n
	return end >= len(text) || !isWordByte(text[end])
}

// prefixBoundary rejects a match that is part of a longer name (for #frag).
func prefixBoundary(prefix string) func(string, int, int) bool {
	return func(text string, idx, n int) bool {
		return wordBoundary(text, idx, n)
	}
}

func allIndexes(text, needle string) []int {
	var out []int
	for start := 0; ; {
		i := strings.Index(text[start:], needle)
		if i < 0 {
			return out
		}
		out = append(out, start+i)
		start += i + len(needle)
	}
}

func positionAt(text string, offset int) (int, int) {
	d := &document{Text: text}
	pos := d.positionAt(offset)
	return pos.Line, pos.Character
}

func lineOffset(text string, line int) int {
	offset := 0
	for i := 0; i < line; i++ {
		idx := strings.IndexByte(text[offset:], '\n')
		if idx < 0 {
			return len(text)
		}
		offset += idx + 1
	}
	return offset
}

// positionEdit makes an edit from a 1-based compiler position.
func positionEdit(d *document, line, col, n int, newText string) map[string]any {
	base := lineOffset(d.Text, line-1) + col - 1
	return replaceAt(d, base, n, newText)
}

func replaceAt(d *document, offset, n int, newText string) map[string]any {
	if offset < 0 || offset > len(d.Text) {
		return nil
	}
	end := offset + n
	if end > len(d.Text) {
		end = len(d.Text)
	}
	return map[string]any{
		"range":   rng{Start: d.positionAt(offset), End: d.positionAt(end)},
		"newText": newText,
	}
}

func sortEdits(edits []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(edits))
	for _, e := range edits {
		if e != nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri := out[i]["range"].(rng)
		rj := out[j]["range"].(rng)
		if ri.Start.Line != rj.Start.Line {
			return ri.Start.Line < rj.Start.Line
		}
		return ri.Start.Character < rj.Start.Character
	})
	return out
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func readText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
