package alert

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/icons"
)

var AlertFixtures = gx.Fixtures[AlertProps]{
	"Default":     {Title: "Heads up", Children: gx.Text("You can add components to your app.")},
	"Destructive": {Variant: Destructive, Title: "Error", Children: gx.Text("Your session expired. Sign in again.")},
	"Icon":        {Icon: icons.Info(icons.InfoProps{}), Title: "Heads up", Children: gx.Text("You can add components to your app.")},
	"DestructiveIcon": {
		Variant:  Destructive,
		Icon:     icons.TriangleAlert(icons.TriangleAlertProps{}),
		Title:    "Error",
		Children: gx.Text("Your session expired. Sign in again."),
	},
	"TitleOnly": {Icon: icons.CircleCheck(icons.CircleCheckProps{}), Title: "Your changes are saved."},
}
