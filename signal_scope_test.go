package gx_test

import (
	"testing"

	"github.com/alternayte/gx"
	"net/http"
	"net/http/httptest"
)

func TestREQ_ACT_05_SignalJSON(t *testing.T) {
	got := gx.SignalJSON("cart.Cart", "42", map[string]any{"qty": 2, "open": false})
	want := `{"cart":{"Cart":{"42":{"open":false,"qty":2}}}}`
	if got != want {
		t.Fatalf("SignalJSON = %s, want %s", got, want)
	}
	unkeyed := gx.SignalJSON("cart.Cart", "", map[string]any{"qty": 1})
	if unkeyed != `{"cart":{"Cart":{"qty":1}}}` {
		t.Fatalf("unkeyed SignalJSON = %s", unkeyed)
	}
}

func TestREQ_ACT_06_SignalPath(t *testing.T) {
	if got, want := gx.SignalPath("cart.Cart", "42", "qty"), `$["cart"]["Cart"]["42"]["qty"]`; got != want {
		t.Fatalf("SignalPath = %s, want %s", got, want)
	}
	if got, want := gx.SignalPath("cart.Cart", "", "qty"), `$["cart"]["Cart"]["qty"]`; got != want {
		t.Fatalf("unkeyed SignalPath = %s, want %s", got, want)
	}
	if got, want := gx.ScopeString("cart.Cart", "42"), "cart.Cart.42"; got != want {
		t.Fatalf("ScopeString = %q, want %q", got, want)
	}
	if got, want := gx.ChildKey("shop.Home", 1), gx.Key("shop.Home.1"); got != want {
		t.Fatalf("ChildKey = %q, want %q", got, want)
	}
	if got, want := gx.ChildKey("", 0), gx.Key("0"); got != want {
		t.Fatalf("root ChildKey = %q, want %q", got, want)
	}
}

func TestREQ_ACT_14_FragmentID(t *testing.T) {
	if got, want := gx.FragmentID("cart", "total", ""), "cart-total"; got != want {
		t.Fatalf("FragmentID = %q, want %q", got, want)
	}
	if got, want := gx.FragmentID("cart", "row", "42"), "cart-row-42"; got != want {
		t.Fatalf("keyed FragmentID = %q, want %q", got, want)
	}
}

// TestREQ_DEV_03_ProdIgnoresReloadHeader covers the dev reload header in a
// production build: a request that holds it still gets the first values of
// the signals, so a client cannot keep stale values with a header
// (REQ-DEV-03, SI-08). The browser suite covers the dev build.
func TestREQ_DEV_03_ProdIgnoresReloadHeader(t *testing.T) {
	node := gx.El("div", gx.Attrs{{Key: "data-signals", Value: `{"a":1}`}}, gx.Text("x"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Gx-Dev-Reload", "1")
	if got, want := gx.StringRequest(r, node), `<div data-signals="{&#34;a&#34;:1}">x</div>`; got != want {
		t.Fatalf("render = %s, want %s", got, want)
	}
}
