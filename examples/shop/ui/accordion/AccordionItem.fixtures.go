package accordion

import "github.com/alternayte/gx"

// Each fixture has its own name: details elements that share a name form one
// exclusive group across the whole gallery page.
var AccordionItemFixtures = gx.Fixtures[AccordionItemProps]{
	"Closed":   {Name: "item-closed", Title: "A question", Children: gx.Text("An answer.")},
	"Open":     {Name: "item-open", Title: "A question", Open: true, Children: gx.Text("An answer.")},
	"Disabled": {Name: "item-disabled", Title: "A question", Disabled: true, Children: gx.Text("An answer.")},
}
