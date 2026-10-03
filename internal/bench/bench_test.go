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
		if err := gx.Render(io.Discard, node); err != nil {
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

func TestNFR_03_RenderWithinTemplBudget(t *testing.T) {
	run := func(f func(*testing.B)) int64 {
		best := int64(-1)
		for i := 0; i < 2; i++ {
			r := testing.Benchmark(f)
			if ns := r.NsPerOp(); best < 0 || ns < best {
				best = ns
			}
		}
		return best
	}
	gxNs := run(func(b *testing.B) {
		node := benchPage()
		for i := 0; i < b.N; i++ {
			if err := gx.Render(io.Discard, node); err != nil {
				b.Fatal(err)
			}
		}
	})
	templNs := run(func(b *testing.B) {
		ctx := context.Background()
		comp := benchTempl()
		for i := 0; i < b.N; i++ {
			if err := comp.Render(ctx, io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	})
	ratio := float64(gxNs) / float64(templNs)
	t.Logf("gx %d ns/op, templ %d ns/op, ratio %.2f", gxNs, templNs, ratio)
	if ratio > 1.5 {
		t.Fatalf("compiled render is %.2fx templ, budget is 1.5x", ratio)
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
