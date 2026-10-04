package input

// inputType returns the type of the input; a zero value is text.
func (p InputProps) inputType() string {
	if p.Type == "" {
		return "text"
	}
	return p.Type
}
