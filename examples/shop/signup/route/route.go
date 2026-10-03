// Package route holds the signup route types of the shop slice (DR-01).
package route

import "github.com/alternayte/gx"

// Address is the nested address form struct (REQ-FRM-08).
type Address struct {
	Street string `form:"street"`
	City   string `form:"city"`
}

// Signup is the signup form input (REQ-FRM-01).
type Signup struct {
	gx.Route  `POST /signup`
	Email     string
	Age       int
	Terms     bool
	Avatar    gx.File
	Address   Address
	Addresses []Address
}

// Rules declares the validation rules of the signup form (REQ-FRM-01).
func (in *Signup) Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
		gx.Field(&in.Age, gx.Min(18), gx.Max(120)),
		gx.Field(&in.Terms, gx.True("terms.required")),
		gx.Field(&in.Avatar, gx.MaxSize(1<<20), gx.Accept("image/*")),
		gx.Field(&in.Address.Street, gx.Required),
	}
}

// AddAddress appends one repeated address row (REQ-FRM-08).
type AddAddress struct {
	gx.Route  `POST /signup/addresses/add`
	Address   Address
	Addresses []Address
	Avatar    gx.File
}

// RemoveAddress removes one repeated address row (REQ-FRM-08).
type RemoveAddress struct {
	gx.Route  `POST /signup/addresses/remove`
	Index     int
	Address   Address
	Addresses []Address
	Avatar    gx.File
}

// Page is the signup page route.
type Page struct {
	gx.Route `GET /signup`
}
