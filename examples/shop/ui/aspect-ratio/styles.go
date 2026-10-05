package aspectratio

// ratio returns the CSS ratio; a zero value is 16 / 9.
func (p AspectRatioProps) ratio() string {
	if p.Ratio == "" {
		return "16 / 9"
	}
	return p.Ratio
}
