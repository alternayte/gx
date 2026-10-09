---
title: "A chart with Chart.js"
description: "A bar chart from the Chart.js npm package in an island: typed props from Go, an entry animation, and bars that move when the server sends new numbers."
section: Guides
order: 15
---

This guide puts a chart from an npm package on a page. The package is [Chart.js](https://www.chartjs.org/). The server gives the numbers as typed props, and an island draws them. The build needs no node.

This is the chart that the guide builds. It runs on this page:

<ChartDemo />

The bars grow from zero when the chart comes into view. The button gives the island new props, and each bar moves to its new value. With the system setting for reduced motion, the chart has no animation.

Read [Islands and web components](/guides/islands/) first for the rules of an island.

## Pin the package

`gx pin` stores the bundled ES module of the package in `js/vendor`, and `gx.lock` records the hash of each file.

```sh
gx pin chart.js@4.5.1/auto
```

The path `/auto` is the entry of Chart.js that registers each chart type. The island imports it as `chart.js/auto`.

## The props

The props of the island are a Go struct. The compiler writes the TypeScript type from it, so the numbers of the server and the code of the chart cannot differ.

```go title="dashboard/charts.go"
package dashboard

// Point is one bar of the sales chart.
type Point struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// SalesChartProps are the props of the SalesChart island.
type SalesChartProps struct {
	Title string  `json:"title"`
	Data  []Point `json:"data"`
}
```

## The island

The island makes one canvas and one chart. `mount` draws the chart with its entry animation. `update` gets new props: it changes the data of the same chart, and Chart.js moves the bars.

```ts title="dashboard/SalesChart.ts"
import Chart from "chart.js/auto";
import type { Mount, Props, Update } from "./SalesChart.props";

// The parts of a Chart.js chart that this island uses. A pinned package has
// the type any.
type BarChart = {
  data: { labels: string[]; datasets: { data: number[] }[] };
  update: () => void;
  destroy: () => void;
};

// One chart for each mounted island.
const charts = new WeakMap<HTMLElement, BarChart>();

const labels = (data: Props["data"]): string[] => data.map((point) => point.label);
const values = (data: Props["data"]): number[] => data.map((point) => point.value);

const mount: Mount = (el, { title, data }) => {
  // Chart.js sizes the canvas from its parent, so the parent has a height.
  el.style.position = "relative";
  el.style.height = "16rem";
  const canvas = el.appendChild(document.createElement("canvas"));
  canvas.setAttribute("role", "img");
  canvas.setAttribute("aria-label", title);
  // No animation for a reader who asks for reduced motion.
  const still = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const chart: BarChart = new Chart(canvas, {
    type: "bar",
    data: {
      labels: labels(data),
      datasets: [{ label: title, data: values(data), backgroundColor: "#2563eb", borderRadius: 4 }],
    },
    options: {
      animation: still ? false : { duration: 600 },
      maintainAspectRatio: false,
      color: "#808080",
      plugins: { legend: { display: false } },
      scales: {
        x: { grid: { display: false }, ticks: { color: "#808080" } },
        y: { beginAtZero: true, ticks: { color: "#808080" }, grid: { color: "rgba(128, 128, 128, 0.25)" } },
      },
    },
  });
  charts.set(el, chart);
  // The cleanup runs when the island leaves the page.
  return () => {
    chart.destroy();
    charts.delete(el);
    el.replaceChildren();
  };
};

export default mount;

// New props arrive here. The chart keeps its canvas and moves each bar to
// its new value.
export const update: Update = ({ data }, el) => {
  const chart = charts.get(el);
  if (!chart) return;
  chart.data.labels = labels(data);
  chart.data.datasets[0].data = values(data);
  chart.update();
};
```

Three parts of the file are for the page and not for Chart.js:

- The cleanup function destroys the chart when the island leaves the page.
- The `animation` option is `false` when the reader asks for reduced motion.
- The canvas has a role and a label, so a screen reader names the chart.

## The page

A tag with the name of the island renders it. The chart sits in a fragment, so an action can send new numbers for that part only.

```gx title="dashboard/Board.gx"
package dashboard

import "acme/dashboard/route"

props {
  Sales []Point
}

<section class="mx-auto max-w-2xl p-6">
  <h1 class="text-xl font-semibold">Sales</h1>
  data := p.Sales
  <div #chart(data []Point)>
    <SalesChart title="Sales by month" data={data} />
  </div>
  <button class="mt-4 rounded-md border border-border px-3 py-1.5 text-sm" on:click={route.Reload{}}>Load new numbers</button>
</section>
```

```go title="dashboard/route/route.go"
// Package route holds the routes of the dashboard slice.
package route

import "github.com/alternayte/gx"

// Board is the page with the chart.
type Board struct {
	gx.Route `GET /board`
}

// Reload sends new numbers to the chart.
type Reload struct {
	gx.Route `POST /board/reload`
}
```

```go title="dashboard/dashboard.go"
package dashboard

import (
	"math/rand/v2"

	"acme/dashboard/route"

	"github.com/alternayte/gx"
)

var months = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}

// sales makes the numbers of the chart. An app reads them from its data.
func sales() []Point {
	points := make([]Point, len(months))
	for i, month := range months {
		points[i] = Point{Label: month, Value: 5 + rand.IntN(20)}
	}
	return points
}

// BoardPage loads the numbers of the chart.
var BoardPage = gx.Page(func(c *gx.Ctx, in route.Board) (BoardProps, error) {
	return BoardProps{Sales: sales()}, nil
}, Board)

// reload patches the fragment of the chart with new numbers.
var reload = gx.Action(func(c *gx.Ctx, in route.Reload) error {
	return c.Patch(BoardChart(sales()))
})

// Routes collects the routes of the slice.
var Routes = gx.Collect(BoardPage, reload)
```

```go title="cmd/app/main.go"
// Command app serves the acme app.
package main

import (
	"log"
	"net/http"
	"os"

	"acme/app"
	"acme/dashboard"
	"acme/gxislands"
	"acme/gxstyles"
	"acme/home"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
)

func main() {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetWidgetStylesheets(gxstyles.Widgets())
	gx.SetIslands(gxislands.Bundle())
	server := gx.New(gx.Config{Adapter: datastar.Adapter()})
	server.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), home.Routes, dashboard.Routes)

	// gx dev sets GX_DEV_ADDR.
	addr := os.Getenv("GX_DEV_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("acme listens on http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, server))
}
```

The server writes the island element with the numbers as JSON:

```html title="GET /board"
<gx-island name="acme/dashboard/SalesChart"
<div data-gx-island-root data-ignore-morph></div></gx-island>
```

## What moves the bars

The button invokes the `Reload` action. The action patches the fragment with new numbers. The morph does not touch the canvas: it changes only the props of the island element. The island loader then calls `update` with the new props, and `chart.update()` animates each bar from its old value to its new value.

An island with no `update` export mounts again with the new props. The chart then starts from zero each time.

The chart on this page is the same island file. This site is static files and has no action. The button of the demo sets the next props on the island element in the browser. The island gets the same `update` call.

## The size of the page

The browser loads Chart.js only for a page with the chart, and only when the chart comes near the viewport: `load="visible"` is the default. A page with no island loads no script of the island.
