package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNFR_03_StaticSubtreeIsOneChunk checks that an element with no
// expression below it is one pre-escaped string in the generated code
// (SDD 3.2: "Static parts are pre-escaped byte chunks"), and that the render
// has the same bytes as the tree of elements.
//
// An element stays a gx.El call when the render or a tree pass reads it: the
// root of the component, an element with an id or a data- attribute, and an
// element with an expression below it.
func TestNFR_03_StaticSubtreeIsOneChunk(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n" +
			`<section id="top" class="card"><header class="flex"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"></path></svg><span>Fish & chips</span><br><!-- note --></header><h3>{p.Title}</h3><ul data-gx-x="1"><li>one</li></ul><a href="/about">About</a> <input disabled name="q"></section>` + "\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	for _, want := range []string{
		`gx.El("section", `,
		`gx.Raw(gx.SafeHTML("<header class=\"flex\"><svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M5 12h14\"></path></svg><span>Fish &amp; chips</span><br><!-- note --></header>"))`,
		`gx.El("h3", `,
		`gx.El("ul", `,
		`gx.Raw(gx.SafeHTML("<li>one</li>"))`,
		`gx.Raw(gx.SafeHTML("<a href=\"/about\">About</a> <input disabled name=\"q\">"))`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Card_gx.go lacks %s", want)
		}
	}
	for _, not := range []string{`gx.El("header"`, `gx.El("svg"`, `gx.El("li"`, `gx.El("a"`, `gx.El("input"`} {
		if strings.Contains(src, not) {
			t.Errorf("Card_gx.go builds a static element at each render: %s", not)
		}
	}
	if t.Failed() {
		t.Logf("Card_gx.go:\n%s", src)
	}

	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mainSrc := `package main

import (
	"fmt"

	"app/ui/card"
	gx "github.com/alternayte/gx"
)

func main() {
	fmt.Print(gx.String(card.Card(card.CardProps{Title: "T"})))
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	want := `<section class="card" id="top"><header class="flex"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"></path></svg><span>Fish &amp; chips</span><br><!-- note --></header><h3>T</h3><ul data-gx-x="1"><li>one</li></ul><a href="/about">About</a> <input disabled name="q"></section>` + "\n"
	if string(out) != want {
		t.Errorf("render:\n got %q\nwant %q", out, want)
	}
}
