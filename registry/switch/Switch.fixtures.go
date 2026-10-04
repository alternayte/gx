package switches

import "github.com/alternayte/gx"

var SwitchFixtures = gx.Fixtures[SwitchProps]{
	"Off":      {Name: "wifi", Label: "Wifi"},
	"On":       {Name: "wifi", Label: "Wifi", Checked: true},
	"Disabled": {Name: "wifi", Label: "Wifi", Attrs: gx.Attrs{gx.Bool("disabled", true)}},
}
