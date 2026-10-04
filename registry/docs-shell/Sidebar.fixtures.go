package shell

import "github.com/alternayte/gx"

var SidebarFixtures = gx.Fixtures[SidebarProps]{
	"Default": {Nav: fixtureNav, Path: "/guides/routing/pages/"},
}
