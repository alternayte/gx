package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const docCard = `package card

props {
  // Title is the heading text.
  Title string
  // Href is the link target.
  //
  // An empty value renders no link.
  Href string = "https://example.com/a"
  Open bool   = false
}

signals {
  // Qty is the count the stepper shows.
  Qty int = 1
}

<article>{p.Title}</article>
`

// TestREQ_AUT_20_PropComment covers the one comment a props or signals
// block accepts: // lines above a field. Every other comment in a block is a
// parse error.
func TestREQ_AUT_20_PropComment(t *testing.T) {
	f, diags := compiler.ParseFile("Card.gx", []byte(docCard))
	if len(diags) > 0 {
		t.Fatalf("diagnostics = %v", diags)
	}
	if got := f.Props[0].Doc; got != "Title is the heading text." {
		t.Errorf("Title doc = %q", got)
	}
	if got := f.Props[1].Doc; got != "Href is the link target.\n\nAn empty value renders no link." {
		t.Errorf("Href doc = %q", got)
	}
	if got := f.Props[1].Default; got != `"https://example.com/a"` {
		t.Errorf("Href default = %q: a // inside a string is not a comment", got)
	}
	if got := f.Props[2].Doc; got != "" {
		t.Errorf("Open doc = %q, want none", got)
	}
	if got := f.Signals[0].Doc; got != "Qty is the count the stepper shows." {
		t.Errorf("Qty doc = %q", got)
	}

	// A blank line between the comment and the field does not detach it.
	f, diags = compiler.ParseFile("Card.gx", []byte("package card\n\nprops {\n  // The title.\n\n  Title string\n}\n\n<p>{p.Title}</p>\n"))
	if len(diags) > 0 || f.Props[0].Doc != "The title." {
		t.Errorf("comment above a blank line: file = %+v, diagnostics = %v", f, diags)
	}

	for name, body := range map[string]string{
		"a comment after the field on its line": "  Title string // the title\n",
		"a comment after a default":             "  Title string = \"a\" // the title\n",
		"a comment with no field below it":      "  Title string\n  // the end\n",
		"a block comment above a field":         "  /* the title */\n  Title string\n",
		"a block comment inside a field":        "  Title /* the title */ string\n",
	} {
		_, diags := compiler.ParseFile("Card.gx", []byte("package card\n\nprops {\n"+body+"}\n\n<p>{p.Title}</p>\n"))
		if !hasCode(diags, compiler.CodeParse) {
			t.Errorf("%s: diagnostics = %v, want GX1000", name, diags)
		}
	}
}

// TestREQ_AUT_17_FmtKeepsPropComment covers the formatter: it keeps the
// description of a field, in one canonical form. A comment with no text is
// no description.
func TestREQ_AUT_17_FmtKeepsPropComment(t *testing.T) {
	src := "package card\n\nprops {\n\n  //The title.\n  //   Indented.   \n  //\n\n  Title string\n  //  \n  Open bool = false\n}\nsignals {\n// The count.\nQty int = 1 }\n\n<p>{p.Title}</p>\n"
	want := "package card\n\nprops {\n  // The title.\n  //   Indented.\n  //\n  Title string\n  Open  bool = false\n}\n\nsignals {\n  // The count.\n  Qty int = 1\n}\n\n<p>{p.Title}</p>\n"
	out, diags := compiler.FormatSource("Card.gx", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("diagnostics = %v", diags)
	}
	if string(out) != want {
		t.Fatalf("formatted source:\n%s\nwant:\n%s", out, want)
	}
	again, diags := compiler.FormatSource("Card.gx", out)
	if len(diags) > 0 || string(again) != want {
		t.Fatalf("format is not idempotent (diagnostics %v):\n%s", diags, again)
	}
}

// TestREQ_AUT_02_GeneratedPropsCarryDescription covers the generated props
// struct: the description of a field is its Go doc comment.
func TestREQ_AUT_02_GeneratedPropsCarryDescription(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": docCard,
	})
	card := string(generateFiles(t, dir)[filepath.Join(dir, "ui/card/Card_gx.go")])
	for _, want := range []string{
		"\t// Title is the heading text.\n\tTitle string\n",
		"\t// Href is the link target.\n\t//\n\t// An empty value renders no link.\n\tHref  string\n",
		"\t// Qty is the count the stepper shows.\n\tQty int `json:\"qty\"`\n",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("Card_gx.go lacks %q:\n%s", want, card)
		}
	}
}
