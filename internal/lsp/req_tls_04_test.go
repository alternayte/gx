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
