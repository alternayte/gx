package avatar

import "github.com/alternayte/gx"

var AvatarFixtures = gx.Fixtures[AvatarProps]{
	"Fallback": {Fallback: "NA"},
	"Small":    {Fallback: "NA", Size: Sm},
	"Large":    {Fallback: "NA", Size: Lg},
}
