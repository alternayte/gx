package compiler

import (
	"strings"
	"testing"
)

// TestREQ_CNT_03_ComponentInListItem checks the body expression of a page
// with a component in a list item: the prose of the list is one Markdown
// document with a slot for the component, the children of the component
// lose the indent of the list item, and a named prop type of the package
// of the component has its qualifier.
func TestREQ_CNT_03_ComponentInListItem(t *testing.T) {
	src := "1. One\n\n   <docs.Card title=\"T\" icon=\"setting\">\n   ```sh\n   run\n   ```\n   </docs.Card>\n\n2. Two <docs.Card title=\"U\" /> end\n"
	nodes, diags := parseContentTree(src, 0)
	if len(diags) > 0 {
		t.Fatalf("parse: %v", diags)
	}
	w := &contentWriter{imports: map[string]string{}}
	w.coll = contentCollection{pkgPath: "app/site", comps: map[string]contentComp{
		"Card": {name: "Card", pkgPath: "app/docs", pkgName: "docs", propsType: "docs.CardProps", props: map[string]Prop{
			"title":    {Name: "Title", Type: "string"},
			"icon":     {Name: "Icon", Type: "IconName"},
			"children": {Name: "Children", Type: "gx.Node"},
		}},
	}}
	expr, err := w.nodes(nodes, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// One document: the block component is a comment at the indent of
		// the list item, the inline component is an element in its line.
		`gxContentSlots("1. One\n\n   <!--gx-slot:0-->\n\n2. Two <gx-slot n=\"1\"></gx-slot> end\n"`,
		`Icon: docs.IconName("setting")`,
		// The children have no indent, so the fence is a code block.
		`gxContentMarkdown("\n` + "```sh\\nrun\\n```\\n" + `")`,
	} {
		if !strings.Contains(expr, want) {
			t.Errorf("the body expression lacks %s:\n%s", want, expr)
		}
	}
	if !w.slots || w.imports["app/docs"] != "docs" {
		t.Errorf("slots = %v, imports = %v", w.slots, w.imports)
	}
}
