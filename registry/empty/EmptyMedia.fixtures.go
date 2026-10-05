package empty

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/icons"
)

var EmptyMediaFixtures = gx.Fixtures[EmptyMediaProps]{
	"Icon":    {Variant: Icon, Children: icons.Info(icons.InfoProps{})},
	"Default": {Children: icons.Info(icons.InfoProps{Class: "size-8"})},
}
