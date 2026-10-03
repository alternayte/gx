package gx_test

import (
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
