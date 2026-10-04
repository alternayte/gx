package gx_test

import (
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_STY_07_TransitionStyle covers gx.Transition and the sanitized
// view-transition style (REQ-STY-07).
func TestREQ_STY_07_TransitionStyle(t *testing.T) {
	hero := gx.Transition[int64]("product-image")
	got := string(gx.TransitionStyle(hero(42)))
	want := "view-transition-name: product-image-42; view-transition-class: product-image"
	if got != want {
		t.Fatalf("TransitionStyle = %q, want %q", got, want)
	}
	noKey := string(gx.TransitionStyle(hero(0)))
	if noKey != "view-transition-name: product-image-0; view-transition-class: product-image" {
		t.Fatalf("keyed zero = %q", noKey)
	}
	dirty := string(gx.TransitionStyle(gx.Transition[string]("product image!!")("/a b/")))
	if dirty != "view-transition-name: product-image-a-b; view-transition-class: product-image" {
		t.Fatalf("sanitized = %q", dirty)
	}
	joined := string(gx.StyleJoin(gx.Style("color: red"), gx.Style("")))
	if joined != "color: red" {
		t.Fatalf("StyleJoin = %q", joined)
	}
}
