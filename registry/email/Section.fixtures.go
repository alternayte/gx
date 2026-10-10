package email

import "github.com/alternayte/gx"

// SectionFixtures are the examples of Section in the dev gallery.
var SectionFixtures = gx.Fixtures[SectionProps]{
	"Default": {Children: Text(TextProps{Children: gx.Text("A section groups the parts of an email.")})},
	"Card":    {Card: true, Children: Text(TextProps{Children: gx.Text("A card section has a border and padding.")})},
}
