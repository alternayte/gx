package empty

import "github.com/alternayte/gx"

var EmptyFixtures = gx.Fixtures[EmptyProps]{
	"Full": {Children: gx.Frag(
		EmptyHeader(EmptyHeaderProps{Children: gx.Frag(
			EmptyMedia(EmptyMediaProps{Variant: Icon, Children: gx.Text("+")}),
			EmptyTitle(EmptyTitleProps{Children: gx.Text("No projects")}),
			EmptyDescription(EmptyDescriptionProps{Children: gx.Text("Create your first project to start.")}),
		)}),
		EmptyContent(EmptyContentProps{Children: gx.Text("New project")}),
	)},
}

var EmptyHeaderFixtures = gx.Fixtures[EmptyHeaderProps]{"Empty": {}}
var EmptyTitleFixtures = gx.Fixtures[EmptyTitleProps]{"Title": {Children: gx.Text("No projects")}}
var EmptyDescriptionFixtures = gx.Fixtures[EmptyDescriptionProps]{"Text": {Children: gx.Text("Create your first project.")}}
var EmptyContentFixtures = gx.Fixtures[EmptyContentProps]{"Empty": {}}
var EmptyMediaFixtures = gx.Fixtures[EmptyMediaProps]{"Icon": {Variant: Icon, Children: gx.Text("+")}}
