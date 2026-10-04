package tabs

import "github.com/alternayte/gx"

var TabsFixtures = gx.Fixtures[TabsProps]{
	"Two": {Children: gx.Frag(
		TabItem(TabItemProps{Label: "Account", Children: gx.Text("Account settings.")}),
		TabItem(TabItemProps{Label: "Password", Children: gx.Text("Password settings.")}),
	)},
	"Synced": {Sync: "demo-tabs", Children: gx.Frag(
		TabItem(TabItemProps{Label: "First", Children: gx.Text("First panel.")}),
		TabItem(TabItemProps{Label: "Second", Children: gx.Text("Second panel.")}),
	)},
}
