// Package shop is the example slice of the reference app.
package shop

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/cart"
	"github.com/alternayte/gx/examples/shop/datatable"
	"github.com/alternayte/gx/examples/shop/route"
	"github.com/alternayte/gx/examples/shop/signup"
)

// ShellLayout wraps every page in the shell (REQ-RTE-08).
var ShellLayout = gx.Layout(
	func(c *gx.Ctx) (struct{}, error) { return struct{}{}, nil },
	func(_ struct{}, children gx.Node) gx.Node {
		return Shell(ShellProps{Children: children})
	})

// HomePage is the home page.
var HomePage = gx.Page(
	func(c *gx.Ctx, in route.Home) (HomeProps, error) { return HomeProps{}, nil },
	Home)

// AboutPage is the about page.
var AboutPage = gx.Page(
	func(c *gx.Ctx, in route.About) (AboutProps, error) { return AboutProps{}, nil },
	About)

// Routes collects every page and action of the shop.
var Routes = append(append(append(gx.Collect(HomePage, AboutPage), cart.Routes...), signup.Routes...), datatable.Routes...)
