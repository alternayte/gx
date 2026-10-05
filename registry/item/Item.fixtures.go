package item

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
	"github.com/alternayte/gx/registry/icons"
)

// body is the content of the item fixtures: an icon, a title with a
// description, and one action.
func body() gx.Node {
	return gx.Frag(
		ItemMedia(ItemMediaProps{Variant: MediaIcon, Children: icons.Info(icons.InfoProps{})}),
		ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Item title")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("A short description of the item.")}),
		)}),
		ItemActions(ItemActionsProps{Children: button.Button(button.ButtonProps{Variant: button.Outline, Size: button.Sm, Children: gx.Text("Open")})}),
	)
}

var ItemFixtures = gx.Fixtures[ItemProps]{
	"Full":    {Variant: Outline, Children: body()},
	"Default": {Children: body()},
	"Muted":   {Variant: Muted, Children: body()},
	"Small": {Variant: Outline, Size: Sm, Children: gx.Frag(
		ItemMedia(ItemMediaProps{Children: icons.CircleCheck(icons.CircleCheckProps{Class: "size-5"})}),
		ItemContent(ItemContentProps{Children: ItemTitle(ItemTitleProps{Children: gx.Text("Your profile is verified.")})}),
		ItemActions(ItemActionsProps{Children: icons.ChevronRight(icons.ChevronRightProps{Class: "size-4"})}),
	)},
	"Link": {Variant: Outline, Size: Sm, Href: gx.URL("/docs"), Children: gx.Frag(
		ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Read the docs")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("The item is one link.")}),
		)}),
		ItemActions(ItemActionsProps{Children: icons.ChevronRight(icons.ChevronRightProps{Class: "size-4"})}),
	)},
	"HeaderAndFooter": {Variant: Outline, Children: gx.Frag(
		ItemHeader(ItemHeaderProps{Children: gx.Text("Header")}),
		ItemContent(ItemContentProps{Children: gx.Frag(
			ItemTitle(ItemTitleProps{Children: gx.Text("Item title")}),
			ItemDescription(ItemDescriptionProps{Children: gx.Text("A short description of the item.")}),
		)}),
		ItemFooter(ItemFooterProps{Children: gx.Text("Footer")}),
	)},
}
