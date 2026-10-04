package alertdialog

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/ui/button"
)

var AlertDialogFixtures = gx.Fixtures[AlertDialogProps]{
	"Default": {
		Id:          "demo-alert",
		Title:       "Delete this item?",
		Description: "This action cannot be undone.",
		Confirm:     button.Button(button.ButtonProps{Variant: button.Destructive, Children: gx.Text("Delete")}),
		Trigger:     button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Delete")}),
	},
}
