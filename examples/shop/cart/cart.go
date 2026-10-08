package cart

import (
	"errors"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/cart/route"
	shoproute "github.com/alternayte/gx/examples/shop/route"
)

// Base is the signal namespace of one Cart instance (REQ-ACT-06).
const Base = "cart.Cart"

// ChangedDetail is the detail of the cart-changed event.
type ChangedDetail struct {
	// Total is the new total of the cart.
	Total int `json:"total"`
}

// Changed tells the page that the total of a cart changed. A host page of
// the cart widget listens for it on the <shop-cart> element.
var Changed = gx.Event[ChangedDetail]("cart-changed")

// Widget is the cart as a widget: a page of a different site shows it with
// the <shop-cart> element. It is the Cart component of the shop pages.
var Widget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {
	return CartProps{Label: in.Label, Total: 10}, nil
}, Cart).Tag("shop-cart")

// Sets the total of the cart to ten times the quantity. It is the action
// behind the Add button, and a tool for the agent of the user (REQ-AI-06).
var Add = gx.Action(func(c *gx.Ctx, in route.Add) error {
	key := gx.ScopeKey(gx.Scope(c.R), Base)
	total := in.Qty * 10
	c.Emit(Changed(ChangedDetail{Total: total}))
	return c.Patch(CartTotal(key, total))
}).Tool()

// Set sets Qty to 2 on the invoking instance.
var Set = gx.Action(func(c *gx.Ctx, in route.Set) error {
	return c.SetSignals(CartSignals{Qty: 2})
})

// Redirect navigates to the home page.
var Redirect = gx.Action(func(c *gx.Ctx, in route.Redirect) error {
	return c.Redirect(shoproute.Home{})
})

// Saves the cart and shows a toast. The browser asks the user before an
// agent runs it.
var Toast = gx.Action(func(c *gx.Ctx, in route.Toast) error {
	gx.ToolResult(c, SavedResult{Saved: true})
	return c.Toast("Saved")
}).Tool(gx.Confirm)

// SavedResult is the result of the Toast tool for an agent.
type SavedResult struct {
	Saved bool `json:"saved"`
}

// ToastDemo shows the toast one demo button asks for. The two upload steps
// share an ID, so the second toast replaces the first in place.
var ToastDemo = gx.Action(func(c *gx.Ctx, in route.ToastDemo) error {
	switch in.Show {
	case "success":
		return c.Toast("Changes saved", gx.ToastSuccess)
	case "info":
		return c.Toast("A new version is available", gx.ToastInfo)
	case "warning":
		return c.Toast("Your trial ends in 3 days", gx.ToastWarning)
	case "error":
		return c.Toast("The upload failed", gx.ToastError)
	case "description":
		return c.Toast("Event created", gx.ToastSuccess, gx.ToastDescription("Monday, 12 January at 09:00"))
	case "action":
		return c.Toast("Item added to the cart", gx.ToastDescription("Read how the shop works."), gx.ToastLink("About", shoproute.About{}))
	case "undo":
		return c.Toast("Item removed from the cart", gx.ToastAction("Undo", route.Undo{}))
	case "sticky":
		return c.Toast("Stays until you close it", gx.ToastSticky)
	case "upload-start":
		return c.Toast("Uploading the file", gx.ToastLoading, gx.ToastID("upload"))
	case "upload-done":
		return c.Toast("File uploaded", gx.ToastSuccess, gx.ToastID("upload"))
	}
	return c.Toast("Event created")
})

// Undo answers the Undo button of a toast. The answer is the next toast.
var Undo = gx.Action(func(c *gx.Ctx, in route.Undo) error {
	return c.Toast("Item restored", gx.ToastSuccess)
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

// WidgetRoutes collects the cart widget and each action that the Cart
// component invokes. The app mounts them in a group with gx.AllowOrigins, so
// a host page of a different origin can call them.
var WidgetRoutes = gx.Collect(Widget, Add, Set, Redirect, Toast, Noop, Fail)

// Routes collects the other cart actions.
var Routes = gx.Collect(ToastDemo, Undo, Transition, Lazy)
