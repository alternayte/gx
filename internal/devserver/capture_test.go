package devserver

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_AI_12_AddFixture covers the write of a captured fixture: a new
// fixtures file, a new entry in a file that exists with the import that
// the entry needs, and no write for a name that exists (REQ-AI-12).
func TestREQ_AI_12_AddFixture(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Card_gx.go"), []byte("package ui\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Card.fixtures.go")
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if formatted, err := format.Source(data); err != nil || string(formatted) != string(data) {
			t.Fatalf("the file is not gofmt source (%v):\n%s", err, data)
		}
		return string(data)
	}

	if err := addFixture(dir, "Card", "FromPage", `{Title: "A", Body: gx.Raw("<b>x</b>")}`, []string{"github.com/alternayte/gx"}); err != nil {
		t.Fatal(err)
	}
	want := `package ui

import "github.com/alternayte/gx"

// CardFixtures are the examples of Card in the dev gallery.
var CardFixtures = gx.Fixtures[CardProps]{
	"FromPage": {Title: "A", Body: gx.Raw("<b>x</b>")},
}
`
	if got := read(); got != want {
		t.Fatalf("the new file:\n%s\nwant:\n%s", got, want)
	}

	if err := addFixture(dir, "Card", "Dated", `{When: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}`, []string{"time"}); err != nil {
		t.Fatal(err)
	}
	got := read()
	for _, part := range []string{
		"\t\"FromPage\": {Title: \"A\", Body: gx.Raw(\"<b>x</b>\")},\n\t\"Dated\":    {When: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)},\n}\n",
		"\t\"time\"\n",
		"\t\"github.com/alternayte/gx\"\n",
		"// CardFixtures are the examples of Card in the dev gallery.\n",
	} {
		if !strings.Contains(got, part) {
			t.Errorf("the file lacks %q:\n%s", part, got)
		}
	}

	err := addFixture(dir, "Card", "Dated", `{}`, nil)
	if err == nil || !strings.Contains(err.Error(), `the fixture "Dated" exists`) {
		t.Errorf("a name that exists: err = %v", err)
	}
	if read() != got {
		t.Error("the failed capture changed the file")
	}

	// A literal on one line, and a file with no gx.Fixtures literal.
	if err := os.WriteFile(path, []byte("package ui\n\nimport \"github.com/alternayte/gx\"\n\nvar Fixtures = gx.Fixtures[CardProps]{\"A\": {}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addFixture(dir, "Card", "B", `{Title: "b"}`, nil); err != nil {
		t.Fatal(err)
	}
	if got := read(); !strings.Contains(got, "\"A\": {},\n\t\"B\": {Title: \"b\"},\n}") {
		t.Errorf("the literal on one line:\n%s", got)
	}
	if err := os.WriteFile(path, []byte("package ui\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addFixture(dir, "Card", "C", `{}`, nil); err == nil || !strings.Contains(err.Error(), "no gx.Fixtures literal") {
		t.Errorf("a file with no literal: err = %v", err)
	}
	for _, name := range []string{"", "has space", "9x", `a"b`} {
		if err := addFixture(dir, "Card", name, `{}`, nil); err == nil || !strings.Contains(err.Error(), "name") {
			t.Errorf("the name %q: err = %v", name, err)
		}
	}
}
