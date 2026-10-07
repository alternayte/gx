package editors

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// corpusFiles returns the .gx files of the highlight corpus, in name order.
func corpusFiles(t *testing.T, root string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, "editors", "corpus", "*.gx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 3 {
		t.Fatalf("the highlight corpus holds %d files", len(files))
	}
	sort.Strings(files)
	return files
}

// snapshot compares got with a file of editors/corpus/snapshots.
// GX_UPDATE_GOLDEN=1 writes the file.
func snapshot(t *testing.T, root, name, got string) {
	t.Helper()
	path := filepath.Join(root, "editors", "corpus", "snapshots", name)
	if os.Getenv("GX_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no snapshot %s; run with GX_UPDATE_GOLDEN=1: %v", name, err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("%s differs at line %d:\n got: %s\nwant: %s\nIf the change is intended, run with GX_UPDATE_GOLDEN=1", name, i+1, g, w)
		}
	}
}

// TestREQ_DEV_09_TextMateHighlighting covers the highlighting of VS Code and
// of the JetBrains plugin: the two share one TextMate grammar. The test
// gives every token of the corpus its scopes and compares them with the
// snapshot.
func TestREQ_DEV_09_TextMateHighlighting(t *testing.T) {
	root := repoRoot(t)
	tools := filepath.Join(root, "editors", "corpus", "tools")
	if _, err := os.Stat(filepath.Join(tools, "node_modules", "vscode-textmate")); err != nil {
		install := exec.Command("bun", "install", "--frozen-lockfile")
		install.Dir = tools
		if out, err := install.CombinedOutput(); err != nil {
			t.Fatalf("bun install in editors/corpus/tools: %v\n%s", err, out)
		}
	}
	args := append([]string{"textmate.mjs", filepath.Join(root, "editors", "vscode", "syntaxes", "gx.tmLanguage.json")}, corpusFiles(t, root)...)
	cmd := exec.Command("bun", args...)
	cmd.Dir = tools
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("textmate.mjs: %v\n%s", err, stderr.String())
	}
	got := string(out)
	snapshot(t, root, "textmate.txt", got)

	// The scopes that carry the Gx meaning must be in use, so an empty
	// grammar cannot pass with an empty snapshot.
	for _, scope := range []string{
		"entity.name.tag.gx", "entity.name.tag.component.gx", "entity.name.tag.custom.gx",
		"entity.other.attribute-name.gx", "keyword.control.directive.gx", "variable.other.signal.gx",
		"entity.name.function.fragment.gx", "meta.embedded.block.go", "meta.embedded.block.css",
		"meta.embedded.block.javascript", "meta.embedded.block.typescript", "comment.block.gx",
	} {
		if !strings.Contains(got, scope) {
			t.Errorf("no token of the corpus has the scope %s", scope)
		}
	}
	// An expression with ">" does not end its tag, and markup inside a
	// control flow block is markup.
	for _, want := range []string{
		`"$Count" meta.tag.gx meta.embedded.block.go variable.other.signal.gx`,
		`"li" meta.tag.gx entity.name.tag.gx`,
		`"transition" meta.tag.gx keyword.control.directive.gx`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the snapshot lacks the token %s", want)
		}
	}
}

var captureLine = regexp.MustCompile("capture: (?:\\d+ - )?([\\w.]+), start: \\((\\d+), (\\d+)\\), end: \\((\\d+), (\\d+)\\)(?:, text: `(.*)`)?")

// TestREQ_DEV_09_TreeSitterHighlighting covers the highlighting of Neovim:
// the captures of the highlight query on the corpus, compared with the
// snapshot.
func TestREQ_DEV_09_TreeSitterHighlighting(t *testing.T) {
	root := repoRoot(t)
	grammar := filepath.Join(root, "editors", "tree-sitter-gx")
	var b strings.Builder
	for _, file := range corpusFiles(t, root) {
		// The pinned CLI comes from the npm cache, as in checks/tree-sitter.sh.
		cmd := exec.Command("npx", "--yes", "--prefer-offline", "tree-sitter-cli@0.25.10", "query", "queries/highlights.scm", file)
		cmd.Dir = grammar
		cmd.Env = append(os.Environ(), "TREE_SITTER_LIBDIR="+filepath.Join(t.TempDir(), "lib"))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("tree-sitter query %s: %v\n%s", filepath.Base(file), err, stderr.String())
		}
		fmt.Fprintf(&b, "== %s\n", filepath.Base(file))
		for _, line := range strings.Split(string(out), "\n") {
			m := captureLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			// Rows and columns start at 1, as in an editor.
			var row, col int
			fmt.Sscan(m[2], &row)
			fmt.Sscan(m[3], &col)
			text := m[6]
			if m[2] != m[4] {
				text = "(to " + m[4] + ":" + m[5] + ")"
			}
			fmt.Fprintf(&b, "%d:%d %s %q\n", row+1, col+1, m[1], text)
		}
	}
	got := b.String()
	snapshot(t, root, "treesitter.txt", got)
	for _, capture := range []string{" tag ", " type ", " attribute ", " keyword ", " function ", " string ", " comment ", " embedded "} {
		if !strings.Contains(got, capture) {
			t.Errorf("no node of the corpus has the capture%s", capture)
		}
	}
}
