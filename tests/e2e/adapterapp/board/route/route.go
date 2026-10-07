// Package route holds the route types of the board slice (DR-01).
package route

import "github.com/alternayte/gx"

// Home is the page with one control for each action answer.
type Home struct {
	gx.Route `GET /{$}`
}

// About is the second page of the layout.
type About struct {
	gx.Route `GET /about`
}

// Inc answers with a morph patch.
type Inc struct {
	gx.Route `POST /inc`
}

// AddLast answers with an append patch.
type AddLast struct {
	gx.Route `POST /items/last`
}

// AddFirst answers with a prepend patch.
type AddFirst struct {
	gx.Route `POST /items/first`
}

// Swap answers with a replace patch.
type Swap struct {
	gx.Route `PUT /swap`
}

// Drop answers with a remove patch.
type Drop struct {
	gx.Route `DELETE /drop`
}

// Leave answers with a redirect.
type Leave struct {
	gx.Route `POST /leave`
}

// Notify answers with a toast.
type Notify struct {
	gx.Route `POST /notify`
}

// Fail answers with an error.
type Fail struct {
	gx.Route `POST /fail`
}

// Quiet answers with nothing.
type Quiet struct {
	gx.Route `POST /quiet`
}

// Fade answers with a patch inside a view transition.
type Fade struct {
	gx.Route `PATCH /fade`
}

// Lazy runs on load.
type Lazy struct {
	gx.Route `GET /lazy`
}

// Seen runs when its element is visible.
type Seen struct {
	gx.Route `GET /seen`
}

// Tick runs on an interval.
type Tick struct {
	gx.Route `GET /tick`
}

// Count adds one to the counter with the name. The page invokes it with
// each event modifier.
type Count struct {
	gx.Route `POST /count/{name}`
	Name     string
}

// JoinPage is the page of the form.
type JoinPage struct {
	gx.Route `GET /join`
}

// Join is the form input.
type Join struct {
	gx.Route `POST /join`
	Name     string
}

// Rules declares the rules of the form.
func (in *Join) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Name, gx.Required, gx.MinLen(3))}
}
