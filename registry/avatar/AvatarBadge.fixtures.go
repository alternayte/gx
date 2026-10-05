package avatar

import "github.com/alternayte/gx"

var AvatarBadgeFixtures = gx.Fixtures[AvatarBadgeProps]{"Default": {}}

// AvatarBadgeWrap renders the badge on an avatar, as a page uses it.
func AvatarBadgeWrap(n gx.Node) gx.Node {
	return Avatar(AvatarProps{Fallback: "NA", Badge: n})
}
