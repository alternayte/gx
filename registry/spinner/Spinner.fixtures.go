package spinner

import "github.com/alternayte/gx"

var SpinnerFixtures = gx.Fixtures[SpinnerProps]{
	"Small": {Size: Sm},
	"Label": {Label: "Loading"},
	"Large": {Size: Lg, Label: "Loading"},
}
