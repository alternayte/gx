package selectbox

import "github.com/alternayte/gx"

func plans() gx.Node {
	return gx.Frag(
		SelectOption(SelectOptionProps{Value: "free", Selected: true, Children: gx.Text("Free")}),
		SelectOption(SelectOptionProps{Value: "pro", Children: gx.Text("Pro")}),
		SelectOption(SelectOptionProps{Value: "team", Children: gx.Text("Team")}),
	)
}

var SelectFixtures = gx.Fixtures[SelectProps]{
	"Plan":     {Name: "plan", Label: "Plan", Class: "w-48", Children: plans()},
	"Small":    {Name: "plan", Label: "Plan", Size: Sm, Class: "w-48", Children: plans()},
	"Fit":      {Name: "plan", Label: "Plan", Children: plans()},
	"Disabled": {Name: "plan", Label: "Plan", Class: "w-48", Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: SelectOption(SelectOptionProps{Value: "free", Children: gx.Text("Free")})},
	"Invalid":  {Name: "plan", Label: "Plan", Class: "w-48", Attrs: gx.Attrs{{Key: "aria-invalid", Value: "true"}}, Children: plans()},
	"Placeholder": {Name: "fruit", Label: "Fruit", Placeholder: "Select a fruit", Class: "w-48", Children: gx.Frag(
		SelectGroup(SelectGroupProps{Label: "Fruits", Children: gx.Frag(
			SelectOption(SelectOptionProps{Value: "apple", Children: gx.Text("Apple")}),
			SelectOption(SelectOptionProps{Value: "banana", Children: gx.Text("Banana")}),
		)}),
		SelectSeparator(SelectSeparatorProps{}),
		SelectGroup(SelectGroupProps{Label: "Vegetables", Children: gx.Frag(
			SelectOption(SelectOptionProps{Value: "carrot", Children: gx.Text("Carrot")}),
			SelectOption(SelectOptionProps{Value: "leek", Disabled: true, Children: gx.Text("Leek")}),
		)}),
	)},
}
