package docs

import "github.com/alternayte/gx"

var CardGridFixtures = gx.Fixtures[CardGridProps]{
	"Two": {Children: gx.Frag(
		Card(CardProps{Title: "One", Children: gx.Text("First.")}),
		Card(CardProps{Title: "Two", Children: gx.Text("Second.")}),
	)},
}
