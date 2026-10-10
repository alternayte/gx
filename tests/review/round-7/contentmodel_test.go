package round7_test

import "testing"

// REQ-AUT-23: "`gx check` reports content model defects as errors: ... an
// element in a parent where the HTML parser moves or drops it." Acceptance:
// "One compile-fail test for each defect, GX2016 to GX2020."
//
// The check of GX2020 has a table of child elements (li, tr, td, option and
// so on) and the rule of a block element in a p. It has no rule for the
// content of a table. The parser of a browser moves an element that is not
// table content to the place before the table (foster parenting): the page
// shows the div above the table, and a morph of the row then finds no div
// in it.

// TestREQ_AUT_23_BlockElementInATable checks a div that is a child of a tr
// and a div that is a child of a table.
func TestREQ_AUT_23_BlockElementInATable(t *testing.T) {
	for name, body := range map[string]string{
		"a div in a row":   "<table><tbody><tr><div>Box</div></tr></tbody></table>",
		"a div in a table": "<table><div>Box</div></table>",
	} {
		if got := viewFindings(t, body, nil); !has(got, "GX2020") {
			t.Errorf("%s: findings [%s], want GX2020: a browser moves the <div> to the place before the table", name, joined(got))
		}
	}
}

// TestREQ_AUT_23_FormInsideAForm checks a form inside a form. The parser of
// a browser drops the start tag of the inner form: its action and its method
// are gone, and its button submits the outer form.
func TestREQ_AUT_23_FormInsideAForm(t *testing.T) {
	body := "<form method=\"post\" action=\"/outer\"><div><form method=\"post\" action=\"/inner\"><button type=\"submit\">Go</button></form></div></form>"
	if got := viewFindings(t, body, nil); !has(got, "GX2020") {
		t.Errorf("a form inside a form: findings [%s], want GX2020: a browser drops the inner <form>", joined(got))
	}
}

// TestREQ_AUT_23_BlockInALinkInAParagraph checks a div inside a link inside
// a p. The check of a block element in a p stops at an <a>, but an <a> is
// no boundary for the parser: the start tag of the div ends the paragraph
// (the p is in button scope), and the div is then a sibling of the p. The
// same div with a span around it in the place of the link has the finding.
func TestREQ_AUT_23_BlockInALinkInAParagraph(t *testing.T) {
	if got := viewFindings(t, `<p>Text <span><div>Box</div></span></p>`, nil); !has(got, "GX2020") {
		t.Fatalf("the fixture is wrong: a div in a span in a p has the findings [%s]", joined(got))
	}
	if got := viewFindings(t, `<p>Text <a href="/x"><div>Box</div></a></p>`, nil); !has(got, "GX2020") {
		t.Errorf("a div in a link in a p: findings [%s], want GX2020: a browser ends the paragraph before the <div>", joined(got))
	}
}

// TestREQ_AUT_23_StaticIDInALoop checks REQ-AUT-23: "two elements with the
// same static `id` in one component" is an error (GX2019).
//
// The check counts the elements of the source with one id. An element with
// a static id in the body of a loop is one element of the source and one
// element of the page for each turn of the loop: each render with two items
// has two elements with the id "row". The check records that the element is
// in a loop (branchStep.loop) and never reads the mark.
func TestREQ_AUT_23_StaticIDInALoop(t *testing.T) {
	body := "<ul>\n  for _, it := range p.Items {\n    <li id=\"row\">{it}</li>\n  }\n</ul>"
	if got := viewFindings(t, body, nil); !has(got, "GX2019") {
		t.Errorf("a static id in a loop: findings [%s], want GX2019: each turn of the loop writes an element with the id \"row\"", joined(got))
	}
}

// TestREQ_AUT_23_ControlWithALabelOfAComponent checks REQ-AUT-23: "a form
// control with no label" is an error (GX2017), and the acceptance sentence
// "The registry and each example app pass."
//
// The input is the child of a component, and the component puts its
// children inside a <label>: the control has a label in each render. The
// check reports GX2017 as an error, so `gx check` fails for correct markup.
// The same check gives no finding for an element whose parent is in a
// different component in each other rule (a <li> as the child of a
// component, a <li> at the top of a component).
func TestREQ_AUT_23_ControlWithALabelOfAComponent(t *testing.T) {
	got := viewFindings(t, `<Field><input name="q"></Field>`, map[string]string{
		"ui/x/Field.gx": "package x\n\nprops {\n  Children gx.Node\n}\n\n<label>Name {p.Children}</label>\n",
	})
	if has(got, "GX2017") {
		t.Errorf("an input inside a component that writes the <label>: findings [%s], want none: the control has a label", joined(got))
	}
}
