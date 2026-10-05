package item

import "github.com/alternayte/gx"

var ItemGroupFixtures = gx.Fixtures[ItemGroupProps]{
	"Separated": {Children: gx.Frag(
		Item(ItemProps{Children: ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Ada Lovelace")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("ada@example.com")}),
		)})}),
		ItemSeparator(ItemSeparatorProps{}),
		Item(ItemProps{Children: ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Grace Hopper")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("grace@example.com")}),
		)})}),
	)},
}
