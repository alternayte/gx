package starlight

import "github.com/alternayte/gx"

var SidebarFixtures = gx.Fixtures[SidebarProps]{
	"Default": {Site: fixtureSite, Nav: fixtureNav, Path: "/start/"},
}
