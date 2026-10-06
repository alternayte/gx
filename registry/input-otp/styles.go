// Package inputotp is a one-time code input: one native input that the
// InputOTPSlots island shows as a row of slots.
package inputotp

import "strconv"

// InputOTPSlotsProps are the props of the island that draws the slots. The
// island reads the value and the caret of the input with the id Input.
type InputOTPSlotsProps struct {
	// Input is the id of the native input.
	Input string `json:"input"`
	// Length is the count of slots.
	Length int `json:"length"`
	// Group is the count of slots before a separator; 0 is no separator.
	Group int `json:"group"`
	// SlotClass, ActiveClass, CaretClass, SeparatorClass and OverlayClass
	// are the classes of the parts. They are Go constants, so the
	// stylesheet build sees them.
	SlotClass      string `json:"slotClass"`
	ActiveClass    string `json:"activeClass"`
	CaretClass     string `json:"caretClass"`
	SeparatorClass string `json:"separatorClass"`
	OverlayClass   string `json:"overlayClass"`
}

// rootClass is the classes of the root. The input and the slots share one
// box, so the island can put the input over the slots.
const rootClass = "relative inline-flex items-center gap-2 has-disabled:opacity-50"

// inputClass is the classes of the native input with no script: a plain
// input with wide letter spacing.
const inputClass = "h-9 w-48 rounded-md border border-input bg-transparent px-3 font-mono text-base tracking-[0.5em] shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed md:text-sm dark:bg-input/30"

// overlayClass replaces the look of the input when the island runs: the
// input covers the slots and shows nothing itself. It still takes the keys,
// the pointer and a paste.
const overlayClass = "absolute inset-0 z-10 h-full w-full cursor-text border-0 bg-transparent p-0 text-transparent caret-transparent opacity-0 shadow-none outline-none disabled:cursor-not-allowed"

// slotClass is the classes of one slot.
const slotClass = "relative -ml-px flex size-9 items-center justify-center border border-input bg-transparent font-mono text-sm shadow-xs transition-[color,box-shadow] first:ml-0 first:rounded-l-md last:rounded-r-md data-[first=true]:ml-0 data-[first=true]:rounded-l-md data-[last=true]:rounded-r-md dark:bg-input/30"

// activeClass is added to the slot that takes the next character while the
// input has the focus.
const activeClass = "z-10 border-ring ring-[3px] ring-ring/50"

// caretClass is the classes of the caret in an empty active slot.
const caretClass = "pointer-events-none h-4 w-px animate-pulse bg-foreground motion-reduce:animate-none"

// separatorClass is the classes of the separator between two groups.
const separatorClass = "mx-1 h-px w-3 bg-muted-foreground"

// length returns the count of characters; a zero value is 6.
func (p InputOTPProps) length() int {
	if p.Length <= 0 {
		return 6
	}
	return p.Length
}

// pattern returns the pattern attribute: exactly length digits.
func (p InputOTPProps) pattern() string {
	return "[0-9]{" + strconv.Itoa(p.length()) + "}"
}
