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

var ItemMediaFixtures = gx.Fixtures[ItemMediaProps]{"Empty": {}}
var ItemContentFixtures = gx.Fixtures[ItemContentProps]{"Empty": {}}
var ItemTitleFixtures = gx.Fixtures[ItemTitleProps]{"Title": {Children: gx.Text("Item title")}}
var ItemDescriptionFixtures = gx.Fixtures[ItemDescriptionProps]{"Text": {Children: gx.Text("Description")}}
var ItemActionsFixtures = gx.Fixtures[ItemActionsProps]{"Empty": {}}
var ItemHeaderFixtures = gx.Fixtures[ItemHeaderProps]{"Empty": {}}
var ItemFooterFixtures = gx.Fixtures[ItemFooterProps]{"Empty": {}}
