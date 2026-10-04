package shell

import "github.com/alternayte/gx"

var SplashFixtures = gx.Fixtures[SplashProps]{
	"Default": {Title: "Deedbox", Tagline: "Event sourcing in .NET.", Actions: gx.Text("Docs")},
	"Plain":   {Title: "Deedbox"},
}
