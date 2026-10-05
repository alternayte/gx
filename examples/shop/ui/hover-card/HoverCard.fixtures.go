package hovercard

import "github.com/alternayte/gx"

func demoCard() gx.Node {
	return gx.Frag(
		gx.El("p", gx.Attrs{{Key: "class", Value: "font-medium"}}, gx.Text("Ada Lovelace")),
		gx.El("p", gx.Attrs{{Key: "class", Value: "text-muted-foreground"}}, gx.Text("First programmer.")),
	)
}

var HoverCardFixtures = gx.Fixtures[HoverCardProps]{
	"User": {
		Class:    "text-sm",
		Trigger:  gx.El("button", gx.Attrs{{Key: "type", Value: "button"}}, gx.Text("@ada")),
		Children: demoCard(),
	},
	"Center": {
		Class:    "ml-32 text-sm",
		Align:    Center,
		Trigger:  gx.El("button", gx.Attrs{{Key: "type", Value: "button"}}, gx.Text("@ada, centered")),
		Children: demoCard(),
	},
	"End": {
		Class:    "ml-64 text-sm",
		Align:    End,
		Trigger:  gx.El("button", gx.Attrs{{Key: "type", Value: "button"}}, gx.Text("@ada, at the end")),
		Children: demoCard(),
	},
}
