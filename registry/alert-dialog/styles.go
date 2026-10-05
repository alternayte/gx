package alertdialog

// Size is the width of an alert dialog.
type Size string

// The sizes of alertdialog.AlertDialog.
const (
	Md Size = "default"
	Sm Size = "sm"
)

// size returns the data-size value; a zero value is Md.
func (p AlertDialogProps) size() string {
	if p.Size == "" {
		return string(Md)
	}
	return string(p.Size)
}

// cancel returns the label of the cancel button; a zero value is Cancel.
func (p AlertDialogProps) cancel() string {
	if p.Cancel == "" {
		return "Cancel"
	}
	return p.Cancel
}
