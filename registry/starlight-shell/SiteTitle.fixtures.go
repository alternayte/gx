package starlight

import "github.com/alternayte/gx"

var SiteTitleFixtures = gx.Fixtures[SiteTitleProps]{
	"Label":    {Site: fixtureSite},
	"Versions": {Site: fixtureVersions},
	"NoLabel":  {Site: Config{Title: "Deedbox"}},
}
