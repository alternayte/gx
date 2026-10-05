package highlight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

const gxSample = `package cart

import "app/ui/card"

props {
  // Items are the lines of the cart.
  Items []Item
  Title string = "Cart"
}

signals {
  Qty int = 1
}

<card.Card title="Cart" class:raised={p.Raised} {...p.Attrs}>
  <:header>Your cart ({len(p.Items)})</:header>
  if len(p.Items) == 0 {
    <p class="muted">Empty &amp; quiet</p>
  } else {
    for _, it := range p.Items {
      <li #row(it Item) on:click={$Qty = $Qty + 1}>{it.Name}</li>
    }
  }
  {/* never renders */}
  <!-- renders -->
  <script>if (a < b) { go("</div>") }</script>
</card.Card>
`

// gxTokens returns the tokens of src, one "type value" entry per token.
func gxTokens(t *testing.T, src string) []chroma.Token {
	t.Helper()
	it, err := gxLexer{}.Tokenise(nil, src)
	if err != nil {
		t.Fatal(err)
	}
	return it.Tokens()
}

// TestREQ_CNT_04_GxLexer covers the .gx lexer: tags, attributes, directives,
// Go expressions, control lines, the props block and comments get types of
// their own, and the gx language uses the lexer (REQ-CNT-04).
func TestREQ_CNT_04_GxLexer(t *testing.T) {
	got := map[string]chroma.TokenType{}
	var joined strings.Builder
	for _, tok := range gxTokens(t, gxSample) {
		joined.WriteString(tok.Value)
		if _, seen := got[tok.Value]; !seen {
			got[tok.Value] = tok.Type
		}
	}
	if joined.String() != gxSample {
		t.Fatalf("the tokens do not join to the source:\n%s", joined.String())
	}
	for value, want := range map[string]chroma.TokenType{
		"package":                             chroma.KeywordNamespace,
		"props":                               chroma.KeywordDeclaration,
		"signals":                             chroma.KeywordDeclaration,
		"// Items are the lines of the cart.": chroma.CommentSingle,
		"Items":                               gxAttribute,
		"card.Card":                           chroma.NameFunction,
		"title":                               gxAttribute,
		`"Cart"`:                              chroma.LiteralString,
		"class:raised":                        chroma.Keyword,
		"on:click":                            chroma.Keyword,
		"...":                                 chroma.Operator,
		":header":                             chroma.NameLabel,
		"if":                                  chroma.Keyword,
		"else":                                chroma.Keyword,
		"for":                                 chroma.Keyword,
		"range":                               chroma.Keyword,
		"li":                                  chroma.NameTag,
		"#row":                                chroma.NameFunction,
		"$Qty":                                chroma.NameEntity,
		"&amp;":                               chroma.NameEntity,
		"{/* never renders */}":               chroma.CommentMultiline,
		"<!-- renders -->":                    chroma.CommentMultiline,
		"len":                                 chroma.NameBuiltin,
	} {
		if got[value] != want {
			t.Errorf("token %q has type %s, want %s", value, got[value], want)
		}
	}
	// The script body is JavaScript: its "<" and "</div>" open no tag.
	if typ, ok := got["div"]; ok {
		t.Errorf("the script body holds a tag token of type %s", typ)
	}

	html := RenderCode("gx", "", "<card.Card title=\"Cart\">{p.Title}</card.Card>\n")
	for _, want := range []string{`class="tok-name-function">card.Card<`, `class="tok-name-variable">title<`, `class="tok-literal-string">&#34;Cart&#34;<`} {
		if !strings.Contains(html, want) {
			t.Errorf("the gx code frame lacks %s:\n%s", want, html)
		}
	}
	if LangForFile("ui/card/Card.gx") != "gx" {
		t.Errorf("a .gx file has the language %q", LangForFile("ui/card/Card.gx"))
	}
}

// TestREQ_CNT_04_GxLexerKeepsSource covers the registry: the tokens of every
// .gx file join to the file, so a code frame never drops or moves text.
func TestREQ_CNT_04_GxLexerKeepsSource(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "registry", "*", "*.gx"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no registry .gx files: %v", err)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var joined strings.Builder
		for _, tok := range gxTokens(t, string(src)) {
			joined.WriteString(tok.Value)
		}
		if joined.String() != string(src) {
			t.Errorf("%s: the tokens do not join to the source", file)
		}
	}
	// A part of a file and broken source keep their text too.
	for _, src := range []string{"<div class=", "{p.Title", "props {\n  Title string", "<a href={x}", "}\n", "<!-- open", "<p>a < b</p>"} {
		var joined strings.Builder
		for _, tok := range gxTokens(t, src) {
			joined.WriteString(tok.Value)
		}
		if joined.String() != src {
			t.Errorf("%q: the tokens join to %q", src, joined.String())
		}
	}
}
