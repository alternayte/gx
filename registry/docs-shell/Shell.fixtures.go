package shell

import "github.com/alternayte/gx"

var ShellFixtures = gx.Fixtures[ShellProps]{
	"Default": {Site: fixtureSite, Nav: fixtureNav, Page: fixturePage, Children: gx.Text("Body.")},
}
