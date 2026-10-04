// Package images reads and resizes PNG and JPEG content images in pure Go
// (REQ-CNT-11). No external scaler is used, so the export stays buildable
// with the standard library only.
package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
)

// Format is a decoded image format.
type Format string

// The supported content image formats.
const (
	PNG  Format = "png"
	JPEG Format = "jpeg"
)

// Decode reads the size and the format of a PNG or JPEG image.
func Decode(data []byte) (image.Image, Format, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err == nil {
		return img, PNG, nil
	}
	img, err = jpeg.Decode(bytes.NewReader(data))
	if err == nil {
		return img, JPEG, nil
	}
	return nil, "", fmt.Errorf("images: not a PNG or JPEG")
}

// Dimensions returns the pixel size of a PNG or JPEG image.
func Dimensions(data []byte) (int, int, Format, error) {
	img, format, err := Decode(data)
	if err != nil {
		return 0, 0, "", err
	}
	b := img.Bounds()
	return b.Dx(), b.Dy(), format, nil
}

// Resize returns the image scaled to width, keeping the aspect ratio, in
// the same format (REQ-CNT-11). A width at or above the source width
// returns the decoded image as encoded again.
func Resize(data []byte, width int) ([]byte, Format, error) {
	img, format, err := Decode(data)
	if err != nil {
		return nil, "", err
	}
	b := img.Bounds()
	if width <= 0 || width >= b.Dx() {
		width = b.Dx()
	}
	height := b.Dy() * width / b.Dx()
	if height < 1 {
		height = 1
	}
	scaled := bilinear(img, width, height)
	var out bytes.Buffer
	switch format {
	case PNG:
		err = png.Encode(&out, scaled)
	case JPEG:
		err = jpeg.Encode(&out, scaled, &jpeg.Options{Quality: 82})
	}
	if err != nil {
		return nil, "", err
	}
	return out.Bytes(), format, nil
}

// SrcsetWidths returns the widths a srcset of one image uses: 480 and 960
// below the source width, largest first (REQ-CNT-11).
func SrcsetWidths(width int) []int {
	var out []int
	for _, w := range []int{960, 480} {
		if w < width {
			out = append(out, w)
		}
	}
	return out
}

// bilinear scales src to w by h with bilinear sampling.
func bilinear(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := float64(b.Dx()), float64(b.Dy())
	for y := 0; y < h; y++ {
		fy := (float64(y)+0.5)*sh/float64(h) - 0.5
		y0 := int(fy)
		if fy < 0 {
			y0 = 0
			fy = 0
		}
		if y0 > b.Dy()-2 {
			y0 = b.Dy() - 2
		}
		if y0 < 0 {
			y0 = 0
		}
		dy := fy - float64(y0)
		for x := 0; x < w; x++ {
			fx := (float64(x)+0.5)*sw/float64(w) - 0.5
			x0 := int(fx)
			if fx < 0 {
				x0 = 0
				fx = 0
			}
			if x0 > b.Dx()-2 {
				x0 = b.Dx() - 2
			}
			if x0 < 0 {
				x0 = 0
			}
			dx := fx - float64(x0)
			r00, g00, b00, a00 := src.At(b.Min.X+x0, b.Min.Y+y0).RGBA()
			r10, g10, b10, a10 := src.At(b.Min.X+x0+1, b.Min.Y+y0).RGBA()
			r01, g01, b01, a01 := src.At(b.Min.X+x0, b.Min.Y+y0+1).RGBA()
			r11, g11, b11, a11 := src.At(b.Min.X+x0+1, b.Min.Y+y0+1).RGBA()
			r := lerp(lerp(r00, r10, dx), lerp(r01, r11, dx), dy)
			g := lerp(lerp(g00, g10, dx), lerp(g01, g11, dx), dy)
			bl := lerp(lerp(b00, b10, dx), lerp(b01, b11, dx), dy)
			al := lerp(lerp(a00, a10, dx), lerp(a01, a11, dx), dy)
			i := dst.PixOffset(x, y)
			dst.Pix[i] = uint8(r >> 8)
			dst.Pix[i+1] = uint8(g >> 8)
			dst.Pix[i+2] = uint8(bl >> 8)
			dst.Pix[i+3] = uint8(al >> 8)
		}
	}
	return dst
}

// lerp interpolates a 16-bit channel value.
func lerp(a, b uint32, t float64) uint32 {
	return uint32(float64(a) + (float64(b)-float64(a))*t)
}

// Ext returns the file extension of a format, with the dot.
func (f Format) Ext() string {
	switch f {
	case PNG:
		return ".png"
	case JPEG:
		return ".jpg"
	}
	return ""
}

// FromPath returns the format of a file name, or "".
func FromPath(path string) Format {
	switch {
	case strings.HasSuffix(strings.ToLower(path), ".png"):
		return PNG
	case strings.HasSuffix(strings.ToLower(path), ".jpg"), strings.HasSuffix(strings.ToLower(path), ".jpeg"):
		return JPEG
	}
	return ""
}
