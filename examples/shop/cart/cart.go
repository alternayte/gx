package cart

import (
	"errors"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/cart/route"
	shoproute "github.com/alternayte/gx/examples/shop/route"
)

// Base is the signal namespace of one Cart instance (REQ-ACT-06).
const Base = "cart.Cart"

// Add is the action behind the Add button.
var Add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	key := gx.ScopeKey(gx.Scope(c.R), Base)
	return c.Patch(CartTotal(key, in.Qty*10))
})

// Set sets Qty to 2 on the invoking instance.
var Set = gx.Action(func(c *gx.Ctx, in route.Set) error {
	return c.SetSignals(CartSignals{Qty: 2})
})

// Redirect navigates to the home page.
var Redirect = gx.Action(func(c *gx.Ctx, in route.Redirect) error {
	return c.Redirect(shoproute.Home{})
})

// Toast shows a toast.
var Toast = gx.Action(func(c *gx.Ctx, in route.Toast) error {
	return c.Toast("Saved")
})

// Noop answers 204.
var Noop = gx.Action(func(c *gx.Ctx, in route.Noop) error { return nil })

// Transition patches inside a view transition.
var Transition = gx.Action(func(c *gx.Ctx, in route.Transition) error {
	return c.Patch(gx.ViewTransition,
		gx.El("span", gx.Attrs{{Key: "id", Value: "transition-target"}}, gx.Text("done")))
})

// Fail fails on purpose.
var Fail = gx.Action(func(c *gx.Ctx, in route.Error) error {
	return errors.New("gx shop: the demo action failed")
})

// Lazy fills the lazy slot.
var Lazy = gx.Action(func(c *gx.Ctx, in route.Lazy) error {
	return c.Patch(gx.El("span", gx.Attrs{{Key: "id", Value: "lazy-slot"}}, gx.Text("loaded")))
})

// Routes collects the cart actions.
var Routes = gx.Collect(Add, Set, Redirect, Toast, Noop, Transition, Fail, Lazy)
