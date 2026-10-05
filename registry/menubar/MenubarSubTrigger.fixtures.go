package menubar

import "github.com/alternayte/gx"

var MenubarSubTriggerFixtures = gx.Fixtures[MenubarSubTriggerProps]{
	"Trigger":  {Children: gx.Text("More tools")},
	"Inset":    {Inset: true, Children: gx.Text("More tools")},
	"Disabled": {Disabled: true, Children: gx.Text("More tools")},
}

// MenubarSubTriggerWrap renders the trigger inside a menu, as a page uses it.
func MenubarSubTriggerWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
