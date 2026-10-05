package contextmenu

import "github.com/alternayte/gx"

var ContextMenuSubTriggerFixtures = gx.Fixtures[ContextMenuSubTriggerProps]{
	"Trigger":  {Children: gx.Text("More tools")},
	"Inset":    {Inset: true, Children: gx.Text("More tools")},
	"Disabled": {Disabled: true, Children: gx.Text("More tools")},
}

// ContextMenuSubTriggerWrap renders the trigger inside a menu, as a page uses it.
func ContextMenuSubTriggerWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
