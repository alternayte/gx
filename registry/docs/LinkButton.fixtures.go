package docs

import "github.com/alternayte/gx"

var LinkButtonFixtures = gx.Fixtures[LinkButtonProps]{
	"Primary":   {Href: gx.URL("/start"), Children: gx.Text("Get started")},
	"Secondary": {Href: gx.URL("/start"), Variant: ButtonSecondary, Children: gx.Text("Secondary")},
	"Outline":   {Href: gx.URL("/start"), Variant: ButtonOutline, Children: gx.Text("Outline")},
	"Ghost":     {Href: gx.URL("/start"), Variant: ButtonGhost, Children: gx.Text("Ghost")},
	"Minimal":   {Href: gx.URL("/start"), Variant: ButtonMinimal, Children: gx.Text("Minimal")},
	"WithIcon":  {Href: gx.URL("/start"), Icon: "right-arrow", Children: gx.Text("Get started")},
}
