package lsp

import (
	"encoding/json"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// semanticTokenTypes is the semantic token legend (REQ-TLS-04). Custom
// types carry the gx model, standard types keep editors that only know the
// standard set working.
var semanticTokenTypes = []string{
	"namespace", "class", "property", "variable", "function", "macro",
	"gxComponent", "gxProp", "gxSignal", "gxRoute", "gxAction", "gxFragment",
}

// textDocumentPosition is the shared params of position requests.
type textDocumentPosition struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position position `json:"position"`
}

// completion returns the completions of one position (REQ-DEV-08).
func (s *Server) completion(params json.RawMessage) any {
	var p textDocumentPosition
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	if strings.HasSuffix(doc.Path, ".md") {
		return completeMarkdown(doc, m, p.Position)
	}
	return complete(doc, m, p.Position)
}

// hover returns the type of the symbol at one position (REQ-DEV-08).
func (s *Server) hover(params json.RawMessage) any {
	var p textDocumentPosition
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	if sym := symbolAt(m, doc.Path, p.Position); sym != nil {
		return map[string]any{
			"contents": map[string]any{
				"kind":  "markdown",
				"value": "```go\n" + sym.Text + " " + sym.Type + "\n```",
			},
			"range": rng{
				Start: position{Line: sym.Line - 1, Character: sym.Col - 1},
				End:   position{Line: sym.Line - 1, Character: sym.EndCol - 1},
			},
		}
	}
	if content, r, ok := s.tagHover(doc, m, p.Position); ok {
		return map[string]any{"contents": map[string]any{"kind": "markdown", "value": content}, "range": r}
	}
	return nil
}

// definition returns the definition location of the symbol at one position
// (REQ-DEV-08).
func (s *Server) definition(params json.RawMessage) any {
	var p textDocumentPosition
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	if sym := symbolAt(m, doc.Path, p.Position); sym != nil && sym.ObjectFile != "" {
		return []location{{
			URI: pathToURI(sym.ObjectFile),
			Range: rng{
				Start: position{Line: max(0, sym.ObjectLine-1), Character: max(0, sym.ObjectCol-1)},
				End:   position{Line: max(0, sym.ObjectLine-1), Character: max(0, sym.ObjectCol-1)},
			},
		}}
	}
	if loc := s.tagDefinition(doc, m, p.Position); loc != nil {
		return []location{*loc}
	}
	return nil
}

// formatting returns the whole-document format edit (REQ-DEV-08).
func (s *Server) formatting(params json.RawMessage) any {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	out, diags := compiler.FormatSource(doc.Path, []byte(doc.Text))
	if len(diags) > 0 {
		return nil
	}
	if string(out) == doc.Text {
		return []any{}
	}
	return []map[string]any{{
		"range":   rng{Start: position{}, End: doc.positionAt(len(doc.Text))},
		"newText": string(out),
	}}
}

// rename returns a workspace edit for props, fragments and signals
// (REQ-DEV-08).
func (s *Server) rename(params json.RawMessage) any {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position position `json:"position"`
		NewName  string   `json:"newName"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	edits := renameAt(doc, m, p.Position, p.NewName)
	if len(edits) == 0 {
		return nil
	}
	changes := map[string][]map[string]any{}
	for uri, list := range edits {
		changes[uri] = list
	}
	return map[string]any{"changes": changes}
}

// codeAction returns the quick fixes and organize-imports actions
// (REQ-DEV-08).
func (s *Server) codeAction(params json.RawMessage) any {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range   rng `json:"range"`
		Context struct {
			Diagnostics []lspDiagnostic `json:"diagnostics"`
		} `json:"context"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	return codeActions(doc, m, p.Range, p.Context.Diagnostics)
}

// semanticTokens returns the full-document semantic tokens (REQ-TLS-04).
func (s *Server) semanticTokens(params json.RawMessage) any {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	return map[string]any{"data": semanticTokenData(m, doc)}
}

// inlayHints returns the generated fragment signatures (REQ-TLS-04).
func (s *Server) inlayHints(params json.RawMessage) any {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil
	}
	doc := s.doc(p.TextDocument.URI)
	if doc == nil {
		return nil
	}
	m, _ := s.session.Model(s.root)
	if m == nil {
		return nil
	}
	return fragmentHints(m, doc)
}

// fileFor returns the parsed compiler file of an open document.
func fileFor(m *compiler.Model, path string) *compiler.File {
	for _, f := range m.Files {
		if filepath.Clean(f.File) == filepath.Clean(path) {
			return f
		}
	}
	return nil
}

// symbolsIn returns the symbols of one file.
func symbolsIn(m *compiler.Model, path string) []compiler.Symbol {
	var out []compiler.Symbol
	for _, sym := range m.Symbols {
		if filepath.Clean(sym.File) == filepath.Clean(path) {
			out = append(out, sym)
		}
	}
	return out
}

// symbolAt returns the innermost symbol at one position.
func symbolAt(m *compiler.Model, path string, pos position) *compiler.Symbol {
	line := pos.Line + 1
	col := pos.Character + 1
	var best *compiler.Symbol
	for i := range m.Symbols {
		sym := &m.Symbols[i]
		if filepath.Clean(sym.File) != filepath.Clean(path) || sym.Line != line {
			continue
		}
		if col >= sym.Col && col <= sym.EndCol {
			if best == nil || sym.Col >= best.Col {
				best = sym
			}
		}
	}
	return best
}

// tagAt returns the component or HTML tag name around one position.
func tagAt(doc *document, pos position) (name string, r rng, ok bool) {
	offset := doc.offsetAt(pos)
	text := doc.Text
	start := offset
	for start > 0 && text[start-1] != '<' && text[start-1] != '>' && text[start-1] != '\n' {
		start--
	}
	if start == 0 || text[start-1] != '<' {
		return "", rng{}, false
	}
	end := start
	for end < len(text) && !strings.ContainsRune(" \t\r\n>/", rune(text[end])) {
		end++
	}
	name = text[start:end]
	if name == "" {
		return "", rng{}, false
	}
	return name, rng{
		Start: doc.positionAt(start),
		End:   doc.positionAt(end),
	}, true
}

// sortPositions orders edits by position.
func sortPositions(xs []compiler.Symbol) {
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].File != xs[j].File {
			return xs[i].File < xs[j].File
		}
		if xs[i].Line != xs[j].Line {
			return xs[i].Line < xs[j].Line
		}
		return xs[i].Col < xs[j].Col
	})
}

var _ = token.Pos(0)
