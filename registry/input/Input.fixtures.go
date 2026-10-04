package input

import "github.com/alternayte/gx"

var InputFixtures = gx.Fixtures[InputProps]{
	"Text":     {Placeholder: "Email"},
	"Filled":   {Value: "ada@example.com"},
	"Disabled": {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Placeholder: "Disabled"},
	"File":     {Type: "file"},
}
