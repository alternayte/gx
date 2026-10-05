package badge

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/icons"
)

var BadgeFixtures = gx.Fixtures[BadgeProps]{
	"Default":     {Children: gx.Text("Badge")},
	"Secondary":   {Variant: Secondary, Children: gx.Text("Secondary")},
	"Destructive": {Variant: Destructive, Children: gx.Text("Destructive")},
	"Outline":     {Variant: Outline, Children: gx.Text("Outline")},
	"Ghost":       {Variant: Ghost, Children: gx.Text("Ghost")},
	"Link":        {Variant: Link, Href: gx.URL("/docs"), Children: gx.Text("Link")},
	"WithIcon":    {Variant: Secondary, Children: gx.Frag(icons.CircleCheck(icons.CircleCheckProps{}), gx.Text("Verified"))},
	"Anchor":      {Href: gx.URL("/docs"), Children: gx.Text("Docs")},
}
