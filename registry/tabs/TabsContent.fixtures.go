package tabs

import "github.com/alternayte/gx"

var TabsContentFixtures = gx.Fixtures[TabsContentProps]{
	"Panel": {Label: "Account", Children: gx.Text("Account settings.")},
}

// TabsContentWrap renders the panel inside a tab group with its trigger, as
// a page uses it.
func TabsContentWrap(n gx.Node) gx.Node {
	return Tabs(TabsProps{Children: gx.Frag(
		TabsList(TabsListProps{Label: "Settings", Children: TabsTrigger(TabsTriggerProps{Label: "Account"})}),
		n,
	)})
}
