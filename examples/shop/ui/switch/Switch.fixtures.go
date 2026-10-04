package switches

import "github.com/alternayte/gx"

var SwitchFixtures = gx.Fixtures[SwitchProps]{
	"Off":      {Name: "wifi"},
	"On":       {Name: "wifi", Checked: true},
	"Disabled": {Name: "wifi", Attrs: gx.Attrs{gx.Bool("disabled", true)}},
}
