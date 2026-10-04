package cart

import "github.com/alternayte/gx"

// Fixtures are the gallery examples of the cart (REQ-AI-03).
var Fixtures = gx.Fixtures[CartProps]{
	"Default": {Label: "Fixture cart", Total: 3},
	"Empty":   {Label: "Empty cart", Total: 0},
}
