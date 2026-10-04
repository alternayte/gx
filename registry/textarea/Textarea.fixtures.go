package textarea

import "github.com/alternayte/gx"

var TextareaFixtures = gx.Fixtures[TextareaProps]{
	"Placeholder": {Placeholder: "Tell us more."},
	"Filled":      {Value: "A short note."},
	"Disabled":    {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Placeholder: "Disabled"},
}
