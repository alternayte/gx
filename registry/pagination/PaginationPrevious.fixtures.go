package pagination

import "github.com/alternayte/gx"

var PaginationPreviousFixtures = gx.Fixtures[PaginationPreviousProps]{"Default": {Href: gx.URL("/?page=1")}}
