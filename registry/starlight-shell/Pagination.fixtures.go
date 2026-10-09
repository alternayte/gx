package starlight

import "github.com/alternayte/gx"

var PaginationFixtures = gx.Fixtures[PaginationProps]{
	"Both": {Prev: fixturePrev, Next: fixtureNext},
	"Next": {Next: fixtureNext},
	"Prev": {Prev: fixturePrev},
}
