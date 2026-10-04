package aspectratio

import "github.com/alternayte/gx"

var AspectRatioFixtures = gx.Fixtures[AspectRatioProps]{
	"Wide":   {Ratio: "16 / 9", Children: gx.Text("16 / 9")},
	"Square": {Ratio: "1 / 1", Children: gx.Text("1 / 1")},
}
