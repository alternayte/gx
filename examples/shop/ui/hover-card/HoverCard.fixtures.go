package hovercard

import "github.com/alternayte/gx"

var HoverCardFixtures = gx.Fixtures[HoverCardProps]{
	"User": {
		Trigger: gx.El("button", gx.Attrs{{Key: "type", Value: "button"}}, gx.Text("@ada")),
		Children: gx.Frag(
			gx.El("p", gx.Attrs{{Key: "class", Value: "font-medium"}}, gx.Text("Ada Lovelace")),
			gx.El("p", gx.Attrs{{Key: "class", Value: "text-muted-foreground"}}, gx.Text("First programmer.")),
		),
	},
}
