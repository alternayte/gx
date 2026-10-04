package docs

import "github.com/alternayte/gx"

var IconFixtures = gx.Fixtures[IconProps]{
	"Decorative": {Body: gx.SafeHTML(`<path d="M4 12h16" />`)},
	"Labelled":   {Body: gx.SafeHTML(`<path d="M4 12h16" />`), Label: "A dash"},
}
