package slider

import "github.com/alternayte/gx"

var SliderFixtures = gx.Fixtures[SliderProps]{
	"Half":     {Name: "volume", Value: 50, Label: "Volume"},
	"Full":     {Name: "volume", Value: 100, Label: "Volume"},
	"Empty":    {Name: "volume", Value: 0, Label: "Volume"},
	"Range":    {Name: "price", Min: 10, Max: 200, Value: 80, Step: 10, Label: "Price"},
	"Disabled": {Name: "volume", Value: 50, Label: "Volume", Disabled: true},
	"Vertical": {Name: "level", Value: 50, Label: "Level", Orientation: Vertical, Class: "h-44"},
}
