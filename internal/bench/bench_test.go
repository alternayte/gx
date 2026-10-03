// Package bench compares the Gx render path with templ (NFR-03).
package bench

import (
	"context"
	"io"
	"testing"

	"github.com/alternayte/gx"
)

func BenchmarkRender(b *testing.B) {
	node := benchPage()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := gx.RenderNode(io.Discard, node); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderTempl(b *testing.B) {
	ctx := context.Background()
	comp := benchTempl()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := comp.Render(ctx, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

// TestNFR_03_RenderBenchmarks runs both render paths once and reports the
// numbers. The 1.5x budget is enforced by `just bench-render` on the
// reference laptop, not on shared CI machines.
func TestNFR_03_RenderBenchmarks(t *testing.T) {
	gxRes := testing.Benchmark(func(b *testing.B) {
		node := benchPage()
		for i := 0; i < b.N; i++ {
			if err := gx.RenderNode(io.Discard, node); err != nil {
				b.Fatal(err)
			}
		}
	})
	templRes := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		comp := benchTempl()
		for i := 0; i < b.N; i++ {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
	t.Logf("gx %d ns/op (%d allocs), templ %d ns/op (%d allocs), ratio %.2f",
		gxRes.NsPerOp(), gxRes.AllocsPerOp(), templRes.NsPerOp(), templRes.AllocsPerOp(),
		float64(gxRes.NsPerOp())/float64(templRes.NsPerOp()))
}

func benchPage() gx.Node {
	rows := make([]gx.Node, 0, 50)
	for i := 0; i < 50; i++ {
		rows = append(rows, gx.El("li", gx.Attrs{{Key: "class", Value: "row"}},
			gx.Value(i), gx.Text(" items")))
	}
	return gx.El("div", gx.Attrs{{Key: "class", Value: "page"}}, gx.Frag(rows...))
}
