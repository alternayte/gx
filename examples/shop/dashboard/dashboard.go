// Package dashboard shows TypeScript islands: one for each load strategy,
// two that share a module, one that follows a signal and one that keeps its
// state when the server patches its props (REQ-ISL-03 to REQ-ISL-06).
package dashboard

import (
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/examples/shop/dashboard/route"
)

// Point is one bar of a chart.
type Point struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// BarChartProps are the props of the BarChart island.
type BarChartProps struct {
	Round int     `json:"round"`
	Data  []Point `json:"data"`
}

// SparklineProps are the props of the Sparkline island.
type SparklineProps struct {
	Data []Point `json:"data"`
}

// StepperProps are the props of the Stepper island. Qty is a signal of the
// page: the island and the input change the same value.
type StepperProps struct {
	Qty gx.SignalRef[int] `json:"qty"`
}

// LegendProps are the props of the Legend island.
type LegendProps struct {
	Labels []string `json:"labels"`
}

// WideTableProps are the props of the WideTable island.
type WideTableProps struct {
	Data []Point `json:"data"`
}

// Revenue returns the numbers of one round. The numbers follow the round,
// so a test can predict them.
func Revenue(round int) []Point {
	labels := []string{"Jan", "Feb", "Mar", "Apr"}
	out := make([]Point, len(labels))
	for i, label := range labels {
		out[i] = Point{Label: label, Value: (i+1)*10 + round}
	}
	return out
}

// DashboardPage is the dashboard page.
var DashboardPage = gx.Page(
	func(c *gx.Ctx, in route.Page) (DashboardProps, error) {
		return DashboardProps{Revenue: Revenue(0)}, nil
	},
	Dashboard)

// Refresh answers with the chart panel of the next round. The islands in
// the panel get new props and keep their DOM (REQ-ISL-06).
var Refresh = gx.Action(func(c *gx.Ctx, in route.Refresh) error {
	return c.Patch(ChartsPanel(ChartsProps{Revenue: Revenue(in.Round), Round: in.Round}))
})

// Routes collects the dashboard page and its action.
var Routes = gx.Collect(DashboardPage, Refresh)
