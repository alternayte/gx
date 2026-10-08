package gxstyles

import (
	"strings"
	"testing"
)

// sample has the forms of a Tailwind 4.3 build with the default theme, cut
// to one declaration each.
const sample = `/*! tailwindcss v4.3.3 | MIT License | https://tailwindcss.com */
@layer properties{@supports (((-webkit-hyphens:none)) and (not (margin-trim:inline))) or ((-moz-orient:inline) and (not (color:rgb(from red r g b)))){*,:before,:after,::backdrop{--tw-shadow:0 0 #0000;--tw-ring-color:initial}}}@layer theme{:root,:host{--spacing:.25rem}}@layer base{*{border-color:var(--border)}}@layer utilities{.shadow-lg{box-shadow:var(--tw-ring-shadow),var(--tw-shadow)}.dark\:bg-foreground:where(.dark,.dark *){background-color:var(--foreground)}}@view-transition{navigation:auto}:root{--primary:oklch(20.5% 0 0)}.dark{--primary:oklch(92.2% 0 0)}@media (prefers-color-scheme:dark){:root:not(.light){--primary:oklch(92.2% 0 0)}}@property --tw-shadow{syntax:"*";inherits:false;initial-value:0 0 #0000}`

// TestREQ_ISL_11_ShadowCSS checks the changes that make a Tailwind build
// work in a shadow root: the start values of the properties, the theme
// tokens on the host element, and dark mode from a class of the host
// element.
func TestREQ_ISL_11_ShadowCSS(t *testing.T) {
	got, err := ShadowCSS([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	css := string(got)
	for _, want := range []string{
		// A browser does not read @property in a shadow root, so the start
		// values hold with no @supports condition.
		`@layer properties{*,:before,:after,::backdrop{--tw-shadow:0 0 #0000;--tw-ring-color:initial}}`,
		// The tokens of the theme are on the host element.
		`}:host{--primary:oklch(20.5% 0 0)}`,
		`:host(.dark){--primary:oklch(92.2% 0 0)}`,
		`@media (prefers-color-scheme:dark){:host(:not(.light)){--primary:oklch(92.2% 0 0)}}`,
		// A dark: class follows the class of the host element.
		`.dark\:bg-foreground:where(.dark,.dark *,:host(.dark) *){background-color:var(--foreground)}`,
		// The rule of Tailwind for its own variables stays.
		`@layer theme{:root,:host{--spacing:.25rem}}`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("the shadow CSS has no %s\n%s", want, css)
		}
	}
	for _, bad := range []string{"@supports", "}:root{", "}.dark{", ":root:not"} {
		if strings.Contains(css, bad) {
			t.Errorf("the shadow CSS still holds %q\n%s", bad, css)
		}
	}
}

// TestREQ_ISL_11_ShadowCSSNeedsTheStartValues checks that a build with
// @property and no block of start values is an error, not a widget with no
// shadows.
func TestREQ_ISL_11_ShadowCSSNeedsTheStartValues(t *testing.T) {
	_, err := ShadowCSS([]byte(`.shadow-lg{box-shadow:var(--tw-shadow)}@property --tw-shadow{syntax:"*";inherits:false}`))
	if err == nil {
		t.Fatal("no error for a build with @property and no start values")
	}
	if _, err := ShadowCSS([]byte(`.p-4{padding:1rem}`)); err != nil {
		t.Fatalf("a build with no @property: %v", err)
	}
}

// TestREQ_ISL_11_WidgetTheme checks the theme input of one widget: Tailwind
// reads the class list of the widget and no other source.
func TestREQ_ISL_11_WidgetTheme(t *testing.T) {
	theme := "@import \"tailwindcss\";\n\n@source \"../.gx/classes.txt\";\n@source not \"../gxstyles\";\n\n:root {\n  --primary: red;\n}\n"
	got, err := widgetTheme([]byte(theme), "/app/.gx/widgets/acme-cart.txt")
	if err != nil {
		t.Fatal(err)
	}
	css := string(got)
	if !strings.Contains(css, `@import "tailwindcss" source(none);`) {
		t.Errorf("Tailwind still looks for sources by itself:\n%s", css)
	}
	if !strings.Contains(css, `@source "/app/.gx/widgets/acme-cart.txt";`) || strings.Contains(css, "classes.txt") {
		t.Errorf("the source is not the class list of the widget:\n%s", css)
	}
	if !strings.Contains(css, "--primary: red;") {
		t.Errorf("the tokens of the theme are gone:\n%s", css)
	}
	if _, err := widgetTheme([]byte(":root { --primary: red; }\n"), "/x.txt"); err == nil {
		t.Error("no error for a theme with no import of tailwindcss")
	}
}
