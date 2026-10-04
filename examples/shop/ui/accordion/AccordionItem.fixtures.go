package accordion

import "github.com/alternayte/gx"

var AccordionItemFixtures = gx.Fixtures[AccordionItemProps]{
	"Closed": {Name: "faq", Title: "A question", Children: gx.Text("An answer.")},
	"Open":   {Name: "faq", Title: "A question", Open: true, Children: gx.Text("An answer.")},
}
