package lsp

import (
	"path/filepath"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// completeMarkdown completes component tags and props in a content
// Markdown file (REQ-TLS-04, REQ-CNT-03).
func completeMarkdown(doc *document, m *compiler.Model, pos position) any {
	coll := collectionFor(m, doc.Path)
	if coll == nil {
		return map[string]any{"isIncomplete": false, "items": []completionItem{}}
	}
	line := doc.line(pos.Line)
	col := pos.Character
	if col > len(line) {
		col = len(line)
	}
	before := line[:col]
	open := strings.LastIndexByte(before, '<')
	if open < 0 {
		return map[string]any{"isIncomplete": false, "items": []completionItem{}}
	}
	frag := before[open+1:]
	if strings.HasPrefix(frag, "/") {
		return map[string]any{"isIncomplete": false, "items": []completionItem{}}
	}
	items := []completionItem{}
	item := func(label string, kind int, detail string, start int) completionItem {
		return completionItem{
			Label: label, Kind: kind, Detail: detail,
			TextEdit: &textEdit{
				Range:   rng{Start: doc.positionAt(start), End: pos},
				NewText: label,
			},
		}
	}
	lineStart := doc.offsetAt(position{Line: pos.Line, Character: 0})
	if !strings.ContainsAny(frag, " \t") {
		// The tag name: offer the collection's components.
		part := frag
		prefix := ""
		if i := strings.LastIndexByte(part, '.'); i >= 0 {
			prefix = part[:i+1]
			part = part[i+1:]
		}
		for _, comp := range coll.Components {
			if part != "" && !strings.HasPrefix(comp.Name, part) {
				continue
			}
			items = append(items, item(prefix+comp.Name, kindClass, "collection component", lineStart+open+1))
		}
		return map[string]any{"isIncomplete": false, "items": items}
	}
	// An attribute inside a tag: offer the props of the component.
	fields := strings.Fields(frag)
	if len(fields) == 0 {
		return map[string]any{"isIncomplete": false, "items": items}
	}
	base := fields[0]
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[i+1:]
	}
	end := lineStart + len(before)
	wordStart := end
	for wordStart > lineStart && isWordByte(doc.Text[wordStart-1]) {
		wordStart--
	}
	for _, comp := range coll.Components {
		if comp.Name != base {
			continue
		}
		for _, prop := range comp.Props {
			attr := lowerFirst(prop.Name)
			if !strings.HasPrefix(attr, strings.ToLower(doc.Text[wordStart:end])) {
				continue
			}
			items = append(items, item(attr, kindProperty, prop.Type, wordStart))
		}
	}
	return map[string]any{"isIncomplete": false, "items": items}
}

// collectionFor returns the collection that holds a Markdown file.
func collectionFor(m *compiler.Model, path string) *compiler.CollectionRef {
	var best *compiler.CollectionRef
	for i := range m.Collections {
		dir := m.Collections[i].Dir
		sep := string(filepath.Separator)
		if path == dir || strings.HasPrefix(path, dir+sep) {
			if best == nil || len(dir) > len(best.Dir) {
				best = &m.Collections[i]
			}
		}
	}
	return best
}
