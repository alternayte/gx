package accordion

import "github.com/alternayte/gx"

var AccordionFixtures = gx.Fixtures[AccordionProps]{
	"Two": {Name: "faq", Children: gx.Frag(
		AccordionItem(AccordionItemProps{Name: "faq", Title: "Is it accessible?", Open: true, Children: gx.Text("Yes. It uses the native details element.")}),
		AccordionItem(AccordionItemProps{Name: "faq", Title: "Is it animated?", Children: gx.Text("Yes. The panel height animates.")}),
	)},
}
