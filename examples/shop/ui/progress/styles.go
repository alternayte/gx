package progress

import "strconv"

// max returns the maximum value; a zero value is 100.
func (p ProgressProps) max() int {
	if p.Max == 0 {
		return 100
	}
	return p.Max
}

// offset returns the translated percentage of the unfilled part.
func (p ProgressProps) offset() string {
	v := p.Value
	max := p.max()
	if max <= 0 {
		max = 100
	}
	if v < 0 {
		v = 0
	}
	if v > max {
		v = max
	}
	return strconv.Itoa(100 - (v * 100 / max))
}
