// Package bench compares the Gx render path with templ (NFR-03).
package bench

import (
	"context"
	"io"
	"testing"

	"github.com/alternayte/gx"
)

// BenchmarkRender builds the page from generated code inside the timed loop
// and writes it, as the templ side does (NFR-03, B-015).
func BenchmarkRender(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := gx.RenderNode(io.Discard, BenchPage(BenchPageProps{Rows: benchRows})); err != nil {
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
		for i := 0; i < b.N; i++ {
			if err := gx.RenderNode(io.Discard, BenchPage(BenchPageProps{Rows: benchRows})); err != nil {
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

// benchRows is the number of rows of the page of each side.
const benchRows = 50
