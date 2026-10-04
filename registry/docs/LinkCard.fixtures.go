package docs

import "github.com/alternayte/gx"

var LinkCardFixtures = gx.Fixtures[LinkCardProps]{
	"Full":  {Href: gx.URL("/errors/GX1000"), Title: "Diagnostics", Description: "Every code has a page.", Children: gx.Text("Read on.")},
	"Short": {Href: gx.URL("/start"), Title: "Start"},
}
