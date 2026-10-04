package drawer

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
)

var DrawerFixtures = gx.Fixtures[DrawerProps]{
	"Default": {
		Id:          "demo-drawer",
		Title:       "Share this page",
		Description: "Choose a destination.",
		Children:    gx.Text("Drawer body."),
		Footer:      button.Button(button.ButtonProps{Children: gx.Text("Copy link")}),
		Trigger:     button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Open drawer")}),
	},
}
