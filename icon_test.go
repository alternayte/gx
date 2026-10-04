package gx_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_STY_06_IconRender covers the icon runtime: inline svg with
// currentColor, aria-hidden by default and a label as role=img
// (REQ-STY-06).
func TestREQ_STY_06_IconRender(t *testing.T) {
	got := gx.String(gx.Icon(`<path d="M1 1"/>`, gx.IconProps{Class: "size-4"}))
	for _, want := range []string{
		"<svg", `viewBox="0 0 24 24"`, `stroke="currentColor"`,
		`aria-hidden="true"`, `class="size-4"`, `<path d="M1 1"/>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("icon lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "role=") {
		t.Fatalf("decorative icon has a role:\n%s", got)
	}

	labeled := gx.String(gx.Icon(`<circle cx="1" cy="1" r="1"/>`, gx.IconProps{Label: "Cart"}))
	if !strings.Contains(labeled, `role="img"`) || !strings.Contains(labeled, `aria-label="Cart"`) {
		t.Fatalf("labeled icon lacks role or label:\n%s", labeled)
	}
	if strings.Contains(labeled, "aria-hidden") {
		t.Fatalf("labeled icon is hidden:\n%s", labeled)
	}
}
