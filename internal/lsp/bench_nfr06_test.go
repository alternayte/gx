package lsp_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestNFR_06_LSPLatency covers the LSP budgets: completion under 100 ms and
// diagnostics under 300 ms after a keystroke (NFR-06).
func TestNFR_06_LSPLatency(t *testing.T) {
	dir, card := featureModule(t)
	text := readBody(t, card)
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(card, text)
	c.waitDiagnostics(card) // warm the session

	// Completion at p.Title: measure the protocol round trip.
	line, col := position(text, "p.Title")
	params := map[string]any{
		"textDocument": map[string]any{"uri": fileURI(card)},
		"position":     map[string]any{"line": line, "character": col + 2},
	}
	var maxCompletion time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		msg := c.request("textDocument/completion", params)
		took := time.Since(start)
		if took > maxCompletion {
			maxCompletion = took
		}
		var list completionList
		if err := json.Unmarshal(msg.Result, &list); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("NFR-06 completion max: %s", maxCompletion)
	if maxCompletion > 100*time.Millisecond {
		t.Fatalf("NFR-06 completion %s, want under 100ms", maxCompletion)
	}

	// Diagnostics after a keystroke: a body-only edit.
	var maxDiagnostics time.Duration
	view := text
	for i := 0; i < 3; i++ {
		view = strings.Replace(view, "total", "total ", 1)
		start := time.Now()
		c.didChange(card, view)
		c.waitDiagnostics(card)
		took := time.Since(start)
		if took > maxDiagnostics {
			maxDiagnostics = took
		}
	}
	t.Logf("NFR-06 diagnostics max: %s", maxDiagnostics)
	if maxDiagnostics > 300*time.Millisecond {
		t.Fatalf("NFR-06 diagnostics %s, want under 300ms", maxDiagnostics)
	}
}
