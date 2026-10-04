package progress

// max returns the maximum value; a zero value is 100.
func (p ProgressProps) max() int {
	if p.Max == 0 {
		return 100
	}
	return p.Max
}
