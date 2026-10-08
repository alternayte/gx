package gx_test

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/alternayte/gx"
)

type changedDetail struct {
	Count int    `json:"count"`
	Note  string `json:"note"`
}

var cartChanged = gx.Event[changedDetail]("cart-changed")

// TestREQ_ISL_17_Emit checks that an action sends a typed domain event with
// its answer: to the adapter of a page as a patch, and to a widget as a step
// of the answer.
func TestREQ_ISL_17_Emit(t *testing.T) {
	h := gx.Action(func(c *gx.Ctx, in actRoute) error {
		c.Emit(cartChanged(changedDetail{Count: 3, Note: "<b>"}))
		return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "total"}}, gx.Text("30")))
	})

	t.Run("a page", func(t *testing.T) {
		adapter := &fakeAdapter{}
		app := gx.New(gx.Config{Adapter: adapter})
		app.Group("/", gx.Collect(h))
		req := httptest.NewRequest("POST", "/act", nil)
		req.Header.Set("Gx-Scope", "cart.Cart.42")
		app.ServeHTTP(httptest.NewRecorder(), req)
		if !adapter.responded || len(adapter.res.Patches) != 2 {
			t.Fatalf("the adapter got %+v, want the event and the patch", adapter.res)
		}
		event, ok := adapter.res.Patches[0].(gx.EventPatch)
		if !ok || event.Name != "cart-changed" || event.Scope != "cart.Cart.42" || event.Detail != (changedDetail{Count: 3, Note: "<b>"}) {
			t.Fatalf("first patch = %#v, want the event with its scope and detail", adapter.res.Patches[0])
		}
	})

	t.Run("a widget", func(t *testing.T) {
		app := gx.New(gx.Config{Adapter: &fakeAdapter{}})
		app.Group("/", gx.Collect(h))
		req := widgetRequest("POST", "/act", "")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		var answer struct {
			Ops []struct {
				Op     string
				Name   string
				Detail map[string]any
			} `json:"ops"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
			t.Fatalf("answer: %v\n%s", err, rec.Body.String())
		}
		if len(answer.Ops) != 2 || answer.Ops[0].Op != "event" || answer.Ops[0].Name != "cart-changed" ||
			answer.Ops[0].Detail["count"] != float64(3) || answer.Ops[0].Detail["note"] != "<b>" || answer.Ops[1].Op != "patch" {
			t.Fatalf("answer = %s", rec.Body.String())
		}
	})
}

// TestREQ_ISL_17_EventName checks the rule of an event name: lowercase words
// with hyphens, and no name of Gx.
func TestREQ_ISL_17_EventName(t *testing.T) {
	for _, name := range []string{"changed", "click", "Cart-Changed", "cart_changed", "cart changed", "cart-", "-cart", "", "gx-ready", "gx-cart-changed", "cart--changed"} {
		t.Run(strconv.Quote(name), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("no panic")
				}
			}()
			gx.Event[changedDetail](name)
		})
	}
	for _, name := range []string{"cart-changed", "composer-sent", "a-b", "order2-paid-in-full"} {
		gx.Event[changedDetail](name)
	}
}

// TestREQ_ISL_17_EmitOutsideAnAction checks that Emit with no answer to
// carry the event is a panic, not a lost event.
func TestREQ_ISL_17_EmitOutsideAnAction(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	(&gx.Ctx{}).Emit(cartChanged(changedDetail{}))
}
