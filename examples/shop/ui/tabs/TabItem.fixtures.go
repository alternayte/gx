package tabs

import "github.com/alternayte/gx"

var TabItemFixtures = gx.Fixtures[TabItemProps]{
	"Item": {Label: "Account", Children: gx.Text("Panel.")},
}

// TabItemWrap renders the item inside a tab group, as a page uses it.
func TabItemWrap(n gx.Node) gx.Node {
	return Tabs(TabsProps{Children: n})
}
