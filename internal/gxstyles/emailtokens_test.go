package gxstyles

import (
	"flag"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gx "github.com/alternayte/gx"
)

var update = flag.Bool("update", false, "write the golden files")

// TestREQ_STY_14_ThemeTokens covers the generated tokens file: the golden
// file for the default theme, and the kinds of values (REQ-STY-14).
func TestREQ_STY_14_ThemeTokens(t *testing.T) {
	got := GenerateTokens([]byte(gx.DefaultThemeCSS))
	if formatted, err := format.Source(got); err != nil || string(formatted) != string(got) {
		t.Fatalf("the generated file is not gofmt source: %v", err)
	}
	golden := filepath.Join("testdata", "tokens_gx.go.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("the generated tokens differ from %s:\n%s", golden, got)
	}
	// No value is a CSS variable or an oklch colour.
	for _, bad := range []string{"var(", "oklch", "rem"} {
		if strings.Contains(string(got), bad) {
			t.Errorf("the generated file has %q", bad)
		}
	}

	light, dark := ThemeValues([]byte(gx.DefaultThemeCSS))
	for name, want := range map[string]string{
		"--background": "#ffffff", "--foreground": "#0a0a0a", "--primary": "#171717",
		"--primary-foreground": "#fafafa", "--border": "#e5e5e5", "--radius": "10px",
	} {
		if light[name] != want {
			t.Errorf("light %s = %q, want %q", name, light[name], want)
		}
	}
	// The dark border is white with an alpha of 10% on the dark background.
	for name, want := range map[string]string{
		"--background": "#0a0a0a", "--primary": "#e5e5e5", "--border": "#232323", "--radius": "10px",
	} {
		if dark[name] != want {
			t.Errorf("dark %s = %q, want %q", name, dark[name], want)
		}
	}

	light, dark = ThemeValues([]byte(`
@layer base { :root { --a: #abc; --b: rgb(255 0 0 / 50%); --c: hsl(120, 100%, 50%); --d: 1.5rem; --e: var(--a);
  --f: calc(var(--d) - 2px); --font: "Inter", sans-serif; --background: black; --g: oklch(62.8% 0.2577 29.23); } }
.dark, .other { --a: #000000; }
@media (prefers-color-scheme: dark) { :root:not(.light) { --c: white; } }
@theme inline { --color-a: var(--a); }
`))
	wantLight := map[string]string{"--a": "#aabbcc", "--b": "#800000", "--c": "#00ff00", "--d": "24px", "--e": "#aabbcc", "--background": "#000000", "--g": "#ff0000"}
	if len(light) != len(wantLight) {
		t.Errorf("light = %v", light)
	}
	for name, want := range wantLight {
		if light[name] != want {
			t.Errorf("light %s = %q, want %q", name, light[name], want)
		}
	}
	if dark["--a"] != "#000000" || dark["--c"] != "#ffffff" || dark["--d"] != "24px" || dark["--e"] != "#000000" {
		t.Errorf("dark = %v", dark)
	}
}
