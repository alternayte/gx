package textarea

import "github.com/alternayte/gx"

var TextareaFixtures = gx.Fixtures[TextareaProps]{
	"Placeholder": {Placeholder: "Tell us more."},
	"Filled":      {Value: "A short note.", Attrs: gx.Attrs{{Key: "aria-label", Value: "Note"}}},
	"Disabled":    {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Placeholder: "Disabled"},
}
