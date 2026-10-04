package alertdialog

// cancel returns the label of the cancel button; a zero value is Cancel.
func (p AlertDialogProps) cancel() string {
	if p.Cancel == "" {
		return "Cancel"
	}
	return p.Cancel
}
