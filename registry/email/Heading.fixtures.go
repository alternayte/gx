package email

import "github.com/alternayte/gx"

// HeadingFixtures are the examples of Heading in the dev gallery.
var HeadingFixtures = gx.Fixtures[HeadingProps]{
	"One":   {Children: gx.Text("Your order is on its way")},
	"Two":   {Level: 2, Children: gx.Text("What you ordered")},
	"Three": {Level: 3, Align: Center, Children: gx.Text("Questions")},
}
