// Package calendar is a month grid for one day: a native date input that
// the CalendarGrid island shows as a grid of days.
package calendar

// WeekStart is the first day of the week of a calendar.
type WeekStart string

// The first days of the week of calendar.Calendar.
const (
	Sunday WeekStart = "sunday"
	Monday WeekStart = "monday"
)

// CalendarGridProps are the props of the island that draws the grid. The
// island reads and writes the date input with the id Input.
type CalendarGridProps struct {
	// Input is the id of the date input.
	Input string `json:"input"`
	// Month is the first month of the grid, as YYYY-MM, or "".
	Month string `json:"month"`
	// WeekStart is the first day of the week: 0 is Sunday, 1 is Monday.
	WeekStart int `json:"weekStart"`
	// Locale is the language of the names, or "".
	Locale string `json:"locale"`
	// Display is the id of an element that shows the chosen day, or "".
	Display string `json:"display"`
	// Popover is the id of a popover to close after a choice, or "".
	Popover string `json:"popover"`
	// PreviousLabel and NextLabel name the two month buttons.
	PreviousLabel string `json:"previousLabel"`
	NextLabel     string `json:"nextLabel"`
	// Classes holds the classes of the parts. They are Go constants, so
	// the stylesheet build sees them.
	Classes GridClasses `json:"classes"`
}

// GridClasses are the classes of the parts of the grid.
type GridClasses struct {
	Root     string `json:"root"`
	Header   string `json:"header"`
	Nav      string `json:"nav"`
	Title    string `json:"title"`
	Table    string `json:"table"`
	Weekday  string `json:"weekday"`
	Cell     string `json:"cell"`
	Day      string `json:"day"`
	Selected string `json:"selected"`
	Today    string `json:"today"`
	Hidden   string `json:"hidden"`
}

// rootClass is the classes of the root.
const rootClass = "inline-block rounded-md border border-border bg-background p-3 text-foreground"

// inputClass is the classes of the date input with no script.
const inputClass = "h-9 rounded-md border border-input bg-transparent px-3 text-base shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-sm dark:bg-input/30"

// gridClasses are the classes that the island gives to its parts.
var gridClasses = GridClasses{
	Root:     "flex w-fit flex-col gap-3",
	Header:   "flex items-center justify-between gap-2",
	Nav:      "inline-flex size-8 items-center justify-center rounded-md border border-border bg-background text-lg leading-none shadow-xs outline-none hover:bg-accent hover:text-accent-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
	Title:    "text-sm font-medium",
	Table:    "border-collapse",
	Weekday:  "size-8 p-0 text-center text-[0.8rem] font-normal text-muted-foreground",
	Cell:     "size-8 p-0 text-center",
	Day:      "inline-flex size-8 items-center justify-center rounded-md text-sm outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-40",
	Selected: "bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground",
	Today:    "bg-accent text-accent-foreground",
	// The island keeps the date input as the form control and takes it out
	// of view.
	Hidden: "sr-only",
}

// label returns the accessible name of the date input; a zero value is
// "Date".
func (p CalendarProps) label() string {
	if p.Label == "" {
		return "Date"
	}
	return p.Label
}

// month returns the first month of the grid: Month, or the month of Value.
func (p CalendarProps) month() string {
	if p.Month != "" {
		return p.Month
	}
	if len(p.Value) >= 7 {
		return p.Value[:7]
	}
	return ""
}

// weekStart returns the first day of the week as the island reads it.
func (p CalendarProps) weekStart() int {
	if p.WeekStart == Monday {
		return 1
	}
	return 0
}
