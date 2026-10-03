// Package signup is the example form slice of the reference app (REQ-FRM-02).
package signup

import (
	"github.com/alternayte/gx"
	shoproute "github.com/alternayte/gx/examples/shop/route"
	formroute "github.com/alternayte/gx/examples/shop/signup/route"
)

// Create is the signup form action (REQ-FRM-02).
var Create = gx.Form(func(c *gx.Ctx, in *formroute.Signup) error {
	if in.Email == "taken@example.com" {
		return gx.FieldError(&in.Email, "email.taken")
	}
	return c.Redirect(shoproute.Home{})
}, SignupView)

// SignupPage renders the signup page (REQ-FRM-02).
var SignupPage = gx.Page(func(c *gx.Ctx, in formroute.Page) (SignupViewProps, error) {
	return Create.Props(&formroute.Signup{}), nil
}, SignupView)

// Routes collects the signup page and form.
var Routes = gx.Collect(SignupPage, Create)
