package chart

import "github.com/alternayte/gx"

var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}

var visitors = []Series{
	{Name: "Desktop", Values: []float64{186, 305, 237, 73, 209, 214}},
	{Name: "Mobile", Values: []float64{80, 200, 120, 190, 130, 140}},
}

var desktop = []Series{
	{Name: "Desktop", Values: []float64{186, 305, 237, 73, 209, 214}},
}

var ChartFixtures = gx.Fixtures[ChartProps]{
	"Bar":  {Id: "chart-bar", Title: "Visitors", Categories: months, Series: visitors},
	"Line": {Id: "chart-line", Title: "Visitors", Kind: Line, Categories: months, Series: visitors},
	"Area": {Id: "chart-area", Title: "Visitors", Kind: Area, Categories: months, Series: desktop},
}
