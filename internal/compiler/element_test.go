package compiler_test

import (
	"html"
	"os/exec"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// waElements is the file that `gx wc pin` writes for two elements of a
// component library.
const waElements = `{
  "package": "@awesome.me/webawesome",
  "version": "3.14.0",
  "elements": [
    {
      "name": "Button",
      "tag": "wa-button",
      "module": "@awesome.me/webawesome/dist/components/button/button.js",
      "attributes": [
        {"name": "variant", "kind": "enum", "values": ["neutral", "brand", "success", "warning", "danger"]},
        {"name": "size", "kind": "enum", "values": ["s", "m", "l"]},
        {"name": "disabled", "kind": "bool"},
        {"name": "with-caret", "kind": "bool"},
        {"name": "href", "kind": "string"},
        {"name": "tabindex", "kind": "number"}
      ],
      "events": ["blur", "focus", "wa-invalid"],
      "slots": ["start", "end"]
    },
    {
      "name": "Icon",
      "tag": "wa-icon",
      "module": "@awesome.me/webawesome/dist/components/icon/icon.js",
      "attributes": [{"name": "name", "kind": "string"}],
      "events": [],
      "slots": []
    }
  ]
}
`

func waTree(t *testing.T, page string, extra map[string]string) string {
	t.Helper()
	files := map[string]string{
		"go.mod":                 moduleWithGx(t),
		"ui/wa/gx-elements.json": waElements,
		"ui/wa/elements_gx.go":   "// Package wa holds the typed tags of a component library.\npackage wa\n",
		"home/Page.gx":           page,
	}
	for k, v := range extra {
		files[k] = v
	}
	return writeTree(t, files)
}

// The acceptance of REQ-ISL-09: <wa.Button variant="brand"> type-checks,
// and a wrong variant fails.
func TestREQ_ISL_09_VariantIsChecked(t *testing.T) {
	ok := waTree(t, "package home\n\nimport \"app/ui/wa\"\n\n<main><wa.Button variant=\"brand\">Save</wa.Button></main>\n", nil)
	if diags := compiler.Check(ok); len(diags) != 0 {
		t.Fatalf("a valid variant: unexpected diagnostics %v", diags)
	}
	bad := waTree(t, "package home\n\nimport \"app/ui/wa\"\n\n<main><wa.Button variant=\"brnad\">Save</wa.Button></main>\n", nil)
	diags := compiler.Check(bad)
	d := diagWith(t, diags, compiler.CodeStaticStringProp)
	if d.Line != 5 || !strings.Contains(d.Msg, `"brnad"`) || !strings.Contains(d.Msg, "<wa.Button>") ||
		!strings.Contains(d.Msg, "neutral, brand, success, warning, danger") {
		t.Fatalf("GX2004 = %v", d)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %v, want only GX2004", diags)
	}
}

func TestREQ_ISL_09_ManifestChecks(t *testing.T) {
	page := `package home

import "app/ui/wa"

props {
  Off  bool
  Text string
}

<main>
  <wa.Button colour="red">a</wa.Button>
  <wa.Button varient="brand">b</wa.Button>
  <wa.Button disabled="yes">c</wa.Button>
  <wa.Button variant>d</wa.Button>
  <wa.Button tabindex="first">e</wa.Button>
  <wa.Button disabled={p.Text}>f</wa.Button>
  <wa.Button on:wa-change={$Open = true}>g</wa.Button>
  <wa.Button><:prefix>x</:prefix>h</wa.Button>
  <wa.Button><:start>x</:start><:start>y</:start>i</wa.Button>
  <wa.Buton>j</wa.Buton>
  <wa.Button href={p.Text}>k</wa.Button>
</main>
`
	dir := waTree(t, page, nil)
	diags := compiler.Check(dir)
	want := []struct {
		line int
		code string
		text string
	}{
		{11, compiler.CodeUnknownAttr, `unknown attribute "colour" on <wa.Button>`},
		{12, compiler.CodeUnknownAttr, `did you mean "variant"?`},
		{13, compiler.CodeStaticStringProp, "is a boolean attribute"},
		{14, compiler.CodeStaticStringProp, "needs a value"},
		{15, compiler.CodeStaticStringProp, "is a number"},
		{16, compiler.CodeType, `attribute "disabled" of <wa.Button> needs a bool expression, got string`},
		{17, compiler.CodeUnknownAttr, `unknown event "wa-change" on <wa.Button>; the element sends blur, focus, wa-invalid`},
		{18, compiler.CodeUnknownAttr, `unknown slot "prefix" on <wa.Button>; the element has the slots start, end`},
		{19, compiler.CodeDuplicateSlot, `slot "start" is given twice`},
		{20, compiler.CodeUnknownComponent, `did you mean "Button"?`},
		// An imported element is an HTML element for the URL rule (SI-02).
		{21, compiler.CodeURLAttr, ""},
	}
	byLine := map[int][]compiler.Diagnostic{}
	for _, d := range diags {
		byLine[d.Line] = append(byLine[d.Line], d)
	}
	for _, w := range want {
		found := false
		for _, d := range byLine[w.line] {
			if d.Code == w.code && strings.Contains(d.Msg, w.text) {
				found = true
			}
		}
		if !found {
			t.Errorf("line %d: no %s with %q in %v", w.line, w.code, w.text, byLine[w.line])
		}
	}
	// Line 17 also reports the signal that the page does not declare.
	if len(diags) < len(want) || len(diags) > len(want)+2 {
		t.Errorf("diagnostics = %d, want about %d:\n%v", len(diags), len(want), diags)
	}
}

// The tag renders the custom element: attributes, a boolean attribute from
// a bool expression, events, classes, a spread and slots.
func TestREQ_ISL_09_TagRendersTheCustomElement(t *testing.T) {
	page := `package home

import (
  "app/home/route"
  "app/ui/wa"
)

props {
  Off   bool
  Size  string
  Extra gx.Attrs = nil
}

signals {
  Open bool = false
}

<main>
  <wa.Button variant="brand" size={p.Size} disabled={p.Off} with-caret class="w-full" id="save" on:click={$Open = true} on:wa-invalid={$Open = false} {...p.Extra}>
    <:start><wa.Icon name="check" /></:start>
    <:end>after <b>text</b></:end>
    Save
  </wa.Button>
  <wa.Button href={route.Home{}} disabled={!p.Off}>Home</wa.Button>
</main>
`
	dir := waTree(t, page, map[string]string{
		"home/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /`\n}\n",
		"main.go": `package main

import (
	"fmt"

	"app/home"
	gx "github.com/alternayte/gx"
)

func main() {
	gx.SetIslands(gx.IslandBundle{Entries: map[string]string{
		"@awesome.me/webawesome/dist/components/button/button.js": "elements/button-ABCD1234.js",
		"@awesome.me/webawesome/dist/components/icon/icon.js":     "elements/icon-ABCD1234.js",
	}})
	fmt.Print(gx.String(home.Page(home.PageProps{Off: true, Size: "l", Extra: gx.Attrs{{Key: "data-x", Value: "1"}}})))
}
`,
	})
	if diags := compiler.Check(dir); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics %v", diags)
	}
	writeGenerated(t, dir)
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, out)
	}
	got := html.UnescapeString(string(out))
	for _, want := range []string{
		`<wa-button class="w-full" variant="brand" size="l" disabled with-caret id="save"`,
		`data-on:click="`,
		`data-on:wa-invalid="`,
		`data-x="1"`,
		`data-gx-module="/_gx/islands/elements/button-ABCD1234.js"`,
		// The tag sets five attributes; a morph leaves the other two alone.
		`data-preserve-attr="href tabindex"`,
		`<wa-icon name="check" data-gx-module="/_gx/islands/elements/icon-ABCD1234.js" slot="start"></wa-icon>`,
		`<span style="display:contents" slot="end">after <b>text</b></span>`,
		`<wa-button href="/" data-gx-active="page" data-gx-module="/_gx/islands/elements/button-ABCD1234.js" data-preserve-attr="size tabindex variant with-caret">Home</wa-button>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %s", want)
		}
	}
	if t.Failed() {
		t.Logf("output:\n%s", got)
	}
}

func TestREQ_ISL_09_ElementModulesInUse(t *testing.T) {
	dir := waTree(t, "package home\n\nimport \"app/ui/wa\"\n\n<main><wa.Button>a</wa.Button><wa.Button>b</wa.Button></main>\n", nil)
	got := compiler.ElementModules(dir)
	if len(got) != 1 || got[0] != "@awesome.me/webawesome/dist/components/button/button.js" {
		t.Fatalf("modules = %v, want the button module only", got)
	}
}
