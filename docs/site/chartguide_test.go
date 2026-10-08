package site

import (
	"os"
	"strings"
	"testing"
)

// TestREQ_DOC_04_ChartGuideShowsTheLiveIsland checks that the island in the
// guide "A chart with Chart.js" is the island that the page of the guide
// mounts: the code block of the guide holds the bytes of SalesChart.ts.
func TestREQ_DOC_04_ChartGuideShowsTheLiveIsland(t *testing.T) {
	island, err := os.ReadFile("SalesChart.ts")
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile("../content/guides/chart-js.md")
	if err != nil {
		t.Fatal(err)
	}
	block := "```ts title=\"dashboard/SalesChart.ts\"\n" + string(island) + "```\n"
	if !strings.Contains(string(page), block) {
		t.Error("the guide does not show docs/site/SalesChart.ts; copy the file into the ts block of the guide")
	}
	if !strings.Contains(string(page), "<ChartDemo />") {
		t.Error("the guide does not mount the chart demo")
	}
}
