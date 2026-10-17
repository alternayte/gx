package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_AUT_21_GeneratedCodeIsATemplateValue checks that the generated
// code of a component returns a template value (DR-11): the markup is in
// constant static strings, an element with an attribute that the render or a
// pass reads has its open tag as a dynamic value, and no element is a gx.El
// call. The render has the bytes of the tree of elements.
func TestREQ_AUT_21_GeneratedCodeIsATemplateValue(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n  Count int\n}\n\n" +
			`<section id="top" class="card"><header class="flex"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"></path></svg><span>Fish & chips</span><br><!-- note --></header><h3>{p.Title}</h3><ul data-gx-x="1"><li>{p.Count}</li></ul><a href="/about">About</a> <input disabled name="q"></section>` + "\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	for _, want := range []string{
		`gx.NewTemplate(`,
		// The static strings: the text is escaped at build time.
		`"<section"`,
		`"><header class=\"flex\"><svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M5 12h14\"></path></svg><span>Fish &amp; chips</span><br><!-- note --></header><h3>"`,
		`"</li></ul><a href=\"/about\">About</a> <input disabled name=\"q\"></section>\n"`,
		// The dynamic values, in the order of the document.
		`gx.Open("section", `,
		`gx.Text(p.Title)`,
		`gx.Open("ul", `,
		`gx.Int(int64(p.Count))`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("Card_gx.go lacks %s", want)
		}
	}
	if strings.Contains(src, "gx.El(") {
		t.Errorf("Card_gx.go builds an element at each render")
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
	fmt.Print(gx.String(card.Card(card.CardProps{Title: "T", Count: 1234})))
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
	want := `<section class="card" id="top"><header class="flex"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"></path></svg><span>Fish &amp; chips</span><br><!-- note --></header><h3>T</h3><ul data-gx-x="1"><li>1234</li></ul><a href="/about">About</a> <input disabled name="q"></section>` + "\n"
	if string(out) != want {
		t.Errorf("render:\n got %q\nwant %q", out, want)
	}
}

// TestREQ_ACT_15_FragmentElementIsMarked checks that the element of a
// fragment is a fragment value in generated code, in the component and in
// the fragment function, so the render gives it a hash (REQ-ACT-15).
func TestREQ_ACT_15_FragmentElementIsMarked(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/cart/Cart.gx": "package cart\n\nprops {\n  Total int\n}\n\n<div><span #total>{p.Total}</span><p class=\"x\">{p.Total}</p></div>\n",
	})
	src := string(generateFiles(t, dir)[filepath.Join(dir, "ui/cart/Cart_gx.go")])
	if got := strings.Count(src, `gx.OpenFragment("span", `); got != 2 {
		t.Errorf("Cart_gx.go has %d fragment values for the span, want 2: the component and the fragment function\n%s", got, src)
	}
	if strings.Contains(src, `gx.OpenFragment("p"`) || strings.Contains(src, `gx.OpenFragment("div"`) {
		t.Errorf("an element that is not a fragment is a fragment value:\n%s", src)
	}
}

// TestREQ_ACT_04_PatchComponentWithTrailingNewline patches a component
// whose .gx file ends with a newline, as gx fmt writes it. The newline is
// text after the root element, not a part to patch: c.Patch sends the root
// element, in a body with no statement and in a body with one.
func TestREQ_ACT_04_PatchComponentWithTrailingNewline(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  ID string\n}\n\n" +
			"<article id={p.ID}>card</article>\n",
		"ui/card/Note.gx": "package card\n\nprops {\n  ID string\n}\n\n" +
			"label := \"note\"\n<p id={p.ID}>{label}</p>\n",
		"main.go": `package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"app/ui/card"
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

type save struct{}

func (save) Pattern() string          { return "POST /save" }
func (save) Bind(*http.Request) error { return nil }

func main() {
	a := gx.Action(func(c *gx.Ctx, in save) error {
		return c.Patch(card.Card(card.CardProps{ID: "c1"}), card.Note(card.NoteProps{ID: "n1"}))
	})
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", gx.Collect(a))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/save", nil)
	req.Header.Set("Datastar-Request", "true")
	app.ServeHTTP(rec, req)
	fmt.Printf("status %d\n%s", rec.Code, rec.Body.String())
}
`,
	})
	_ = writeGenerated(t, dir)
	out := runModule(t, dir, "run", ".")
	for _, want := range []string{"status 200", `<article id="c1">card</article>`, `<p id="n1">note</p>`} {
		if !strings.Contains(out, want) {
			t.Errorf("answer lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "not an element") {
		t.Errorf("the trailing newline is a patch root:\n%s", out)
	}
}
