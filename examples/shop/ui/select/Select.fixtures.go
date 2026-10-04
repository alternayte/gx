package selectbox

import "github.com/alternayte/gx"

var SelectFixtures = gx.Fixtures[SelectProps]{
	"Plan": {Name: "plan", Label: "Plan", Class: "w-48", Children: gx.Frag(
		SelectOption(SelectOptionProps{Value: "free", Selected: true, Children: gx.Text("Free")}),
		SelectOption(SelectOptionProps{Value: "pro", Children: gx.Text("Pro")}),
		SelectOption(SelectOptionProps{Value: "team", Children: gx.Text("Team")}),
	)},
	"Disabled": {Name: "plan", Label: "Plan", Class: "w-48", Attrs: gx.Attrs{gx.Bool("disabled", true)}, Children: SelectOption(SelectOptionProps{Value: "free", Children: gx.Text("Free")})},
}
