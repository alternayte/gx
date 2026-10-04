package pagination

import "github.com/alternayte/gx"

var PaginationNextFixtures = gx.Fixtures[PaginationNextProps]{"Default": {Href: gx.URL("/?page=2")}}
