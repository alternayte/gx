package lintplugin_test

import (
	"testing"

	"github.com/alternayte/gx/internal/analyze"
	"github.com/alternayte/gx/lintplugin"
)

// TestREQ_TLS_03_PluginAnalyzers covers the golangci-lint module plugin: it
// builds the same Gx analyzers that gx lint runs (REQ-TLS-03).
func TestREQ_TLS_03_PluginAnalyzers(t *testing.T) {
	p, err := lintplugin.New(nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := p.BuildAnalyzers()
	if err != nil {
		t.Fatalf("BuildAnalyzers: %v", err)
	}
	want := analyze.Analyzers()
	if len(got) != len(want) {
		t.Fatalf("analyzers = %d, want %d", len(got), len(want))
	}
	seen := map[string]bool{}
	for _, a := range got {
		seen[a.Name] = true
	}
	for _, a := range want {
		if !seen[a.Name] {
			t.Fatalf("plugin lacks analyzer %q", a.Name)
		}
	}
	if p.GetLoadMode() == "" {
		t.Fatal("GetLoadMode is empty")
	}
}
