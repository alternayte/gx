package gx_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// fakeAdapter records the response of an action (REQ-ACT-01). The Datastar
// adapter contract tests live in adapters/datastar.
type fakeAdapter struct {
	responded bool
	res       *gx.Response
	wrote     int
}

func (f *fakeAdapter) Name() string     { return "fake" }
func (f *fakeAdapter) Signals() bool    { return true }
func (f *fakeAdapter) Runtime() gx.Node { return nil }
func (f *fakeAdapter) Assets() map[string][]byte {
	return nil
}

func (f *fakeAdapter) Respond(w http.ResponseWriter, _ *http.Request, res *gx.Response) error {
	f.responded = true
	f.res = res
	status := res.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	f.wrote++
	return nil
}

func (f *fakeAdapter) ReadSignals(*http.Request, any) error { return nil }

// Invoke writes its arguments in a made-up syntax, so a test can tell the
// adapter that wrote an invocation.
func (f *fakeAdapter) Invoke(method, url, scope string) gx.Attr {
	return gx.Attr{Key: "data-fake-on", Value: "fake(" + method + " " + url + " " + scope + ")"}
}

// On writes the event and the URL in a made-up syntax.
func (f *fakeAdapter) On(inv gx.Invocation) []gx.Attr {
	return []gx.Attr{{Key: "data-fake-on-" + inv.Event, Value: inv.Method + " " + inv.URL}}
}

// actRoute is a hand-written action input for unit tests. Generated route
// types provide the same Pattern and Bind methods.
type actRoute struct{}

func (actRoute) Pattern() string          { return "POST /act" }
func (actRoute) Bind(*http.Request) error { return nil }

// redirectTarget implements the redirect target interface.
type redirectTarget struct{}

func (redirectTarget) URL() string { return "/home" }

func serveAction(t *testing.T, a *fakeAdapter, h gx.Handler) *httptest.ResponseRecorder {
	t.Helper()
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(h))
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest("POST", "/act", nil))
	return rec
}

func TestREQ_ACT_01_PatchAnswer(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2")))
	})
	rec := serveAction(t, a, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one patch", a.res)
	}
	p, ok := a.res.Patches[0].(gx.ElementPatch)
	if !ok {
		t.Fatalf("patch is %T, want gx.ElementPatch", a.res.Patches[0])
	}
	if p.Target != "#count" || p.Mode != gx.ModeMorph {
		t.Fatalf("patch = %+v, want morph #count", p)
	}
	if got := gx.String(p.Node); got != `<span id="count">2</span>` {
		t.Fatalf("patch node = %q", got)
	}
}

func TestREQ_ACT_01_SetSignalsAnswer(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.SetSignals(map[string]int{"qty": 2})
	})
	app := gx.New(gx.Config{Adapter: a})
	app.Group("/", gx.Collect(h))
	req := httptest.NewRequest("POST", "/act", nil)
	req.Header.Set("Gx-Scope", "cart.42")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one patch", a.res)
	}
	p, ok := a.res.Patches[0].(gx.SignalPatch)
	if !ok {
		t.Fatalf("patch is %T, want gx.SignalPatch", a.res.Patches[0])
	}
	if p.Scope != "cart.42" {
		t.Fatalf("scope = %q, want cart.42", p.Scope)
	}
	if m, ok := p.Signals.(map[string]int); !ok || m["qty"] != 2 {
		t.Fatalf("signals = %#v", p.Signals)
	}
}

func TestREQ_ACT_01_RedirectAnswer(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Redirect(redirectTarget{})
	})
	rec := serveAction(t, a, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %q", rec.Code, rec.Body.String())
	}
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one patch", a.res)
	}
	p, ok := a.res.Patches[0].(gx.RedirectPatch)
	if !ok {
		t.Fatalf("patch is %T, want gx.RedirectPatch", a.res.Patches[0])
	}
	if p.URL != "/home" {
		t.Fatalf("redirect URL = %q, want /home", p.URL)
	}
}

func TestREQ_ACT_01_ToastAnswer(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		return c.Toast("Saved")
	})
	_ = serveAction(t, a, h)
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one patch", a.res)
	}
	p, ok := a.res.Patches[0].(gx.ToastPatch)
	if !ok {
		t.Fatalf("patch is %T, want gx.ToastPatch", a.res.Patches[0])
	}
	if p.Text != "Saved" {
		t.Fatalf("toast = %q, want Saved", p.Text)
	}
}

func TestREQ_ACT_01_NoAnswer(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return nil })
	rec := serveAction(t, a, h)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if a.responded {
		t.Fatal("adapter responded to an action with no answer")
	}
}

func TestREQ_ACT_01_ActionError(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(c *gx.Ctx, in actRoute) error { return errors.New("boom") })
	rec := serveAction(t, a, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with a toast stream", rec.Code)
	}
	if a.res == nil || a.res.Err == nil {
		t.Fatal("handler error is not recorded for the dev overlay")
	}
	if !a.responded || len(a.res.Patches) != 1 {
		t.Fatalf("adapter got %+v, want one toast", a.res)
	}
	if p, ok := a.res.Patches[0].(gx.ToastPatch); !ok || p.Text == "" {
		t.Fatalf("patch = %#v, want a toast", a.res.Patches[0])
	}
}

func TestREQ_ACT_04_PatchModes(t *testing.T) {
	cases := []struct {
		name string
		arg  gx.Node
		want gx.PatchMode
	}{
		{"default", nil, gx.ModeMorph},
		{"append", gx.Append, gx.ModeAppend},
		{"prepend", gx.Prepend, gx.ModePrepend},
		{"replace", gx.Replace, gx.ModeReplace},
		{"remove", gx.Remove, gx.ModeRemove},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &fakeAdapter{}
			h := gx.Action(func(ctx *gx.Ctx, in actRoute) error {
				node := gx.El("span", gx.Attrs{{Key: "id", Value: "count"}}, gx.Text("2"))
				if c.arg == nil {
					return ctx.Patch(node)
				}
				return ctx.Patch(c.arg, node)
			})
			serveAction(t, a, h)
			if len(a.res.Patches) != 1 {
				t.Fatalf("patches = %+v", a.res.Patches)
			}
			p := a.res.Patches[0].(gx.ElementPatch)
			if p.Mode != c.want || p.Target != "#count" {
				t.Fatalf("patch = %+v, want mode %d target #count", p, c.want)
			}
		})
	}
}

func TestREQ_ACT_04_PatchNeedsID(t *testing.T) {
	a := &fakeAdapter{}
	h := gx.Action(func(ctx *gx.Ctx, in actRoute) error {
		return ctx.Patch(gx.El("span", nil, gx.Text("x")))
	})
	rec := serveAction(t, a, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with the error toast", rec.Code)
	}
	if p, ok := a.res.Patches[0].(gx.ToastPatch); !ok || !strings.Contains(p.Text, "no id") {
		t.Fatalf("patch = %#v, want a toast about the missing id", a.res.Patches[0])
	}
}
