package lsp_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// semanticTokensResult decodes a textDocument/semanticTokens/full response.
type semanticTokensResult struct {
	Data []uint32 `json:"data"`
}

// TestREQ_TLS_04_SemanticTokens covers semantic tokens over standard LSP
// (REQ-TLS-04).
func TestREQ_TLS_04_SemanticTokens(t *testing.T) {
	_, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, filepath.Dir(filepath.Dir(filepath.Dir(card))))
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	msg := c.request("textDocument/semanticTokens/full", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
	})
	var res semanticTokensResult
	if err := json.Unmarshal(msg.Result, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Data) == 0 || len(res.Data)%5 != 0 {
		t.Fatalf("token data = %v, want groups of 5", res.Data)
	}
	types := map[uint32]bool{}
	for i := 0; i+4 < len(res.Data); i += 5 {
		types[res.Data[i+3]] = true
	}
	// 6 = gxComponent, 8 = gxSignal.
	if !types[6] || !types[8] {
		t.Fatalf("token types = %v, want the component and signal tokens", types)
	}
}

// TestREQ_TLS_04_InlayHints covers the inlay hints for generated fragment
// signatures (REQ-TLS-04).
func TestREQ_TLS_04_InlayHints(t *testing.T) {
	dir := module(t, map[string]string{
		"card/Card.gx": "package card\n\n<article>\n  total := 5\n  <span #total(total int)>{total}</span>\n</article>\n",
	})
	card := filepath.Join(dir, "card/Card.gx")
	text := readBody(t, card)
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	msg := c.request("textDocument/inlayHint", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 100, "character": 0},
		},
	})
	var hints []struct {
		Position struct{ Line, Character int } `json:"position"`
		Label    string                        `json:"label"`
	}
	if err := json.Unmarshal(msg.Result, &hints); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hints {
		if strings.Contains(h.Label, "CardTotal(total int) gx.Node") {
			found = true
		}
	}
	if !found {
		t.Fatalf("hints = %+v, want the generated fragment signature", hints)
	}
}

// TestREQ_TLS_04_OrganizeImports covers the organize-imports code action
// (REQ-TLS-04).
func TestREQ_TLS_04_OrganizeImports(t *testing.T) {
	dir, card := featureModule(t)
	c := newClient(t, dir)
	c.initialize()
	text := "package card\n\nimport \"app/ui/badge\"\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n"
	c.didOpen(card, text)
	c.waitDiagnostics(card)

	msg := c.request("textDocument/codeAction", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": 0, "character": 0},
		},
		"context": map[string]any{
			"diagnostics": []any{},
			"only":        []string{"source.organizeImports"},
		},
	})
	var list codeActionList
	if err := json.Unmarshal(msg.Result, &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range list {
		if a.Kind != "source.organizeImports" || a.Edit == nil {
			continue
		}
		for _, edits := range a.Edit.Changes {
			for _, e := range edits {
				if !strings.Contains(e.NewText, "app/ui/badge") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("organize imports did not drop the unused import: %+v", list)
	}
}

// TestREQ_TLS_04_AnalyzerDiagnostics covers analyzer diagnostics for open
// Go files: the LSP publishes GX7001 from the analyzer set (REQ-TLS-04).
func TestREQ_TLS_04_AnalyzerDiagnostics(t *testing.T) {
	dir := module(t, map[string]string{
		"cart/cart.go": "package cart\n\nimport gx \"github.com/alternayte/gx\"\n\nvar userInput string\n\nvar a = gx.SafeHTML(userInput)\n",
	})
	cart := filepath.Join(dir, "cart/cart.go")
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(cart, readBody(t, cart))
	d := c.waitDiagnostics(cart)
	found := false
	for _, item := range d.Items {
		if item.Code == "GX7001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v, want GX7001", d.Items)
	}
}

// TestREQ_TLS_04_ContentMarkdown covers content Markdown support: the LSP
// diagnoses an undeclared component and a bad prop, and completes the
// collection's components and their props (REQ-TLS-04, REQ-CNT-03).
func TestREQ_TLS_04_ContentMarkdown(t *testing.T) {
	dir := module(t, map[string]string{
		"docs/Aside.gx":         "package docs\n\nprops {\n  Kind string\n}\n\n<aside>{p.Kind}</aside>\n",
		"docs/content.go":       "package docs\n\nimport \"github.com/alternayte/gx\"\n\ntype DocMeta struct {\n\tTitle string\n}\n\nvar Docs = gx.Collection[DocMeta](\"content/docs\").Components(Aside)\n",
		"content/docs/start.md": "---\ntitle: Start\n---\n\n<docs.Aside kind=\"tip\">Good</docs.Aside>\n<docs.Widget />\n<docs.Aside wrong=\"x\" />\n",
	})
	md := filepath.Join(dir, "content/docs/start.md")
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(md, readBody(t, md))

	d := c.waitDiagnostics(md)
	codes := map[string]bool{}
	for _, item := range d.Items {
		codes[item.Code] = true
	}
	if !codes["GX8002"] || !codes["GX2003"] {
		t.Fatalf("content diagnostics = %+v", d.Items)
	}

	// Completion of a component name and of a prop.
	view := readBody(t, md) + "\n<docs.\n"
	c.didChange(md, view)
	c.waitDiagnostics(md)
	line, col := position(view, "<docs.")
	msg := c.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(md)},
		"position":     map[string]any{"line": line, "character": col + len("<docs.")},
	})
	var list completionList
	if err := json.Unmarshal(msg.Result, &list); err != nil {
		t.Fatal(err)
	}
	if !contains(labels(list), "docs.Aside") {
		t.Fatalf("tag completions = %v", labels(list))
	}

	view = readBody(t, md) + "\n<docs.Aside \n"
	c.didChange(md, view)
	c.waitDiagnostics(md)
	line, col = position(view, "<docs.Aside ")
	msg = c.request("textDocument/completion", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(md)},
		"position":     map[string]any{"line": line, "character": col + len("<docs.Aside ")},
	})
	if err := json.Unmarshal(msg.Result, &list); err != nil {
		t.Fatal(err)
	}
	if !contains(labels(list), "kind") {
		t.Fatalf("prop completions = %v", labels(list))
	}
}
