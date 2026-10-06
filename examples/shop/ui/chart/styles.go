// Package chart is a bar, line or area chart. The server renders the data
// as a table, and the ChartCanvas island draws it as SVG.
package chart

import "strconv"

// Kind is the drawing of a chart.
type Kind string

// The kinds of chart.Chart.
const (
	Bar  Kind = "bar"
	Line Kind = "line"
	Area Kind = "area"
)

// Series is one row of numbers with a name.
type Series struct {
	// Name is the name of the series in the legend.
	Name string `json:"name"`
	// Values holds one value for each category.
	Values []float64 `json:"values"`
}

// ChartCanvasProps are the props of the island that draws the chart.
type ChartCanvasProps struct {
	// Table is the id of the data table that the island takes out of view.
	Table string `json:"table"`
	// Label is the accessible name of the drawing.
	Label string `json:"label"`
	// Kind is "bar", "line" or "area".
	Kind string `json:"kind"`
	// Categories are the labels of the horizontal axis.
	Categories []string `json:"categories"`
	// Series are the rows of numbers.
	Series []Series `json:"series"`
	// Classes holds the classes of the parts. They are Go constants, so
	// the stylesheet build sees them.
	Classes CanvasClasses `json:"classes"`
}

// CanvasClasses are the classes of the parts that the island draws.
type CanvasClasses struct {
	Root       string `json:"root"`
	Figure     string `json:"figure"`
	SVG        string `json:"svg"`
	Grid       string `json:"grid"`
	Axis       string `json:"axis"`
	Highlight  string `json:"highlight"`
	Tooltip    string `json:"tooltip"`
	Legend     string `json:"legend"`
	LegendItem string `json:"legendItem"`
	Swatch     string `json:"swatch"`
	Hidden     string `json:"hidden"`
}

const rootClass = "flex w-full max-w-lg flex-col gap-2 text-foreground"

const captionClass = "text-sm font-medium"

// tableClass is the classes of the data table with no script.
const tableClass = "w-full border-collapse text-sm"

const cellClass = "border border-border px-2 py-1 text-left font-normal"

var canvasClasses = CanvasClasses{
	Root:       "relative block",
	Figure:     "block rounded-md outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
	SVG:        "block h-auto w-full",
	Grid:       "stroke-border",
	Axis:       "fill-muted-foreground text-[11px]",
	Highlight:  "fill-muted opacity-60",
	Tooltip:    "pointer-events-none absolute top-2 z-10 -translate-x-1/2 rounded-md border border-border bg-popover px-2 py-1.5 text-xs whitespace-nowrap text-popover-foreground shadow-md",
	Legend:     "mt-2 flex flex-wrap items-center justify-center gap-4 text-xs text-muted-foreground",
	LegendItem: "flex items-center gap-1.5",
	Swatch:     "inline-block size-2.5 rounded-xs",
	// The island keeps the table for a screen reader and takes it out of
	// view.
	Hidden: "sr-only",
}

// kind returns the drawing; a zero value is Bar.
func (p ChartProps) kind() Kind {
	switch p.Kind {
	case Line, Area:
		return p.Kind
	}
	return Bar
}

func (p ChartProps) tableID() string { return p.Id + "-data" }

// formatValue writes one number of the table with no trailing zeros.
func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
