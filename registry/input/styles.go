package input

// Control is the classes of an input without its focus ring. A group that
// draws the ring on its own border takes these classes for its control.
const Control = "h-9 w-full min-w-0 rounded-md border border-input bg-transparent px-3 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none selection:bg-primary selection:text-primary-foreground file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40"

// Class is the classes of an input. A control that looks like an input
// takes these classes.
const Class = Control + " focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"

// inputType returns the type of the input; a zero value is text.
func (p InputProps) inputType() string {
	if p.Type == "" {
		return "text"
	}
	return p.Type
}
