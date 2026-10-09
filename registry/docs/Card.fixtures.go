package docs

import "github.com/alternayte/gx"

var CardFixtures = gx.Fixtures[CardProps]{
	"TitleAndBody": {Title: "Card", Description: "A short description.", Children: gx.Text("Body text.")},
	"BodyOnly":     {Children: gx.Text("Body only.")},
	"TitleOnly":    {Title: "Title only"},
	"WithIcon":     {Title: "Settings", Icon: "setting", Children: gx.Text("Body text.")},
}
