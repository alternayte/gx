// Package signup is the example form slice of the reference app (REQ-FRM-02).
package signup

import (
	"strconv"

	"github.com/alternayte/gx"
	shoproute "github.com/alternayte/gx/examples/shop/route"
	formroute "github.com/alternayte/gx/examples/shop/signup/route"
	"github.com/alternayte/gx/examples/shop/ui"
)

// Create is the signup form action (REQ-FRM-02).
var Create = gx.Form(func(c *gx.Ctx, in *formroute.Signup) error {
	if in.Email == "taken@example.com" {
		return gx.FieldError(&in.Email, "email.taken")
	}
	return c.Redirect(shoproute.Home{})
}, SignupView)

// AddAddress appends one repeated address row (REQ-FRM-08).
var AddAddress = gx.Action(func(c *gx.Ctx, in formroute.AddAddress) error {
	rows := append(append([]formroute.Address{}, in.Addresses...), formroute.Address{})
	return c.Patch(addressList(formroute.SignupAddressesFieldValue("signup", rows, nil)))
})

// RemoveAddress removes one repeated address row (REQ-FRM-08).
var RemoveAddress = gx.Action(func(c *gx.Ctx, in formroute.RemoveAddress) error {
	if in.Index < 0 || in.Index >= len(in.Addresses) {
		return nil
	}
	rows := append(append([]formroute.Address{}, in.Addresses[:in.Index]...), in.Addresses[in.Index+1:]...)
	return c.Patch(addressList(formroute.SignupAddressesFieldValue("signup", rows, nil)))
})

// SignupPage renders the signup page (REQ-FRM-02).
var SignupPage = gx.Page(func(c *gx.Ctx, in formroute.Page) (SignupViewProps, error) {
	return Create.Props(&formroute.Signup{}), nil
}, SignupView)

// addressList renders the repeated address rows (REQ-FRM-08).
func addressList(f formroute.SignupAddressesField) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "id", Value: "signup-addresses"}},
		f.Each(addressRow),
	)
}

// addressRow renders one repeated address row.
func addressRow(i int, a formroute.AddressForm) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "class", Value: "address-row"}, {Key: "data-index", Value: strconv.Itoa(i)}},
		ui.TextField(ui.TextFieldProps{Field: a.Street, Label: "Street", Type: "text"}),
		removeButton(i),
	)
}

// removeButton posts the form to the remove action (REQ-FRM-08).
func removeButton(i int) gx.Node {
	return gx.El("button", gx.Attrs{
		{Key: "type", Value: "submit"},
		{Key: "formaction", Value: "/signup/addresses/remove", Kind: gx.AttrURL},
		{Key: "formnovalidate", Value: "true", Kind: gx.AttrBool},
		{Key: "name", Value: "index"},
		{Key: "value", Value: strconv.Itoa(i)},
	}, gx.Text("Remove"))
}

// addButton posts the form to the add action (REQ-FRM-08).
func addButton() gx.Node {
	return gx.El("button", gx.Attrs{
		{Key: "type", Value: "submit"},
		{Key: "formaction", Value: "/signup/addresses/add", Kind: gx.AttrURL},
		{Key: "formnovalidate", Value: "true", Kind: gx.AttrBool},
	}, gx.Text("Add address"))
}

// Routes collects the signup page and form.
var Routes = gx.Collect(SignupPage, Create, AddAddress, RemoveAddress)
