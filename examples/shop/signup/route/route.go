// Package route holds the signup route types of the shop slice (DR-01).
package route

import "github.com/alternayte/gx"

// Signup is the signup form input (REQ-FRM-01).
type Signup struct {
	gx.Route `POST /signup`
	Email    string
	Age      int
	Terms    bool
}

// Rules declares the validation rules of the signup form (REQ-FRM-01).
func (in *Signup) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
		gx.Field(&in.Age, gx.Min(18), gx.Max(120)),
		gx.Field(&in.Terms, gx.True("terms.required")),
	}
}

// Page is the signup page route.
type Page struct {
	gx.Route `GET /signup`
}
