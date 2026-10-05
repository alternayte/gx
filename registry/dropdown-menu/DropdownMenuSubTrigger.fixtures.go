package dropdownmenu

import "github.com/alternayte/gx"

var DropdownMenuSubTriggerFixtures = gx.Fixtures[DropdownMenuSubTriggerProps]{
	"Trigger":  {Children: gx.Text("More tools")},
	"Inset":    {Inset: true, Children: gx.Text("More tools")},
	"Disabled": {Disabled: true, Children: gx.Text("More tools")},
}

// DropdownMenuSubTriggerWrap renders the trigger inside a menu, as a page uses it.
func DropdownMenuSubTriggerWrap(n gx.Node) gx.Node {
	return gx.El("div", gx.Attrs{{Key: "role", Value: "menu"}, {Key: "class", Value: "w-56"}}, n)
}
