// Package route holds the cart action routes (DR-01).
package route

import "github.com/alternayte/gx"

// Add patches the total of the invoking cart instance.
type Add struct {
	gx.Route `POST /cart/add`
	gx.Unchecked
	Qty int `signal:"qty"`
}

// Set sets the signals of the invoking cart instance.
type Set struct {
	gx.Route `POST /cart/set`
}

// Redirect navigates to the home page.
type Redirect struct {
	gx.Route `POST /cart/redirect`
}

// Toast shows a toast.
type Toast struct {
	gx.Route `POST /cart/toast`
}

// ToastDemo shows one toast of the demo row: a kind, a description, a
// link, an action, or one step of a replace by ID.
type ToastDemo struct {
	gx.Route `POST /cart/toast-demo`
	Show     string `query:"show"`
}

// Undo is the action behind the Undo button of a toast.
type Undo struct {
	gx.Route `POST /cart/undo`
}

// Noop answers 204.
type Noop struct {
	gx.Route `POST /cart/noop`
}

// Transition patches inside a view transition.
type Transition struct {
	gx.Route `POST /cart/transition`
}

// Error fails on purpose, for the toast path.
type Error struct {
	gx.Route `POST /cart/error`
}

// Lazy is a GET action for load, visible and interval runs.
type Lazy struct {
	gx.Route `GET /lazy`
}
