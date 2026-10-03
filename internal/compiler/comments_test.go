package compiler_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

func TestREQ_AUT_16_Comments(t *testing.T) {
	f, diags := compiler.ParseFragment([]byte("<div>{/* drop */}<!-- keep --></div>"))
	if len(diags) > 0 || f == nil {
		t.Fatalf("diagnostics = %v", diags)
	}
	el, ok := f.Body[0].(*compiler.Element)
	if !ok {
		t.Fatalf("body[0] is %T, want element", f.Body[0])
	}
	if len(el.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(el.Children))
	}
	if _, ok := el.Children[0].(*compiler.Comment); !ok {
		t.Fatalf("child 0 is %T, want Comment", el.Children[0])
	}
	if _, ok := el.Children[1].(*compiler.HTMLComment); !ok {
		t.Fatalf("child 1 is %T, want HTMLComment", el.Children[1])
	}

	visible := compiler.VisibleNodes(el.Children)
	if len(visible) != 1 {
		t.Fatalf("visible nodes = %d, want 1", len(visible))
	}
	if _, ok := visible[0].(*compiler.HTMLComment); !ok {
		t.Fatalf("visible[0] is %T, want HTMLComment", visible[0])
	}

	out := string(compiler.Format(f))
	if !strings.Contains(out, "{/* drop */}") {
		t.Errorf("formatted output dropped the gx comment:\n%s", out)
	}
	if !strings.Contains(out, "<!-- keep -->") {
		t.Errorf("formatted output dropped the HTML comment:\n%s", out)
	}
}
