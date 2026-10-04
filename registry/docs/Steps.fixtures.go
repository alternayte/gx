package docs

import "github.com/alternayte/gx"

var StepsFixtures = gx.Fixtures[StepsProps]{
	"Three": {Children: gx.Raw(gx.SafeHTML(`<ol><li>First</li><li>Second</li><li>Third</li></ol>`))}, //gx:trusted a fixture is repository source (SI-12)
}
