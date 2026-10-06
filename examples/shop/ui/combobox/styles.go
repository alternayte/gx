// Package combobox is a select with a text filter: a native select that
// the ComboboxInput island shows as an input with a list of options.
package combobox

// Option is one option of a combobox.
type Option struct {
	// Value is the value that the form sends.
	Value string
	// Label is the text of the option.
	Label string
	// Disabled takes the option out of the choice.
	Disabled bool
}

// ComboboxInputProps are the props of the island that draws the input and
// the list. The island reads the options of the select with the id Select.
type ComboboxInputProps struct {
	// Select is the id of the native select.
	Select string `json:"select"`
	// Placeholder is the text of the empty input.
	Placeholder string `json:"placeholder"`
	// EmptyText is the text of the list with no match.
	EmptyText string `json:"emptyText"`
	// Classes holds the classes of the parts. They are Go constants, so
	// the stylesheet build sees them.
	Classes InputClasses `json:"classes"`
}

// InputClasses are the classes of the parts that the island draws.
type InputClasses struct {
	Root   string `json:"root"`
	Input  string `json:"input"`
	List   string `json:"list"`
	Option string `json:"option"`
	Empty  string `json:"empty"`
	Hidden string `json:"hidden"`
}

// rootClass is the classes of the root. The list is placed below it.
const rootClass = "relative inline-block w-64"

// selectClass is the classes of the select with no script.
const selectClass = "h-9 w-full rounded-md border border-input bg-transparent px-3 text-base shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30"

// inputClasses are the classes that the island gives to its parts.
var inputClasses = InputClasses{
	Root:   "block",
	Input:  "h-9 w-full rounded-md border border-input bg-transparent px-3 text-base shadow-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30",
	List:   "absolute top-full left-0 z-50 mt-1 max-h-60 w-full overflow-y-auto rounded-md border border-border bg-popover p-1 text-popover-foreground shadow-md",
	Option: "cursor-default rounded-sm px-2 py-1.5 text-sm select-none data-[active=true]:bg-accent data-[active=true]:text-accent-foreground aria-selected:font-medium aria-disabled:pointer-events-none aria-disabled:opacity-50",
	Empty:  "absolute top-full left-0 z-50 mt-1 w-full rounded-md border border-border bg-popover px-2 py-6 text-center text-sm text-popover-foreground shadow-md",
	// The island keeps the select as the form control and takes it out of
	// view.
	Hidden: "sr-only",
}

// placeholder returns the text of the empty input; a zero value is
// "Select an option".
func (p ComboboxProps) placeholder() string {
	if p.Placeholder == "" {
		return "Select an option"
	}
	return p.Placeholder
}

// emptyText returns the text of the list with no match; a zero value is
// "No option found.".
func (p ComboboxProps) emptyText() string {
	if p.EmptyText == "" {
		return "No option found."
	}
	return p.EmptyText
}
