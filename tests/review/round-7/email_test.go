package round7_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_REG_17_RawHTMLWithASignalAnActionOrAnIsland checks REQ-REG-17:
// "In a tree that `gx.Email` renders, a `class` attribute, a signal, a
// client expression, an action invocation, an island and a `script` element
// are errors: a compile error for a component of an `email` package where
// the compiler sees it, and an error result of `gx.Email` in each other
// case." The function has the name gx.RenderEmail.
//
// gx.RenderEmail finds a class attribute and a script element in the HTML
// that the tree renders, so it finds them in a gx.Raw node too. It finds a
// signal, a client expression, an action invocation and an island only in
// the attributes of element nodes. The same constructs in a gx.Raw node,
// for example the HTML of a Markdown body or of a stored template, give a
// message with no error.
func TestREQ_REG_17_RawHTMLWithASignalAnActionOrAnIsland(t *testing.T) {
	opt := gx.EmailOptions{BaseURL: "https://shop.example"}
	// The fixture: the two constructs that the function finds in raw HTML.
	for name, raw := range map[string]gx.Node{
		"a class attribute": gx.Raw(`<p class="note">x</p>`),
		"a script element":  gx.Raw(`<script>alert(1)</script>`),
	} {
		if _, err := gx.RenderEmail(raw, opt); err == nil {
			t.Fatalf("the fixture is wrong: raw HTML with %s gives no error", name)
		}
	}
	for name, raw := range map[string]gx.Node{
		"a signal":             gx.Raw(`<div data-signals="{&quot;open&quot;:false}">x</div>`),
		"a client expression":  gx.Raw(`<span data-text="$open">x</span>`),
		"an action invocation": gx.Raw(`<button data-on:click="@post('/cart/add')">Add</button>`),
		"an island":            gx.Raw(`<gx-island src="/_gx/islands/chart.js"></gx-island>`),
	} {
		msg, err := gx.RenderEmail(raw, opt)
		if err == nil || !strings.Contains(err.Error(), "GX6010") {
			t.Errorf("raw HTML with %s: err = %v, want the error of GX6010; the email is %s", name, err, msg.HTML)
		}
	}
}
