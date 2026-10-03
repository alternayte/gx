package lsp

import (
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// semantic token legend indexes.
const (
	tokComponent = 6
	tokProp      = 7
	tokSignal    = 8
	tokFragment  = 11
)

// semToken is one semantic token before encoding.
type semToken struct {
	line, col, length, typ int
}

// semanticTokenData returns the encoded full-document tokens (REQ-TLS-04).
func semanticTokenData(m *compiler.Model, doc *document) []uint32 {
	f := fileFor(m, doc.Path)
	if f == nil {
		return []uint32{}
	}
	var toks []semToken
	at := func(line, col int) int { return lineOffset(doc.Text, line-1) + col - 1 }
	add := func(offset, length, typ int) {
		if length <= 0 || offset < 0 || offset > len(doc.Text) {
			return
		}
		pos := doc.positionAt(offset)
		toks = append(toks, semToken{line: pos.Line, col: pos.Character, length: utf16Len(doc.Text[offset:min(len(doc.Text), offset+length)]), typ: typ})
	}
	var walk func(nodes []compiler.Node)
	walk = func(nodes []compiler.Node) {
		for _, node := range nodes {
			switch t := node.(type) {
			case *compiler.Element:
				if resolveTag(m, f, t.Name) != nil {
					add(at(t.At.Line, t.At.Col)+1, len(t.Name), tokComponent)
				}
				for _, a := range t.Attrs {
					switch a.Kind {
					case compiler.AttrFragment:
						add(at(a.At.Line, a.At.Col), len(a.Name)+1, tokFragment)
					case compiler.AttrExpr, compiler.AttrString, compiler.AttrBool:
						if resolveTag(m, f, t.Name) != nil {
							add(at(a.At.Line, a.At.Col), len(a.Name), tokProp)
						}
					}
					if a.Kind == compiler.AttrExpr {
						base := at(a.ValueAt.Line, a.ValueAt.Col)
						for _, s := range f.Signals {
							for _, idx := range allIndexes(a.Value, "$"+s.Name) {
								add(base+idx, len(s.Name)+1, tokSignal)
							}
						}
					}
				}
				walk(t.Children)
			case *compiler.Control:
				walk(t.Body)
				walk(t.Else)
				for _, c := range t.Cases {
					walk(c.Body)
				}
			}
		}
	}
	walk(f.Body)
	for _, s := range f.Signals {
		add(at(s.At.Line, s.At.Col), len(s.Name), tokSignal)
	}
	sort.Slice(toks, func(i, j int) bool {
		if toks[i].line != toks[j].line {
			return toks[i].line < toks[j].line
		}
		return toks[i].col < toks[j].col
	})
	data := make([]uint32, 0, len(toks)*5)
	prevLine, prevCol := 0, 0
	lastEnd := -1
	for _, t := range toks {
		if t.line == prevLine && t.col < lastEnd {
			continue // never emit overlapping tokens
		}
		deltaLine := t.line - prevLine
		deltaCol := t.col - prevCol
		if deltaLine != 0 {
			deltaCol = t.col
		}
		data = append(data, uint32(deltaLine), uint32(deltaCol), uint32(t.length), uint32(t.typ), 0)
		prevLine, prevCol = t.line, t.col
		lastEnd = t.col + t.length
	}
	return data
}

// fragmentHints returns inlay hints for the generated fragment signatures
// (REQ-TLS-04).
func fragmentHints(m *compiler.Model, doc *document) []map[string]any {
	f := fileFor(m, doc.Path)
	if f == nil {
		return []map[string]any{}
	}
	comp := componentOfFile(m, f)
	if comp == nil {
		return []map[string]any{}
	}
	var out []map[string]any
	for _, el := range compiler.Fragments(f) {
		for _, a := range el.Attrs {
			if a.Kind != compiler.AttrFragment {
				continue
			}
			params := strings.TrimSpace(a.Value)
			sig := comp.Name + upperFirst(a.Name) + "(" + params + ") gx.Node"
			offset := lineOffset(doc.Text, a.At.Line-1) + a.At.Col - 1 + 1 + len(a.Name)
			if a.Value != "" {
				offset += len(a.Value) + 2
			}
			out = append(out, map[string]any{
				"position":    doc.positionAt(offset),
				"label":       " : " + sig,
				"kind":        1, // type
				"paddingLeft": true,
			})
		}
	}
	return out
}
