package shell

import "github.com/alternayte/gx"

var SidebarGroupFixtures = gx.Fixtures[SidebarGroupProps]{
	"Default": {Group: fixtureStart, Path: "/start/"},
	"Nested":  {Group: fixtureGuides, Path: "/guides/routing/pages/"},
}
