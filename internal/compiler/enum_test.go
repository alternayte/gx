package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_STY_05_EnumCoverage covers GX5001: a gx.Enum[T] literal that
// misses a constant of T is reported, and a complete map is clean
// (REQ-STY-05).
func TestREQ_STY_05_EnumCoverage(t *testing.T) {
	styles := `package card

import "github.com/alternayte/gx"

type Variant string

const (
	Default Variant = "default"
	Raised  Variant = "raised"
)

var variant = gx.Enum[Variant]{Default: "border"}
`
	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": styles,
	})
	diags := compiler.Check(dir)
	found := false
	for _, d := range diags {
		if d.Code == compiler.CodeEnum {
			found = true
			if !strings.Contains(d.Msg, "Raised") {
				t.Errorf("GX5001 message = %q", d.Msg)
			}
			if !strings.HasSuffix(d.File, "styles.go") || d.Line != 12 {
				t.Errorf("GX5001 position = %s:%d, want styles.go:12", d.File, d.Line)
			}
		}
	}
	if !found {
		t.Fatalf("no GX5001: %v", diags)
	}

	// The complete map is clean.
	complete := strings.Replace(styles, `gx.Enum[Variant]{Default: "border"}`,
		"gx.Enum[Variant]{Default: \"border\", Raised: \"border shadow\"}", 1)
	clean := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"ui/card/styles.go": complete,
	})
	for _, d := range compiler.Check(clean) {
		if d.Code == compiler.CodeEnum {
			t.Fatalf("complete map reported: %v", d)
		}
	}
}
