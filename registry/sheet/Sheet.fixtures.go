package sheet

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var SheetFixtures = gx.Fixtures[SheetProps]{
	"Right": {
		Id:          "demo-sheet",
		Title:       "Filters",
		Description: "Narrow the result set.",
		Children:    gx.Text("Sheet body."),
		Trigger:     button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Open sheet")}),
	},
	"Bottom": {
		Id:      "demo-sheet-bottom",
		Side:    Bottom,
		Title:   "Filters",
		Trigger: button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Open bottom sheet")}),
	},
}
