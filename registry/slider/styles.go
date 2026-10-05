package slider

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Orientation is the direction of a slider.
type Orientation string

// The orientations of slider.Slider.
const (
	Horizontal Orientation = "horizontal"
	Vertical   Orientation = "vertical"
)

// The track and the thumb of a native range input are pseudo-elements with
// one name per engine. WebKit and Blink have no pseudo-element for the range,
// so the track paints it with a gradient up to --gx-fill; Gecko has
// ::-moz-range-progress.
var orientationClass = gx.Enum[Orientation]{
	Horizontal: "h-1.5 w-full [&::-webkit-slider-runnable-track]:h-1.5 [&::-webkit-slider-runnable-track]:bg-[linear-gradient(to_right,var(--primary)_var(--gx-fill),var(--muted)_var(--gx-fill))] [&::-webkit-slider-thumb]:-mt-[5px] [&::-moz-range-track]:h-1.5 [&::-moz-range-progress]:h-1.5",
	Vertical:   "h-full min-h-44 w-1.5 [direction:rtl] [writing-mode:vertical-lr] [&::-webkit-slider-runnable-track]:w-1.5 [&::-webkit-slider-runnable-track]:bg-[linear-gradient(to_top,var(--primary)_var(--gx-fill),var(--muted)_var(--gx-fill))] [&::-webkit-slider-thumb]:-ml-[5px] [&::-moz-range-track]:w-1.5 [&::-moz-range-progress]:w-1.5",
}

// class returns the classes of one slider.
func (p SliderProps) class() string {
	const base = "block touch-none appearance-none bg-transparent outline-none select-none disabled:pointer-events-none disabled:opacity-50 [&::-webkit-slider-runnable-track]:rounded-full [&::-webkit-slider-thumb]:box-border [&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:border [&::-webkit-slider-thumb]:border-primary [&::-webkit-slider-thumb]:bg-white [&::-webkit-slider-thumb]:shadow-sm [&::-webkit-slider-thumb]:ring-ring/50 [&::-webkit-slider-thumb]:transition-[color,box-shadow] [&::-webkit-slider-thumb]:hover:ring-4 focus-visible:[&::-webkit-slider-thumb]:ring-4 motion-reduce:[&::-webkit-slider-thumb]:transition-none [&::-moz-range-track]:rounded-full [&::-moz-range-track]:bg-muted [&::-moz-range-progress]:rounded-full [&::-moz-range-progress]:bg-primary [&::-moz-range-thumb]:box-border [&::-moz-range-thumb]:size-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border [&::-moz-range-thumb]:border-primary [&::-moz-range-thumb]:bg-white [&::-moz-range-thumb]:shadow-sm [&::-moz-range-thumb]:ring-ring/50 [&::-moz-range-thumb]:transition-[color,box-shadow] [&::-moz-range-thumb]:hover:ring-4 focus-visible:[&::-moz-range-thumb]:ring-4 motion-reduce:[&::-moz-range-thumb]:transition-none"
	orientation := p.Orientation
	if orientation == "" {
		orientation = Horizontal
	}
	return gx.Cx(base, orientationClass[orientation], p.Class)
}

// min returns the minimum value.
func (p SliderProps) min() int {
	return p.Min
}

// max returns the maximum value; a zero value is 100.
func (p SliderProps) max() int {
	if p.Max == 0 {
		return 100
	}
	return p.Max
}

// step returns the step; a zero value is 1.
func (p SliderProps) step() int {
	if p.Step == 0 {
		return 1
	}
	return p.Step
}

// fill returns the custom property that paints the range of the first
// render. The behaviour runtime updates it on input.
func (p SliderProps) fill() gx.Style {
	span := p.max() - p.min()
	v := p.Value - p.min()
	if span <= 0 || v < 0 {
		v, span = 0, 1
	}
	if v > span {
		v = span
	}
	return gx.Style("--gx-fill: " + strconv.Itoa(v*100/span) + "%")
}
