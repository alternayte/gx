package compiler_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/compiler"
)

// sessionTree writes a two-component module for the session tests.
func sessionTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"ui/card/Page.gx": "package card\n\n<Card title=\"hi\" />\n",
	})
}

// TestNFR_05_IncrementalSession covers the incremental compile: a body
// change re-checks and regenerates one file under the 50 ms budget
// (NFR-05).
func TestNFR_05_IncrementalSession(t *testing.T) {
	dir := sessionTree(t)
	s := compiler.NewSession()
	files, diags := s.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("full: %v", diags)
	}
	card := filepath.Join(dir, "ui/card/Card_gx.go")
	if !strings.Contains(string(files[card]), `"article"`) {
		t.Fatalf("initial output:\n%s", files[card])
	}

	view := filepath.Join(dir, "ui/card/Card.gx")
	changed := strings.Replace(cardSource(t, view), "<article>", `<article class="card">`, 1)
	if err := os.WriteFile(view, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	files, diags = s.Generate(dir)
	took := time.Since(start)
	if len(diags) > 0 {
		t.Fatalf("incremental: %v", diags)
	}
	// The class is in a static string of the template (DR-11).
	if !strings.Contains(string(files[card]), `card\"`) {
		t.Fatalf("incremental output:\n%s", files[card])
	}
	t.Logf("NFR-05 incremental session: %s", took)
	if took > 50*time.Millisecond {
		t.Fatalf("NFR-05 incremental session %s, want under 50ms", took)
	}
}

func cardSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestSession_TypeErrorIncremental covers a type error in a changed body:
// the session reports it and keeps the last good output out (NFR-05,
// REQ-DEV-06).
func TestSession_TypeErrorIncremental(t *testing.T) {
	dir := sessionTree(t)
	s := compiler.NewSession()
	if _, diags := s.Generate(dir); len(diags) > 0 {
		t.Fatalf("full: %v", diags)
	}
	view := filepath.Join(dir, "ui/card/Card.gx")
	bad := strings.Replace(cardSource(t, view), "{p.Title}", "{p.Nope}", 1)
	if err := os.WriteFile(view, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags := s.Generate(dir)
	found := false
	for _, d := range diags {
		if d.Code == compiler.CodeType && strings.Contains(d.Msg, "Nope") {
			found = true
		}
	}
	if !found {
		t.Fatalf("incremental type error = %v", diags)
	}
}

// TestSession_SignatureChangeFallsBack covers a props change: the session
// must re-check the caller and report the missing required prop (REQ-AUT-02).
func TestSession_SignatureChangeFallsBack(t *testing.T) {
	dir := sessionTree(t)
	s := compiler.NewSession()
	if _, diags := s.Generate(dir); len(diags) > 0 {
		t.Fatalf("full: %v", diags)
	}
	view := filepath.Join(dir, "ui/card/Card.gx")
	changed := strings.Replace(cardSource(t, view), "Title string", "Title string\n  Sub string", 1)
	if err := os.WriteFile(view, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags := s.Generate(dir)
	found := false
	for _, d := range diags {
		if d.Code == compiler.CodeRequiredProp {
			found = true
		}
	}
	if !found {
		t.Fatalf("signature change diags = %v", diags)
	}
}

// TestSession_GoChangeFull covers a .go change: the session reloads the
// module and sees the new default.
func TestSession_GoChangeFull(t *testing.T) {
	dir := sessionTree(t)
	s := compiler.NewSession()
	if _, diags := s.Generate(dir); len(diags) > 0 {
		t.Fatalf("full: %v", diags)
	}
	styles := filepath.Join(dir, "ui", "card", "titles.go")
	if err := os.WriteFile(styles, []byte("package card\n\nconst Title = \"new\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(dir, "ui/card/Page.gx")
	changed := strings.Replace(cardSource(t, page), `title="hi"`, "title={Title}", 1)
	if err := os.WriteFile(page, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	files, diags := s.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("go change: %v", diags)
	}
	pageGo := filepath.Join(dir, "ui/card/Page_gx.go")
	if !strings.Contains(string(files[pageGo]), "Title") {
		t.Fatalf("page output:\n%s", files[pageGo])
	}
}

// TestSession_BrokenThenFixed covers a parse error and its fix: the session
// must report the error, then rebuild after the file is restored.
func TestSession_BrokenThenFixed(t *testing.T) {
	dir := sessionTree(t)
	s := compiler.NewSession()
	if _, diags := s.Generate(dir); len(diags) > 0 {
		t.Fatalf("full: %v", diags)
	}
	view := filepath.Join(dir, "ui/card/Card.gx")
	original := cardSource(t, view)
	if err := os.WriteFile(view, []byte(original+"\nif true {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, diags := s.Generate(dir); len(diags) == 0 {
		t.Fatal("broken file produced no diagnostics")
	}
	// Putting the file back is a change against what was on disk, not
	// against the last good run.
	if err := os.WriteFile(view, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	files, diags := s.Generate(dir)
	if len(diags) > 0 {
		t.Fatalf("fixed: %v", diags)
	}
	card := filepath.Join(dir, "ui/card/Card_gx.go")
	if !strings.Contains(string(files[card]), `"article"`) {
		t.Fatalf("fixed output:\n%s", files[card])
	}
}
