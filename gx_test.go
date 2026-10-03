package gx_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

func TestRenderBasics(t *testing.T) {
	node := gx.El("div", gx.Attrs{{Key: "class", Value: "a<b"}},
		gx.Text("x < y"),
		gx.Raw(gx.SafeHTML("<br>")),
	)
	want := `<div class="a&lt;b">x &lt; y<br></div>`
	if got := gx.String(node); got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestRenderBoolAttr(t *testing.T) {
	on := gx.El("input", gx.Attrs{gx.Bool("disabled", true)})
	if got, want := gx.String(on), "<input disabled>"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
	off := gx.El("input", gx.Attrs{gx.Bool("disabled", false)})
	if got, want := gx.String(off), "<input>"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestRenderFragment(t *testing.T) {
	node := gx.Frag(gx.Text("a"), gx.El("b", nil, gx.Text("c")))
	if got, want := gx.String(node), "a<b>c</b>"; got != want {
		t.Fatalf("String = %q, want %q", got, want)
	}
}

func TestREQ_AUT_12_EscapingMatrix(t *testing.T) {
	cases := []struct {
		name string
		node gx.Node
		want string
	}{
		{"text", gx.Text("a<b&c>\"'"), `a&lt;b&amp;c&gt;"'`},
		{"attribute", gx.El("div", gx.Attrs{{Key: "title", Value: `a<b&c>"'`}}), `<div title="a&lt;b&amp;c&gt;&#34;&#39;"></div>`},
		{"url", gx.El("a", gx.Attrs{{Key: "href", Value: `/a b?x=1&y=2"`, Kind: gx.AttrURL}}), `<a href="/a%20b?x=1&amp;y=2%22"></a>`},
		{"style", gx.El("div", gx.Attrs{{Key: "style", Value: `color: red" onmouseover="x`, Kind: gx.AttrStyle}}), `<div style="color: red&#34; onmouseover=&#34;x"></div>`},
		{"raw", gx.Raw(gx.SafeHTML("<b>&</b>")), `<b>&</b>`},
		{"text no double escape", gx.Text("&amp;"), "&amp;amp;"},
		{"text nul", gx.Text("a\x00b"), "a\x00b"},
		{"attr backtick", gx.El("div", gx.Attrs{{Key: "data-x", Value: "a`b"}}), "<div data-x=\"a`b\"></div>"},
		{"url colon", gx.El("a", gx.Attrs{{Key: "href", Value: "mailto:a@b", Kind: gx.AttrURL}}), `<a href="mailto:a@b"></a>`},
		{"url fragment", gx.El("a", gx.Attrs{{Key: "href", Value: "#top", Kind: gx.AttrURL}}), `<a href="#top"></a>`},
		{"url crlf", gx.El("a", gx.Attrs{{Key: "href", Value: "a\r\nb", Kind: gx.AttrURL}}), `<a href="a%0D%0Ab"></a>`},
		{"url script", gx.El("a", gx.Attrs{{Key: "href", Value: "javascript:alert(1)", Kind: gx.AttrURL}}), `<a href="javascript:alert%281%29"></a>`},
		{"style semicolon", gx.El("div", gx.Attrs{{Key: "style", Value: "color:red;background:blue", Kind: gx.AttrStyle}}), `<div style="color:red;background:blue"></div>`},
		{"style tag", gx.El("div", gx.Attrs{{Key: "style", Value: "</style><script>", Kind: gx.AttrStyle}}), `<div style="&lt;/style&gt;&lt;script&gt;"></div>`},
	}
	for _, c := range cases {
		if got := gx.String(c.node); got != c.want {
			t.Errorf("%s: String = %q, want %q", c.name, got, c.want)
		}
	}
}

// FuzzSI_02_URLOutput checks that a URL attribute never lets its value leave
// the attribute or the element (SI-02).
func FuzzSI_02_URLOutput(f *testing.F) {
	f.Add("/a b?x=1&y=2")
	f.Add("javascript:alert(1)")
	f.Add("a\x00b")
	f.Add(`"><script>alert(1)</script>`)
	f.Fuzz(func(t *testing.T, s string) {
		out := gx.String(gx.El("a", gx.Attrs{{Key: "href", Value: s, Kind: gx.AttrURL}}))
		const prefix = `<a href="`
		const suffix = `"></a>`
		if !strings.HasPrefix(out, prefix) || !strings.HasSuffix(out, suffix) {
			t.Fatalf("output = %q", out)
		}
		value := out[len(prefix) : len(out)-len(suffix)]
		if strings.ContainsAny(value, `"<>`) {
			t.Fatalf("url value %q left the attribute: %q", s, out)
		}
	})
}

func TestREQ_AUT_12_Classes(t *testing.T) {
	if got, want := gx.Classes("a", gx.When("b", true), gx.When("c", false)), "a b"; got != want {
		t.Fatalf("Classes = %q, want %q", got, want)
	}
}
