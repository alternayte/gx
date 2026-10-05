package shell

import "github.com/alternayte/gx"

var TocFixtures = gx.Fixtures[TocProps]{
	"Default": {Headings: fixtureHeadings},
}
