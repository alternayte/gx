package slider

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
