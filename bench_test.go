package gx_test

import (
	"io"
	"testing"

	"github.com/alternayte/gx"
)

// BenchmarkRender measures the compiled render path (NFR-03).
func BenchmarkRender(b *testing.B) {
	node := benchPage()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gx.Render(io.Discard, node); err != nil {
			b.Fatal(err)
		}
	}
}

func benchPage() gx.Node {
	rows := make([]gx.Node, 0, 50)
	for i := 0; i < 50; i++ {
		rows = append(rows, gx.El("li", gx.Attrs{{Key: "class", Value: "row"}},
			gx.Value(i), gx.Text(" items")))
	}
	return gx.El("div", gx.Attrs{{Key: "class", Value: "page"}}, gx.Frag(rows...))
}
