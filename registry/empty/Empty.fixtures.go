package empty

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/button"
	"github.com/alternayte/gx/registry/icons"
)

var EmptyFixtures = gx.Fixtures[EmptyProps]{
	"Full": {Children: gx.Frag(
		EmptyHeader(EmptyHeaderProps{Children: gx.Frag(
			EmptyMedia(EmptyMediaProps{Variant: Icon, Children: icons.Info(icons.InfoProps{})}),
			EmptyTitle(EmptyTitleProps{Children: gx.Text("No projects")}),
			EmptyDescription(EmptyDescriptionProps{Children: gx.Text("Create your first project to start.")}),
		)}),
		EmptyContent(EmptyContentProps{Children: button.Button(button.ButtonProps{Children: gx.Text("New project")})}),
	)},
	"Outline": {Class: "border", Children: EmptyHeader(EmptyHeaderProps{Children: gx.Frag(
		EmptyTitle(EmptyTitleProps{Children: gx.Text("No results")}),
		EmptyDescription(EmptyDescriptionProps{Children: gx.Text("Change the filter and search again.")}),
	)})},
}
