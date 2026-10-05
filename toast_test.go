package gx_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx"
)

// TestREQ_REG_11_ToastOptions checks that every option reaches the patch,
// and that a kind is an option.
func TestREQ_REG_11_ToastOptions(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Toast("Saved", gx.ToastSuccess,
			gx.ToastDescription("All changes are stored."),
			gx.ToastID("save"),
			gx.ToastDuration(8*time.Second),
			gx.ToastSticky,
			gx.ToastLink("Open", redirectTarget{}))
	})
	_ = serveAction(t, a, h)
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one patch", a.res)
	}
	got, ok := a.res.Patches[0].(gx.ToastPatch)
	if !ok {
		t.Fatalf("patch is %T, want gx.ToastPatch", a.res.Patches[0])
	}
	want := gx.ToastPatch{
		Text:        "Saved",
		Kind:        gx.ToastSuccess,
		Description: "All changes are stored.",
		ID:          "save",
		Duration:    8 * time.Second,
		Sticky:      true,
		Action:      gx.ToastControl{Label: "Open", URL: "/home"},
	}
	if got != want {
		t.Fatalf("patch = %+v, want %+v", got, want)
	}
}

// actionTarget is a hand-written action route. Generated route types provide
// the same Pattern and URL methods.
type actionTarget struct{ ID int }

func (actionTarget) Pattern() string { return "POST /undo/{id}" }
func (actionTarget) URL() string     { return "/undo/7" }

// TestREQ_REG_11_ToastOneControl checks that ToastAction keeps the method
// and the URL of the route value, and that the later of two controls wins.
func TestREQ_REG_11_ToastOneControl(t *testing.T) {
	link := gx.ToastLink("Open", redirectTarget{})
	action := gx.ToastAction("Undo", actionTarget{ID: 7})
	cases := []struct {
		name string
		opts []gx.ToastOption
		want gx.ToastControl
	}{
		{"action", []gx.ToastOption{action}, gx.ToastControl{Label: "Undo", URL: "/undo/7", Method: "POST"}},
		{"action wins", []gx.ToastOption{link, action}, gx.ToastControl{Label: "Undo", URL: "/undo/7", Method: "POST"}},
		{"link wins", []gx.ToastOption{action, link}, gx.ToastControl{Label: "Open", URL: "/home"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := &fakeAdapter{}
			_ = serveAction(t, a, gx.Action(func(c *gx.Ctx, in actRoute) error { return c.Toast("Removed", tt.opts...) }))
			got, ok := a.res.Patches[0].(gx.ToastPatch)
			if !ok || got.Action != tt.want {
				t.Fatalf("patch = %+v, want the control %+v", a.res.Patches[0], tt.want)
			}
		})
	}
}

// TestREQ_REG_11_ToastActionMarkup checks the plain toast of an action: one
// button that the adapter makes invoke the action, and that closes the
// toast. It holds no link.
func TestREQ_REG_11_ToastActionMarkup(t *testing.T) {
	old := gx.AdapterOf(nil)
	defer gx.SetAdapter(old)
	gx.SetAdapter(&fakeAdapter{})
	p := gx.ToastPatch{Text: "Removed", Action: gx.ToastControl{Label: "Undo", URL: "/undo/7", Method: "POST"}}
	html := gx.String(gx.ToastNode(p))
	want := `<button type="button" data-gx-close="" data-fake-on="fake(POST /undo/7 )">Undo</button>`
	if !strings.Contains(html, want) {
		t.Fatalf("toast lacks %s:\n%s", want, html)
	}
	if strings.Contains(html, "<a ") {
		t.Fatalf("an action toast holds a link:\n%s", html)
	}
}

// TestREQ_REG_11_ErrorToastKind checks that the toast of a failed action has
// the error kind.
func TestREQ_REG_11_ErrorToastKind(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return errors.New("boom") })
	_ = serveAction(t, a, h)
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one toast", a.res)
	}
	p, ok := a.res.Patches[0].(gx.ToastPatch)
	if !ok || p.Kind != gx.ToastError || p.Text != "boom" {
		t.Fatalf("patch = %#v, want an error toast with the handler error", a.res.Patches[0])
	}
}

// TestREQ_REG_11_ToastMarkup checks the contract between the toast markup
// and the behaviour runtime: the role, the element id of a named toast and
// the time before the toast leaves.
func TestREQ_REG_11_ToastMarkup(t *testing.T) {
	cases := []struct {
		name  string
		patch gx.ToastPatch
		want  []string
		not   string
	}{
		{"default", gx.ToastPatch{Text: "Saved"}, []string{`role="status"`, `data-kind="default"`, `data-duration="4000"`, `data-gx-close`}, ` id=`},
		{"error", gx.ToastPatch{Text: "Failed", Kind: gx.ToastError}, []string{`role="alert"`, `data-kind="error"`}, `role="status"`},
		{"named", gx.ToastPatch{Text: "Saved", ID: "save"}, []string{`id="gx-toast-save"`}, ""},
		{"duration", gx.ToastPatch{Text: "Saved", Duration: 1500 * time.Millisecond}, []string{`data-duration="1500"`}, ""},
		{"loading stays", gx.ToastPatch{Text: "Saving", Kind: gx.ToastLoading, Duration: time.Second}, []string{`data-duration="0"`}, ""},
		{"sticky stays", gx.ToastPatch{Text: "Saved", Sticky: true}, []string{`data-duration="0"`}, ""},
		{"action", gx.ToastPatch{Text: "Saved", Action: gx.ToastControl{Label: "Open", URL: "/home"}}, []string{`<a href="/home">Open</a>`}, ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			html := gx.String(gx.ToastNode(tt.patch))
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Fatalf("toast lacks %s:\n%s", want, html)
				}
			}
			if tt.not != "" && strings.Contains(html, tt.not) {
				t.Fatalf("toast has %s:\n%s", tt.not, html)
			}
		})
	}
}

// TestREQ_REG_11_ToasterLoadsBehavior checks that a page with the toaster
// region ships the behaviour runtime, which runs the toast timers.
func TestREQ_REG_11_ToasterLoadsBehavior(t *testing.T) {
	body := servePage(t, &nfr04Adapter{}, gx.Toaster)
	if !strings.Contains(body, `id="gx-toaster"`) || !strings.Contains(body, `aria-live="polite"`) {
		t.Fatalf("page lacks the toaster region:\n%s", body)
	}
	if !strings.Contains(body, "/_gx/behavior.js") {
		t.Fatalf("toaster page lacks the behaviour runtime:\n%s", body)
	}
}
