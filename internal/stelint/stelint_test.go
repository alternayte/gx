package stelint_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/stelint"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func words(t *testing.T) []stelint.Word {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "ste-words.txt"))
	if err != nil {
		t.Fatal(err)
	}
	list := stelint.ParseWords(string(raw))
	if len(list) < 20 {
		t.Fatalf("the word list holds %d entries", len(list))
	}
	return list
}

// TestNFR_10_Rules covers each rule of the linter with text that breaks it
// and text that follows it (NFR-10).
func TestNFR_10_Rules(t *testing.T) {
	list := words(t)
	long := "This sentence has more words than the limit of the standard because it joins many parts with and, with or and with other small words until it is too long."
	for _, tc := range []struct {
		name string
		text string
		rule string // "" means no finding
	}{
		{"a short sentence", "Run the command. It writes one file.", ""},
		{"a long sentence", long, "length"},
		{"a long paragraph", "One. Two. Three. Four. Five. Six. Seven.", "paragraph"},
		{"six sentences", "One. Two. Three. Four. Five. Six.", ""},
		{"a contraction", "The page doesn't change.", "contraction"},
		{"its is not a contraction", "The form keeps its values.", ""},
		{"the future tense", "The command will write the file.", "tense"},
		{"the passive voice in a step", "1. The file is written by the command.", "passive"},
		{"the passive voice with not in a step", "2. The handler was not called.", "passive"},
		{"a state in a step is not passive", "1. Make sure that the prop is required and the box is checked.", ""},
		{"the passive voice in a description", "The file is written by the command.", ""},
		{"the passive voice in a bullet", "- The file is written by the command.", ""},
		{"a word that is not approved", "You should utilize the helper.", "word"},
		{"a phrase that is not approved", "Run it in order to see the page.", "word"},
		{"a word inside a longer word", "The dismay is real.", ""},
		{"code is not prose", "Write `should` or `can't` in a string. See [the page that will change](/x/).", "tense"},
		{"a code span alone", "Write `it will be changed` in the test.", ""},
		{"a code block", "```go\n// This will be utilized.\n```\n", ""},
		{"a list item is a sentence", "- " + long, "length"},
		{"a table cell", "| Name | Use |\n| --- | --- |\n| `x` | It will run. |\n", "tense"},
		{"a heading", "## The page should load\n", "word"},
		{"the section name of the registry", "## Don't\n", ""},
		{"frontmatter title", "---\ntitle: \"What we will do\"\norder: 1\n---\n\nText.\n", "tense"},
		{"frontmatter keys are not prose", "---\nsample: will\n---\n\nText.\n", ""},
		{"a file name with dots", "Open `a.b`. The file main.go holds it.", ""},
	} {
		findings := stelint.Lint("page.md", tc.text, list)
		switch {
		case tc.rule == "" && len(findings) > 0:
			t.Errorf("%s: unexpected findings %v", tc.name, findings)
		case tc.rule != "":
			found := false
			for _, f := range findings {
				if f.Rule == tc.rule {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no %q finding in %v", tc.name, tc.rule, findings)
			}
		}
	}
	f := stelint.Lint("page.md", "Line one.\n\nIt will fail.\n", list)
	if len(f) != 1 || f[0].Line != 3 || f[0].File != "page.md" || !strings.Contains(f[0].String(), "page.md:3: tense") {
		t.Fatalf("finding = %v", f)
	}
}

// TestNFR_10_DocsFollowSTE runs the linter on every page of the docs site
// and on the README, as `just ste-lint` does (NFR-10).
func TestNFR_10_DocsFollowSTE(t *testing.T) {
	repo := repoRoot(t)
	findings, files, err := stelint.LintPaths([]string{filepath.Join(repo, "docs", "content"), filepath.Join(repo, "README.md")}, words(t))
	if err != nil {
		t.Fatal(err)
	}
	if files < 120 {
		t.Fatalf("the linter read %d files; the docs hold more", files)
	}
	for i, f := range findings {
		if i == 40 {
			t.Errorf("and %d more", len(findings)-i)
			break
		}
		t.Errorf("%s", strings.TrimPrefix(f.String(), repo+"/"))
	}
}
