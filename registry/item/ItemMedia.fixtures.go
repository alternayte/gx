package item

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/registry/icons"
)

var ItemMediaFixtures = gx.Fixtures[ItemMediaProps]{
	"Icon":    {Variant: MediaIcon, Children: icons.Info(icons.InfoProps{})},
	"Default": {Children: icons.Info(icons.InfoProps{Class: "size-5"})},
	"Image": {Variant: MediaImage, Children: gx.El("div", gx.Attrs{
		{Key: "class", Value: "size-full bg-muted"},
	})},
}
