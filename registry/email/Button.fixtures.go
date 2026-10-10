package email

import "github.com/alternayte/gx"

// ButtonFixtures are the examples of Button in the dev gallery.
var ButtonFixtures = gx.Fixtures[ButtonProps]{
	"Default": {Href: gx.URL("/orders/1042"), Children: gx.Text("See the order")},
	"Center":  {Href: gx.URL("/orders/1042"), Align: Center, Children: gx.Text("Follow the parcel")},
}
