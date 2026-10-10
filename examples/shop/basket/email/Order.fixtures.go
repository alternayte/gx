package email

import "github.com/alternayte/gx"

// OrderFixtures are the examples of the order email in the dev gallery.
var OrderFixtures = gx.Fixtures[OrderProps]{
	"TwoItems": {Number: "1042", Name: "Ada", Total: "CHF 29.50", Items: []Item{
		{Name: "Green tea", Qty: 2, Price: "CHF 24.00"},
		{Name: "Cup & saucer", Qty: 1, Price: "CHF 5.50"},
	}},
}
