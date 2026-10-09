package content_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/content"
)

// TestREQ_CNT_03_BodySlots checks that a component between two prose chunks
// stays in the Markdown block that holds it: a component in a list item is
// in that list item, and the list is one list.
func TestREQ_CNT_03_BodySlots(t *testing.T) {
	md := "1. One\n\n   <!--gx-slot:0-->\n\n2. Two <gx-slot n=\"1\"></gx-slot> end\n\n<gx-slot n=\"2\"></gx-slot>\n"
	node, err := content.BodySlots([]byte(md), gx.Text("[A]"), gx.Text("[B]"), gx.Text("[C]"))
	if err != nil {
		t.Fatal(err)
	}
	out := gx.String(node)
	if n := strings.Count(out, "<ol"); n != 1 {
		t.Fatalf("the output has %d lists, want 1:\n%s", n, out)
	}
	first := out[strings.Index(out, "<li>"):strings.Index(out, "</li>")]
	if !strings.Contains(first, "One") || !strings.Contains(first, "[A]") {
		t.Errorf("slot 0 is not in the first list item:\n%s", out)
	}
	if !strings.Contains(out, "Two [B] end") {
		t.Errorf("slot 1 is not inside its line:\n%s", out)
	}
	// A slot that is the only content of a paragraph has no paragraph.
	if strings.Contains(out, "<p>[C]</p>") || !strings.Contains(out, "[C]") {
		t.Errorf("slot 2 is in a paragraph or is missing:\n%s", out)
	}
	if strings.Contains(out, "gx-slot") {
		t.Errorf("a slot marker is in the output:\n%s", out)
	}
}
