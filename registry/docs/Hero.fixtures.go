package docs

import "github.com/alternayte/gx"

var HeroFixtures = gx.Fixtures[HeroProps]{
	"Full": {Title: "Gx", Tagline: "Server-rendered Go web apps.", Actions: gx.Raw(gx.SafeHTML(`<a href="/start">Get started</a>`))}, //gx:trusted a fixture is repository source (SI-12)
	"Title": {Title: "Title only"},
}
