package starlight

import "github.com/alternayte/gx"

var ShellFixtures = gx.Fixtures[ShellProps]{
	"Default": {Site: fixtureSite, Nav: fixtureNav, Page: fixturePage, Children: gx.Text("Body.")},
	"Splash": {
		Site:     fixtureSite,
		Nav:      fixtureNav,
		Page:     Page{Title: "Deedbox", Path: "/", Splash: true},
		Hero:     Hero(HeroProps{Title: "Deedbox", Tagline: "Event-source part of your app."}),
		Children: gx.Text("Body."),
	},
}
