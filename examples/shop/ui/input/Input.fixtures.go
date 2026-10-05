package input

import "github.com/alternayte/gx"

var InputFixtures = gx.Fixtures[InputProps]{
	"Text":     {Placeholder: "Email"},
	"Filled":   {Value: "ada@example.com", Attrs: gx.Attrs{{Key: "aria-label", Value: "Email"}}},
	"Disabled": {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Placeholder: "Disabled"},
	"Invalid":  {Value: "ada@", Attrs: gx.Attrs{{Key: "aria-label", Value: "Email"}, {Key: "aria-invalid", Value: "true"}}},
	"File":     {Type: "file", Attrs: gx.Attrs{{Key: "aria-label", Value: "File"}}},
}
