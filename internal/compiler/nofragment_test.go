package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// TestREQ_ACT_16_NoFragmentMark checks what the compiler writes for a
// fragment in a branch: each other branch has the mark gx.NoFragment with
// the id of the fragment, so c.Update can remove a fragment that the new
// props do not give (REQ-ACT-16, B-019). A fragment in a loop and a
// fragment with a key of its own have no mark: the id is not the same
// outside the branch.
func TestREQ_ACT_16_NoFragmentMark(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"ui/box/Box.gx": `package box

props {
  Total string
  Note  string
  Kind  int
  Items []string
}

<div id="box">
  <span #total>{p.Total}</span>
  if p.Note != "" {
    <p #note>{p.Note}</p>
  }
  if p.Kind > 0 {
    <b #plus>more</b>
  } else {
    <i #minus>less</i>
  }
  switch p.Kind {
  case 1:
    <em #one>one</em>
  case 2:
    <em #two>two</em>
  }
  if len(p.Items) > 0 {
    for _, it := range p.Items {
      <u #item(it string) key={it}>{it}</u>
    }
  }
</div>
`,
	})
	files, diags := compiler.NewSession().Generate(dir)
	if compiler.Failed(diags) {
		t.Fatalf("generate: %v", diags)
	}
	src := string(files[filepath.Join(dir, "ui", "box", "Box_gx.go")])
	if src == "" {
		t.Fatal("no generated file")
	}
	for _, id := range []string{"box-note", "box-plus", "box-minus", "box-one", "box-two"} {
		if !strings.Contains(src, `gx.NoFragment("`+id+`")`) {
			t.Errorf("the generated code has no gx.NoFragment(%q):\n%s", id, src)
		}
	}
	// A switch with no default gets one: it has the mark of each case.
	if !strings.Contains(src, "default:") {
		t.Errorf("the switch has no default branch with the marks:\n%s", src)
	}
	// The case 1 has the mark of #two and not of its own fragment.
	one := src[strings.Index(src, "case 1:"):strings.Index(src, "case 2:")]
	if !strings.Contains(one, `gx.NoFragment("box-two")`) || strings.Contains(one, `gx.NoFragment("box-one")`) {
		t.Errorf("the case 1 has the wrong marks:\n%s", one)
	}
	for _, bad := range []string{`NoFragment("box-total")`, "NoFragment(gx.FragmentID", `NoFragment("box-item`} {
		if strings.Contains(src, bad) {
			t.Errorf("the generated code has %s:\n%s", bad, src)
		}
	}
}
