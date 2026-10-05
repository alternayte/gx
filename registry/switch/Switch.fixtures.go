package switches

import "github.com/alternayte/gx"

var SwitchFixtures = gx.Fixtures[SwitchProps]{
	"Off":      {Name: "wifi", Label: "Wifi"},
	"On":       {Name: "wifi", Label: "Wifi", Checked: true},
	"Disabled": {Name: "wifi", Label: "Wifi", Disabled: true},
	"Small":    {Name: "wifi", Label: "Wifi", Size: Sm},
	"SmallOn":  {Name: "wifi", Label: "Wifi", Size: Sm, Checked: true},
}
