package togglegroup

import "github.com/alternayte/gx"

// align returns three exclusive items. Each fixture passes its own name:
// radios that share a name form one group across the whole gallery page.
func align(name string) gx.Node {
	return gx.Frag(
		ToggleGroupItem(ToggleGroupItemProps{Name: name, Value: "left", Checked: true, Children: gx.Text("Left")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: name, Value: "center", Children: gx.Text("Center")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: name, Value: "right", Children: gx.Text("Right")}),
	)
}

var ToggleGroupFixtures = gx.Fixtures[ToggleGroupProps]{
	"Three":         {Label: "Alignment", Children: align("align")},
	"Outline":       {Label: "Alignment", Variant: Outline, Children: align("align-outline")},
	"Spaced":        {Label: "Alignment", Spacing: 2, Children: align("align-spaced")},
	"OutlineSpaced": {Label: "Alignment", Variant: Outline, Spacing: 2, Children: align("align-outline-spaced")},
	"Small":         {Label: "Alignment", Variant: Outline, Size: Sm, Children: align("align-small")},
	"Large":         {Label: "Alignment", Variant: Outline, Size: Lg, Children: align("align-large")},
	"Multiple": {Label: "Format", Variant: Outline, Children: gx.Frag(
		ToggleGroupItem(ToggleGroupItemProps{Name: "bold", Value: "on", Multiple: true, Checked: true, Children: gx.Text("Bold")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: "italic", Value: "on", Multiple: true, Checked: true, Children: gx.Text("Italic")}),
		ToggleGroupItem(ToggleGroupItemProps{Name: "strike", Value: "on", Multiple: true, Disabled: true, Children: gx.Text("Strike")}),
	)},
}
