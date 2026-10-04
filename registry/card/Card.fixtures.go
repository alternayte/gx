package card

import "github.com/alternayte/gx"

var CardFixtures = gx.Fixtures[CardProps]{
	"Full":      {Title: "Card title", Description: "A short description.", Children: gx.Text("Card body."), Footer: gx.Text("Footer")},
	"TitleOnly": {Title: "Title only"},
	"BodyOnly":  {Children: gx.Text("Body only.")},
}
