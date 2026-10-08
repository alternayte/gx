package gx

import "regexp"

// EventValue is one domain event with its detail, ready for Ctx.Emit.
type EventValue struct {
	Name   string
	Detail any
}

// EventPatch is a domain event in the answer of an action (REQ-ISL-17). An
// adapter dispatches it in the browser as a CustomEvent.
type EventPatch struct {
	Name   string
	Detail any
	// Scope names the component instance that invoked the action.
	Scope string
}

func (EventPatch) patch() {}

// eventName is the form of a domain event name: lowercase words with a
// hyphen between them, as "cart-changed". The hyphen keeps the name
// different from each event of the browser.
var eventName = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)

// Event returns a typed domain event (REQ-ISL-17). Call it with a detail to
// make the value that Ctx.Emit sends:
//
//	var Changed = gx.Event[ChangedDetail]("cart-changed")
//	c.Emit(Changed(ChangedDetail{Count: 3}))
//
// In a widget, the element of the host page dispatches the event. On a page
// of the app, the event goes to the component that invoked the action. The
// detail goes to the browser as JSON, so it holds no gx.Secret (SI-04).
//
// Event panics for a name that is not lowercase words with hyphens, and for
// a name that starts with "gx-": these names are the events of Gx.
func Event[D any](name string) func(D) EventValue {
	if !eventName.MatchString(name) {
		panic("gx: Event(" + quoteTag(name) + "): an event name is lowercase words with a hyphen between them, for example cart-changed")
	}
	if len(name) > 3 && name[:3] == "gx-" {
		panic("gx: Event(" + quoteTag(name) + "): a name that starts with gx- is an event of Gx")
	}
	return func(detail D) EventValue {
		return EventValue{Name: name, Detail: detail}
	}
}

// Emit sends a domain event with the answer of an action or a form
// (REQ-ISL-17). The server knows that a change is real. An event is thus a
// call of the server and not of the template.
//
// Emit panics outside an action or a form.
func (c *Ctx) Emit(e EventValue) {
	if c.res == nil {
		panic("gx: Emit is only valid in an action or a form")
	}
	checkSecret(e.Detail)
	c.res.Patches = append(c.res.Patches, EventPatch{Name: e.Name, Detail: e.Detail, Scope: Scope(c.R)})
}
