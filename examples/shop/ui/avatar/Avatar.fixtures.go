package avatar

import "github.com/alternayte/gx"

var AvatarFixtures = gx.Fixtures[AvatarProps]{
	"Fallback": {Fallback: "NA"},
	"Small":    {Fallback: "NA", Size: Sm},
	"Large":    {Fallback: "NA", Size: Lg},
	"Badge":    {Fallback: "NA", Badge: AvatarBadge(AvatarBadgeProps{})},
	"LargeBadge": {Fallback: "NA", Size: Lg, Badge: AvatarBadge(AvatarBadgeProps{
		Class: "bg-green-600 dark:bg-green-800",
	})},
}
