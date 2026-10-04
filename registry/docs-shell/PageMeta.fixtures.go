package shell

import "github.com/alternayte/gx"

var PageMetaFixtures = gx.Fixtures[PageMetaProps]{
	"Default": {Site: fixtureSite, Page: fixturePage},
	"Plain":   {Site: Config{Title: "Site"}, Page: Page{Title: "Page", Path: "/page/"}},
}
