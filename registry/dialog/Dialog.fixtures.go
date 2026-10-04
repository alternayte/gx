package dialog

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var DialogFixtures = gx.Fixtures[DialogProps]{
	"Default": {
		Id:          "demo-dialog",
		Title:       "Edit profile",
		Description: "Change your display name.",
		Children:    gx.Text("Dialog body."),
		Footer:      button.Button(button.ButtonProps{Children: gx.Text("Save")}),
		Trigger:     button.Button(button.ButtonProps{Variant: button.Outline, Children: gx.Text("Open dialog")}),
	},
}
