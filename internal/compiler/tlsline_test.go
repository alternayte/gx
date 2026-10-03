package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeGenerated(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := generateFiles(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestREQ_TLS_02_LineDirectives(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Items []string\n}\n\n<p>{p.Items[3]}</p>\n",
		"ui/card/Page.gx": "package card\n\n<Card items={[]string{\"a\"}} />\n",
	})
	files := writeGenerated(t, dir)
	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(card, "//line Card.gx:7:1") {
		t.Fatalf("no //line directive for the expression:\n%s", card)
	}

	mainSrc := `package main

import (
	"fmt"

	"app/ui/card"
	gx "github.com/alternayte/gx"
)

func main() {
	fmt.Print(gx.String(card.Page(card.PageProps{})))
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected a panic, got success:\n%s", out)
	}
	if !strings.Contains(string(out), "Card.gx:7") {
		t.Fatalf("panic does not name the .gx position:\n%s", out)
	}
}

func TestREQ_TLS_02_VetReportsTemplatePosition(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nimport \"fmt\"\n\nprops {\n  Items []string\n}\n\n<p>{fmt.Sprintf(\"%d\")}</p>\n",
	})
	writeGenerated(t, dir)
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "Card.gx:9") {
		t.Fatalf("vet does not name the .gx position:\n%s", out)
	}
}
