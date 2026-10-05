package textarea

import "github.com/alternayte/gx"

var TextareaFixtures = gx.Fixtures[TextareaProps]{
	"Placeholder": {Placeholder: "Tell us more."},
	"Filled":      {Value: "A short note.", Attrs: gx.Attrs{{Key: "aria-label", Value: "Note"}}},
	"Rows":        {Rows: 6, Placeholder: "Six rows in a browser without field-sizing.", Attrs: gx.Attrs{{Key: "aria-label", Value: "Long note"}}},
	"Invalid":     {Value: "No.", Attrs: gx.Attrs{{Key: "aria-label", Value: "Reason"}, {Key: "aria-invalid", Value: "true"}}},
	"Disabled":    {Attrs: gx.Attrs{gx.Bool("disabled", true)}, Placeholder: "Disabled"},
}
