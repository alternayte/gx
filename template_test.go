package gx_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// cardTemplate is what the compiler writes for
//
//	<section {attrs}><h3>{title}</h3><ul id={id}>{rows}</ul><p>end</p></section>
//
// The section and the ul have their attributes as dynamic values.
var cardTemplate = gx.NewTemplate(
	[]string{"<section", "><h3>", "</h3><ul", ">", "</ul><p>end</p></section>"},
	[]int{0, 2, 1, 2},
	[]gx.TemplateEl{
		{Slot: 0, Start: 0, EndStatic: 4, End: len("</ul><p>end</p></section>")},
		{Slot: 2, Start: len("</h3>"), EndStatic: 4, End: len("</ul>")},
	},
	[]gx.TemplateRoot{{El: 0, Slot: -1}},
)

func cardValue(attrs gx.Attrs, title, listID string, rows gx.Node) gx.Node {
	return cardTemplate.With(
		gx.Open("section", attrs),
		gx.Text(title),
		gx.Open("ul", gx.Attrs{{Key: "id", Value: listID}}),
		rows,
	)
}

func cardTree(attrs gx.Attrs, title, listID string, rows gx.Node) gx.Node {
	return gx.El("section", attrs,
		gx.El("h3", nil, gx.Text(title)),
		gx.El("ul", gx.Attrs{{Key: "id", Value: listID}}, rows),
		gx.El("p", nil, gx.Text("end")),
	)
}

// TestREQ_AUT_21_TemplateValueBytes checks that a template value renders the
// bytes of the tree of elements, with a request in scope (REQ-AUT-21).
func TestREQ_AUT_21_TemplateValueBytes(t *testing.T) {
	attrs := gx.Attrs{
		{Key: "class", Value: `a "b" <c>`},
		{Key: "href", Value: "/home", Kind: gx.AttrURL, Active: "page"},
		gx.Bool("hidden", true),
		gx.Bool("open", false),
	}
	rows := gx.Frag(gx.El("li", nil, gx.Text("1 < 2")), gx.Value(7))
	req := httptest.NewRequest("GET", "/home", nil)
	want := gx.StringRequest(req, cardTree(attrs, "Fish & chips", "rows", rows))
	got := gx.StringRequest(req, cardValue(attrs, "Fish & chips", "rows", rows))
	if got != want {
		t.Fatalf("template value:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(got, `aria-current="page"`) {
		t.Fatalf("the fixture is wrong: no active link in %q", got)
	}
}

// templateSink keeps a value on the heap, as a caller of a component does.
var templateSink gx.Node

// TestREQ_AUT_21_FewValuesOneAllocation checks that a template value with
// few dynamic values is one allocation (DR-11).
func TestREQ_AUT_21_FewValuesOneAllocation(t *testing.T) {
	row := gx.NewTemplate([]string{"<li>", "</li>"}, []int{1}, nil, []gx.TemplateRoot{{El: -1, Slot: -1, Name: "li"}})
	text := gx.Text("x")
	allocs := testing.AllocsPerRun(100, func() { templateSink = row.With(text) })
	if allocs != 1 {
		t.Fatalf("a template value with one dynamic value made %v allocations, want 1", allocs)
	}
}

// TestREQ_AUT_22_HeadOfTemplateValue checks that the deepest title wins when
// the head nodes are dynamic values of templates (REQ-RTE-11).
func TestREQ_AUT_22_HeadOfTemplateValue(t *testing.T) {
	// <div>{deep}</div>{shallow}: the first value is one element deep.
	layout := gx.NewTemplate([]string{"<div>", "</div>", ""}, []int{1, 0}, nil,
		[]gx.TemplateRoot{{El: -1, Slot: -1, Name: "div"}, {El: -1, Slot: 1}})
	deep, shallow := gx.Head(gx.HeadProps{Title: "deep"}), gx.Head(gx.HeadProps{Title: "shallow"})
	for _, c := range []struct {
		name string
		n    gx.Node
		want string
	}{
		{"deep first", layout.With(deep, shallow), "deep"},
		{"deep last", layout.With(shallow, deep), "shallow"},
		{"tree", gx.Frag(gx.El("div", nil, deep), shallow), "deep"},
	} {
		if got := gx.HeadOf(c.n).Title; got != c.want {
			t.Errorf("%s: title %q, want %q", c.name, got, c.want)
		}
	}
}

// TestREQ_AUT_22_MarkersOfTemplateValue checks that the app reads the
// runtime markers of a template value from its open tags (NFR-04).
func TestREQ_AUT_22_MarkersOfTemplateValue(t *testing.T) {
	body := nfr04Serve(t, func() gx.Node {
		return cardValue(gx.Attrs{{Key: "data-gx-tabs", Value: "x"}}, "T", "rows", nil)
	})
	if !strings.Contains(body, "/_gx/tabs.js") {
		t.Errorf("a template value with a tabs marker ships no tabs module:\n%s", body)
	}
	plain := nfr04Serve(t, func() gx.Node { return cardValue(nil, "T", "rows", nil) })
	if strings.Contains(plain, "<script") {
		t.Errorf("a template value with no marker ships JS:\n%s", plain)
	}
}

// TestREQ_AUT_22_PatchOfTemplateValue checks the roots of a patch and the
// lookup of an element by its id on a template value (REQ-ACT-04,
// REQ-FRM-05).
func TestREQ_AUT_22_PatchOfTemplateValue(t *testing.T) {
	patch := func(n gx.Node) (*fakeAdapter, *httptest.ResponseRecorder) {
		a := &fakeAdapter{}
		rec := serveAction(t, a, gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Patch(n) }))
		return a, rec
	}
	rows := gx.El("li", nil, gx.Text("one"))

	// The root has an id: one patch with the HTML of the whole value.
	withID := cardValue(gx.Attrs{{Key: "id", Value: "card"}}, "T", "rows", rows)
	a, _ := patch(withID)
	p, ok := a.res.Patches[0].(gx.ElementPatch)
	if !ok || p.Target != "#card" {
		t.Fatalf("patch = %#v, want an element patch of #card", a.res.Patches[0])
	}
	if got, want := gx.String(p.Node), gx.String(withID); got != want {
		t.Errorf("root patch:\n got %q\nwant %q", got, want)
	}

	// The root has no id: the error of a tree.
	a, _ = patch(cardValue(nil, "T", "rows", rows))
	if toast, ok := a.res.Patches[0].(gx.ToastPatch); !ok || !strings.Contains(toast.Text, "<section> has no id") {
		t.Errorf("patch = %#v, want a toast about <section>", a.res.Patches[0])
	}

	// An element inside the value is a patch target by its id.
	inner := gx.ElementByID(withID, "rows")
	if inner == nil {
		t.Fatal("no element with the id rows")
	}
	if got, want := gx.String(inner), `<ul id="rows"><li>one</li></ul>`; got != want {
		t.Errorf("inner element:\n got %q\nwant %q", got, want)
	}
	a, _ = patch(inner)
	if p, ok := a.res.Patches[0].(gx.ElementPatch); !ok || p.Target != "#rows" {
		t.Errorf("patch = %#v, want an element patch of #rows", a.res.Patches[0])
	}
	if gx.ElementByID(withID, "none") != nil {
		t.Error("found an element for an id that no element has")
	}
}
