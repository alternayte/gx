package checkbox

// invalid returns the aria-invalid value of the input.
func (p CheckboxProps) invalid() string {
	if p.Invalid {
		return "true"
	}
	return "false"
}
