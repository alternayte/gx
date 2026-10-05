package tabs

import "github.com/alternayte/gx"

// demo returns a list of two triggers and their two panels.
func demo(variant Variant, first, second string) gx.Node {
	return gx.Frag(
		TabsList(TabsListProps{Variant: variant, Label: "Settings", Children: gx.Frag(
			TabsTrigger(TabsTriggerProps{Label: first}),
			TabsTrigger(TabsTriggerProps{Label: second}),
		)}),
		TabsContent(TabsContentProps{Label: first, Children: gx.Text(first + " settings.")}),
		TabsContent(TabsContentProps{Label: second, Children: gx.Text(second + " settings.")}),
	)
}

var TabsFixtures = gx.Fixtures[TabsProps]{
	"Two":          {Children: demo(Default, "Account", "Password")},
	"Line":         {Children: demo(Line, "Account", "Password")},
	"Vertical":     {Orientation: Vertical, Children: demo(Default, "Account", "Password")},
	"VerticalLine": {Orientation: Vertical, Children: demo(Line, "Account", "Password")},
	"Default":      {Default: "Password", Children: demo(Default, "Account", "Password")},
	"Synced":       {Sync: "demo-tabs", Children: demo(Default, "First", "Second")},
	"Disabled": {Children: gx.Frag(
		TabsList(TabsListProps{Label: "Settings", Children: gx.Frag(
			TabsTrigger(TabsTriggerProps{Label: "Account"}),
			TabsTrigger(TabsTriggerProps{Label: "Billing", Disabled: true}),
			TabsTrigger(TabsTriggerProps{Label: "Password"}),
		)}),
		TabsContent(TabsContentProps{Label: "Account", Children: gx.Text("Account settings.")}),
		TabsContent(TabsContentProps{Label: "Billing", Children: gx.Text("Billing settings.")}),
		TabsContent(TabsContentProps{Label: "Password", Children: gx.Text("Password settings.")}),
	)},
}
