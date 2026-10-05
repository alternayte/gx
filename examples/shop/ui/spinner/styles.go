package spinner

// label returns the accessible name; a zero value is Loading.
func (p SpinnerProps) label() string {
	if p.Label == "" {
		return "Loading"
	}
	return p.Label
}
