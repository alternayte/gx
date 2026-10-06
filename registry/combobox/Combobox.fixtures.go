package combobox

import "github.com/alternayte/gx"

var frameworks = []Option{
	{Value: "gx", Label: "Gx"},
	{Value: "templ", Label: "templ"},
	{Value: "gomponents", Label: "gomponents"},
	{Value: "next", Label: "Next.js"},
	{Value: "rails", Label: "Rails", Disabled: true},
	{Value: "sveltekit", Label: "SvelteKit"},
}

var ComboboxFixtures = gx.Fixtures[ComboboxProps]{
	"Empty":    {Id: "combo-empty", Name: "framework", Label: "Framework", Options: frameworks, Placeholder: "Select a framework"},
	"Chosen":   {Id: "combo-chosen", Name: "framework", Label: "Framework", Options: frameworks, Value: "templ"},
	"Disabled": {Id: "combo-disabled", Name: "framework", Label: "Framework", Options: frameworks, Value: "gx", Disabled: true},
}
