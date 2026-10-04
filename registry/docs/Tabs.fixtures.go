package docs

import "github.com/alternayte/gx"

var TabsFixtures = gx.Fixtures[TabsProps]{
	"Two": {Children: gx.Frag(
		TabItem(TabItemProps{Label: "Postgres", Children: gx.Text("Postgres body.")}),
		TabItem(TabItemProps{Label: "SQL Server", Children: gx.Text("SQL Server body.")}),
	)},
	"Synced": {Sync: "db", Children: gx.Frag(
		TabItem(TabItemProps{Label: "Option A", Children: gx.Text("A.")}),
		TabItem(TabItemProps{Label: "Option B", Children: gx.Text("B.")}),
	)},
}
