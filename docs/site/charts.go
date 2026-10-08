package site

import "encoding/json"

// Point is one bar of the sales chart.
type Point struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// SalesChartProps are the props of the SalesChart island: the island of the
// guide "A chart with Chart.js", which the page of the guide also mounts.
type SalesChartProps struct {
	Title string  `json:"title"`
	Data  []Point `json:"data"`
}

// chartTitle is the title of the chart of the guide.
const chartTitle = "Sales by month"

// chartSets are the numbers of the live chart of the guide. The button of
// the demo shows the next set.
var chartSets = func() [][]Point {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	var sets [][]Point
	for _, values := range [][]int{
		{12, 19, 8, 15, 22, 17},
		{20, 9, 14, 24, 11, 18},
		{7, 16, 23, 10, 19, 25},
	} {
		set := make([]Point, len(months))
		for i, month := range months {
			set[i] = Point{Label: month, Value: values[i]}
		}
		sets = append(sets, set)
	}
	return sets
}()

// chartSetsJSON is the props of the island for each set, as the server
// writes them: the demo script puts the next one into the props attribute.
// The docs site is static, so no action can send new props.
var chartSetsJSON = func() string {
	props := make([]SalesChartProps, len(chartSets))
	for i, set := range chartSets {
		props[i] = SalesChartProps{Title: chartTitle, Data: set}
	}
	data, err := json.Marshal(props)
	if err != nil {
		panic(err)
	}
	return string(data)
}()
