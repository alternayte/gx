package avatar

import "github.com/alternayte/gx"

var AvatarGroupFixtures = gx.Fixtures[AvatarGroupProps]{
	"Three": {Children: gx.Frag(
		Avatar(AvatarProps{Fallback: "NA"}),
		Avatar(AvatarProps{Fallback: "AL"}),
		Avatar(AvatarProps{Fallback: "GH"}),
		AvatarGroupCount(AvatarGroupCountProps{Children: gx.Text("+3")}),
	)},
	"Small": {Children: gx.Frag(
		Avatar(AvatarProps{Fallback: "NA", Size: Sm}),
		Avatar(AvatarProps{Fallback: "AL", Size: Sm}),
		AvatarGroupCount(AvatarGroupCountProps{Children: gx.Text("+2")}),
	)},
}
