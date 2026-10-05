package spinner

import "github.com/alternayte/gx"

var SpinnerFixtures = gx.Fixtures[SpinnerProps]{
	"Default": {},
	"Large":   {Class: "size-8"},
	"Muted":   {Class: "size-6 text-muted-foreground", Label: "Saving"},
}
