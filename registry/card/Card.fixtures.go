package card

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
)

var CardFixtures = gx.Fixtures[CardProps]{
	"Full":      {Title: "Card title", Description: "A short description.", Children: gx.Text("Card body."), Footer: gx.Text("Footer")},
	"TitleOnly": {Title: "Title only"},
	"BodyOnly":  {Children: gx.Text("Body only.")},
	"Action": {
		Title:       "Sign in",
		Description: "Enter your email to sign in.",
		Action:      button.Button(button.ButtonProps{Variant: button.Link, Children: gx.Text("Sign up")}),
		Children:    gx.Text("Card body."),
		Footer:      button.Button(button.ButtonProps{Class: "w-full", Children: gx.Text("Continue")}),
	},
}
