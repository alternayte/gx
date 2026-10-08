package gxstyles

import (
	"strings"
	"testing"
)

// TestREQ_ISL_13_WidgetTokens checks the CSS variables of a widget: the
// tokens of the host element that the stylesheet reads, also through a
// different token.
func TestREQ_ISL_13_WidgetTokens(t *testing.T) {
	css := `@layer properties{*,:before,:after,::backdrop{--tw-shadow:0 0 #0000}}` +
		`@layer theme{:host{--spacing:.25rem;--color-primary:var(--primary)}}` +
		`@layer base{*{border-color:var(--border)}}` +
		`@layer utilities{.p-4{padding:calc(var(--spacing)*4)}.bg-primary{background-color:var(--color-primary)}.rounded-xl{border-radius:calc(var( --radius ) + 4px)}}` +
		`:host{--primary:oklch(20.5% 0 0);--border:var(--line);--line:#ddd;--radius:.5rem;--muted:#eee;--sidebar:var(--muted);display:block;color:var(--text)}` +
		`:host(.dark){--primary:oklch(92.2% 0 0);--text:#fff}` +
		`@media (prefers-color-scheme:dark){:host(:not(.light)){--primary:var(--inverse)}}` +
		`:host,.card{--card:#fff}.card:after{content:"} var(--muted)";color:var(--card)}` +
		`:host{--inverse:#fff}`
	got := strings.Join(WidgetTokens([]byte(css)), " ")
	// --muted and --sidebar have no reader. --card is not a token of the
	// host element only. --spacing and --tw-shadow are in a layer.
	if want := "--border --inverse --line --primary --radius --text"; got != want {
		t.Errorf("tokens = %s, want %s", got, want)
	}
	if got := WidgetTokens(nil); len(got) != 0 {
		t.Errorf("tokens of no stylesheet = %v", got)
	}
}
