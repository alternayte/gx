package tabs

import "github.com/alternayte/gx"

var TabsListFixtures = gx.Fixtures[TabsListProps]{
	"Default": {Label: "Settings", Children: gx.Frag(
		TabsTrigger(TabsTriggerProps{Label: "Account"}),
		TabsTrigger(TabsTriggerProps{Label: "Password"}),
	)},
	"Line": {Variant: Line, Label: "Settings", Children: gx.Frag(
		TabsTrigger(TabsTriggerProps{Label: "Account"}),
		TabsTrigger(TabsTriggerProps{Label: "Password"}),
	)},
}

// TabsListWrap renders the list inside a tab group with the panels of its
// triggers, as a page uses it.
func TabsListWrap(n gx.Node) gx.Node {
	return Tabs(TabsProps{Children: gx.Frag(
		n,
		TabsContent(TabsContentProps{Label: "Account", Children: gx.Text("Account settings.")}),
		TabsContent(TabsContentProps{Label: "Password", Children: gx.Text("Password settings.")}),
	)})
}
