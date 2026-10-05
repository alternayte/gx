package tabs

import "github.com/alternayte/gx"

var TabsTriggerFixtures = gx.Fixtures[TabsTriggerProps]{
	"Trigger":  {Label: "Account"},
	"Disabled": {Label: "Account", Disabled: true},
}

// TabsTriggerWrap renders the trigger inside a tab group with its panel, as
// a page uses it.
func TabsTriggerWrap(n gx.Node) gx.Node {
	return Tabs(TabsProps{Children: gx.Frag(
		TabsList(TabsListProps{Label: "Settings", Children: n}),
		TabsContent(TabsContentProps{Label: "Account", Children: gx.Text("Account settings.")}),
	)})
}
