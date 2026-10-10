package email

import "github.com/alternayte/gx"

// DocumentFixtures are the examples of Document in the dev gallery.
var DocumentFixtures = gx.Fixtures[DocumentProps]{
	"Default": {Children: Text(TextProps{Children: gx.Text("The content of the email is here.")})},
	"Preview": {Preview: "Your order is on its way.", Width: 480, Children: Text(TextProps{Children: gx.Text("An inbox shows the preview text after the subject.")})},
}
