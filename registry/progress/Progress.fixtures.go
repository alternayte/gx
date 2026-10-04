package progress

import "github.com/alternayte/gx"

var ProgressFixtures = gx.Fixtures[ProgressProps]{
	"Half":  {Value: 50, Label: "Upload"},
	"Full":  {Value: 100, Label: "Upload"},
	"Empty": {Value: 0, Label: "Upload"},
}
