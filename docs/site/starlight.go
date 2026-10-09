package site

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/docs/registry"
)

// starlightStyle returns the Starlight theme as a style element, for the
// preview of a Starlight item.
func starlightStyle() gx.Node {
	return gx.El("style", nil, gx.Raw(gx.SafeHTML(registry.StarlightCSS))) //gx:trusted the stylesheet is a file of the repository
}
