package alert

import "github.com/alternayte/gx"

var AlertFixtures = gx.Fixtures[AlertProps]{
	"Default":     {Title: "Heads up", Children: gx.Text("You can add components to your app.")},
	"Destructive": {Variant: Destructive, Title: "Error", Children: gx.Text("Your session expired. Sign in again.")},
}
