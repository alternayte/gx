package shell

import "github.com/alternayte/gx"

var SidebarGroupFixtures = gx.Fixtures[SidebarGroupProps]{
	"Default": {Group: fixtureNav.Groups[0], Path: "/start/"},
	"Nested":  {Group: fixtureNav.Groups[1], Path: "/guides/routing/pages/"},
}
