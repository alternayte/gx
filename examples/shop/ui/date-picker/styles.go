// Package datepicker is a button that opens a calendar in a popover.
package datepicker

// triggerClass is the classes of the trigger button: as wide as a date and
// with its text at the start.
const triggerClass = "w-64 justify-start font-normal"

// displayClass is the classes of the text of the trigger. The placeholder
// is muted.
const displayClass = "truncate data-[empty=true]:text-muted-foreground"

// popoverID returns the id of the popover.
func (p DatePickerProps) popoverID() string { return p.Id + "-popover" }

// displayID returns the id of the element that shows the chosen day.
func (p DatePickerProps) displayID() string { return p.Id + "-display" }

// empty reports, as an attribute value, whether no day is chosen.
func (p DatePickerProps) empty() string {
	if p.Value == "" {
		return "true"
	}
	return "false"
}

// text returns the text of the trigger for the first render: the day as
// the server has it, or the placeholder. The calendar island writes the day
// in the language of the user.
func (p DatePickerProps) text() string {
	if p.Value != "" {
		return p.Value
	}
	// A props value that Go code makes has no default.
	if p.Placeholder == "" {
		return "Pick a date"
	}
	return p.Placeholder
}
