package item

import "github.com/alternayte/gx"

var ItemFixtures = gx.Fixtures[ItemProps]{
	"Full": {Class: "p-4", Children: gx.Frag(
		ItemMedia(ItemMediaProps{Children: gx.Text("*")}),
		ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Item title")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("A short description of the item.")}),
		)}),
		ItemActions(ItemActionsProps{Children: gx.Text("Open")}),
	)},
}
