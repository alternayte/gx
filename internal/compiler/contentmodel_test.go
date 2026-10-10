package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// contentFindings generates one component and returns the content model
// findings of gx check for it, as "code line" texts in order.
func contentFindings(t *testing.T, body string) ([]string, []compiler.Diagnostic) {
	t.Helper()
	dir := writeTree(t, map[string]string{
		"go.mod":       moduleWithGx(t),
		"ui/x/View.gx": "package x\n\nprops {\n  On    bool\n  ID    string\n  Attrs gx.Attrs = nil\n  Items []string\n}\n\n" + body + "\n",
	})
	writeGenerated(t, dir)
	var out []string
	diags := compiler.CheckApp(dir, compiler.CheckOptions{})
	for _, d := range diags {
		if !strings.HasSuffix(d.File, "View.gx") {
			t.Fatalf("an unexpected diagnostic: %s", d.String())
		}
		// The body starts at line 10 of the file.
		out = append(out, d.Code+" "+string(rune('0'+d.Line-9)))
	}
	return out, diags
}

// TestREQ_AUT_23_ContentModel checks each content model defect of gx check:
// one case that is a defect and the cases next to it that are not
// (REQ-AUT-23).
func TestREQ_AUT_23_ContentModel(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want []string
	}{
		{"an img with no alt", `<div><img src="/a.png"></div>`, []string{"GX2016 1"}},
		{"an img with alt, an empty alt, a spread or a hidden one", "<div>\n<img src=\"/a.png\" alt=\"A\">\n<img src=\"/a.png\" alt=\"\">\n<img src=\"/a.png\" {...p.Attrs}>\n<img src=\"/a.png\" aria-hidden=\"true\">\n</div>", nil},

		{"a control with no label", "<form>\n<input name=\"q\">\n<select name=\"s\"></select>\n<textarea name=\"t\"></textarea>\n</form>", []string{"GX2017 2", "GX2017 3", "GX2017 4"}},
		{"a control inside a label", `<label>Name <input name="q"></label>`, nil},
		{"a control with a label for its id", "<div>\n<label for=\"q\">Name</label>\n<input id=\"q\" name=\"q\">\n<label for={p.ID}>Name</label>\n<input id={p.ID} name=\"r\">\n</div>", nil},
		{"a label for a different id", "<div>\n<label for=\"other\">Name</label>\n<input id=\"q\" name=\"q\">\n</div>", []string{"GX2017 3"}},
		{"a control with an aria label, a title or a spread", "<div>\n<input name=\"a\" aria-label=\"Name\">\n<input name=\"b\" aria-labelledby=\"x\">\n<input name=\"c\" title=\"Name\">\n<input name=\"d\" {...p.Attrs}>\n</div>", nil},
		{"an input that needs no label", "<form>\n<input type=\"hidden\" name=\"a\">\n<input type=\"submit\" value=\"Go\">\n<input type=\"button\" value=\"Go\">\n</form>", nil},

		{"a button inside a link", `<a href="/x"><button type="button">Go</button></a>`, []string{"GX2018 1"}},
		{"a link inside a button, below a span", `<button type="button"><span><a href="/x">Go</a></span></button>`, []string{"GX2018 1"}},
		{"a link beside a button", `<div><a href="/x">Go</a><button type="button">Go</button></div>`, nil},

		{"one id on two elements", "<div>\n<p id=\"a\">One</p>\n<p id=\"a\">Two</p>\n<p id=\"b\">Three</p>\n</div>", []string{"GX2019 3"}},
		{"one id in the two branches of an if", "<div>\n  if p.On {\n    <p id=\"a\">One</p>\n  } else {\n    <p id=\"a\">Two</p>\n  }\n</div>", nil},
		{"one id in a branch and after it", "<div>\n  if p.On {\n    <p id=\"a\">One</p>\n  }\n  <p id=\"a\">Two</p>\n</div>", []string{"GX2019 5"}},

		{"a div in a paragraph", `<p>Text <div>Box</div></p>`, []string{"GX2020 1"}},
		{"a list item with no list, a row with no table", "<div>\n<li>One</li>\n<tr><td>One</td></tr>\n</div>", []string{"GX2020 2", "GX2020 3"}},
		{"a list and a table", "<div>\n<ul><li>One</li></ul>\n<table><tbody><tr><td>One</td></tr></tbody></table>\n<p>Text <span>more</span></p>\n</div>", nil},
		{"a list item at the top of a component", `<li>One</li>`, nil},
		{"a list item in a loop of a list", "<ul>\n  for _, it := range p.Items {\n    <li>{it}</li>\n  }\n</ul>", nil},
	} {
		got, _ := contentFindings(t, c.body)
		if strings.Join(got, ", ") != strings.Join(c.want, ", ") {
			t.Errorf("%s: findings [%s], want [%s]", c.name, strings.Join(got, ", "), strings.Join(c.want, ", "))
		}
	}
}

// TestREQ_AUT_23_FindingsFailTheCheck checks that a content model defect is
// an error of gx check and not of gx generate, and that a heading that skips
// a level is a warning that does not fail the check (REQ-AUT-23).
func TestREQ_AUT_23_FindingsFailTheCheck(t *testing.T) {
	_, diags := contentFindings(t, `<div><img src="/a.png"></div>`)
	if len(diags) != 1 || diags[0].Hint || !compiler.Failed(diags) {
		t.Errorf("an img with no alt: %v, want one error", diags)
	}
	got, diags := contentFindings(t, "<section>\n<h1>Title</h1>\n<h3>Part</h3>\n<h4>Sub</h4>\n<h2>Next</h2>\n</section>")
	if strings.Join(got, ", ") != "GX2021 3" || !diags[0].Hint || compiler.Failed(diags) {
		t.Errorf("a heading that skips a level: %v, want one warning at the h3 that does not fail the check", diags)
	}
	if !strings.Contains(diags[0].Msg, "<h3>") || !strings.Contains(diags[0].Fix, "<h2>") {
		t.Errorf("the warning does not name the heading and the fix: %+v", diags[0])
	}
}
