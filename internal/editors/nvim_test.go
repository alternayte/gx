package editors

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_TLS_06_NeovimArtifacts covers the Neovim integration: filetype
// detection, the LSP config, conform and nvim-lint setups, the LazyVim
// extra and the tree-sitter grammar with its queries (REQ-TLS-06).
func TestREQ_TLS_06_NeovimArtifacts(t *testing.T) {
	nvim := filepath.Join(repoRoot(t), "editors", "nvim")
	for _, want := range []struct {
		file string
		rels []string
	}{
		{"ftdetect/gx.lua", []string{"gx = \"gx\""}},
		{"lsp/gx.lua", []string{"\"lsp\"", `filetypes = { "gx" }`}},
		{"gx.lua", []string{"\"fmt\""}},
		{"nvim-lint.lua", []string{"\"lint\"", "from_pattern"}},
		{"lazyvim-extra-gx.lua", []string{"servers.gx", "formatters_by_ft"}},
	} {
		body := readBody(t, filepath.Join(nvim, want.file))
		for _, rel := range want.rels {
			if !strings.Contains(body, rel) {
				t.Fatalf("%s lacks %q:\n%s", want.file, rel, body)
			}
		}
	}

	ts := filepath.Join(repoRoot(t), "editors", "tree-sitter-gx")
	if !strings.Contains(readBody(t, filepath.Join(ts, "grammar.js")), "name: 'gx'") {
		t.Fatal("grammar.js is not the gx grammar")
	}
	if !strings.Contains(readBody(t, filepath.Join(ts, "src", "parser.c")), "tree_sitter_gx") {
		t.Fatal("parser.c does not hold the gx parser")
	}
	highlights := readBody(t, filepath.Join(ts, "queries", "highlights.scm"))
	for _, capture := range []string{"@type", "@function", "@keyword"} {
		if !strings.Contains(highlights, capture) {
			t.Fatalf("highlights.scm lacks %s", capture)
		}
	}
	injections := readBody(t, filepath.Join(ts, "queries", "injections.scm"))
	for _, want := range []string{`injection.language "go"`, `injection.language "typescript"`, `injection.language "css"`} {
		if !strings.Contains(injections, want) {
			t.Fatalf("injections.scm lacks %s", want)
		}
	}
	if !strings.Contains(readBody(t, filepath.Join(nvim, "test", "smoke.lua")), "vim.lsp") {
		t.Fatal("the smoke test does not attach the LSP")
	}
}
