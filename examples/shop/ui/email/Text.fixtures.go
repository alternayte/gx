package email

import "github.com/alternayte/gx"

// TextFixtures are the examples of Text in the dev gallery.
var TextFixtures = gx.Fixtures[TextProps]{
	"Default": {Children: gx.Text("Thank you for your order. We send it in two days.")},
	"Muted":   {Muted: true, Children: gx.Text("You get this email because you have an account.")},
	"Center":  {Align: Center, Children: gx.Text("Text in the centre")},
}
