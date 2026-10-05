package navigationmenu

import "github.com/alternayte/gx"

var NavigationMenuLinkFixtures = gx.Fixtures[NavigationMenuLinkProps]{
	"Link":          {Href: gx.URL("/docs"), Children: gx.Text("Docs")},
	"Active":        {Href: gx.URL("/"), Active: true, Children: gx.Text("Home")},
	"Trigger":       {Href: gx.URL("/docs"), Variant: Trigger, Children: gx.Text("Docs")},
	"TriggerActive": {Href: gx.URL("/"), Variant: Trigger, Active: true, Children: gx.Text("Home")},
}
