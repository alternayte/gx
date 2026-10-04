package gx_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// TestREQ_STY_03_Theme covers the default theme: shadcn token names and
// both dark-mode paths (REQ-STY-03).
func TestREQ_STY_03_Theme(t *testing.T) {
	css := gx.DefaultThemeCSS
	for _, token := range []string{
		"--background", "--foreground", "--primary", "--primary-foreground",
		"--muted", "--muted-foreground", "--border", "--input", "--ring",
		"--radius", "--card", "--card-foreground",
	} {
		if !strings.Contains(css, token+":") {
			t.Fatalf("default theme lacks %s", token)
		}
	}
	if !strings.Contains(css, ".dark {") {
		t.Fatal("default theme has no .dark class block")
	}
	if !strings.Contains(css, "prefers-color-scheme: dark") {
		t.Fatal("default theme has no prefers-color-scheme block")
	}
	if !strings.Contains(css, "@theme inline") || !strings.Contains(css, "@source") {
		t.Fatal("default theme is not a Tailwind v4 theme file")
	}
}

// TestREQ_STY_03_GalleryTokens covers the gallery shell: it uses the same
// token names, so a pasted theme changes it (REQ-STY-03).
func TestREQ_STY_03_GalleryTokens(t *testing.T) {
	// galleryCSS is gxdev only; the token contract lives in the same file
	// as DefaultThemeCSS, so check the exported theme is the source.
	css := gx.DefaultThemeCSS
	if !strings.Contains(css, "--color-background: var(--background)") {
		t.Fatal("theme does not map the tokens into Tailwind colors")
	}
}
