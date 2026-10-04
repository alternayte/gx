package images_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/alternayte/gx/internal/images"
)

// TestREQ_CNT_11_Resize covers the pure-Go scaler: dimensions, aspect
// ratio, format and the srcset widths (REQ-CNT-11).
func TestREQ_CNT_11_Resize(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for x := 0; x < 1000; x++ {
		for y := 0; y < 500; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	w, h, format, err := images.Dimensions(data)
	if err != nil || w != 1000 || h != 500 || format != images.PNG {
		t.Fatalf("Dimensions = %d, %d, %s, %v", w, h, format, err)
	}

	variant, format, err := images.Resize(data, 500)
	if err != nil {
		t.Fatal(err)
	}
	if format != images.PNG {
		t.Fatalf("format = %s", format)
	}
	decoded, _, err := image.Decode(bytes.NewReader(variant))
	if err != nil {
		t.Fatal(err)
	}
	b := decoded.Bounds()
	if b.Dx() != 500 || b.Dy() != 250 {
		t.Fatalf("variant = %dx%d, want 500x250", b.Dx(), b.Dy())
	}

	widths := images.SrcsetWidths(1000)
	if len(widths) != 2 || widths[0] != 960 || widths[1] != 480 {
		t.Fatalf("SrcsetWidths = %v", widths)
	}
	if got := images.SrcsetWidths(600); len(got) != 1 || got[0] != 480 {
		t.Fatalf("SrcsetWidths(600) = %v", got)
	}
	if got := images.SrcsetWidths(200); len(got) != 0 {
		t.Fatalf("SrcsetWidths(200) = %v", got)
	}
}
