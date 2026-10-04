package textarea

// rows returns the visible rows; a zero value is 4.
func (p TextareaProps) rows() int {
	if p.Rows == 0 {
		return 4
	}
	return p.Rows
}
