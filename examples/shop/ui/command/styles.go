// Package command is a list of commands with a search input: the server
// renders the list, and the CommandInput island filters it and runs an item
// from the keyboard.
package command

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Group is one group of items with a heading.
type Group struct {
	// Heading is the text above the items.
	Heading string
	// Items are the items of the group, in order.
	Items []Item
}

// Item is one command.
type Item struct {
	// Label is the text of the item.
	Label string
	// Value is the value of the command-select event. Empty means Label.
	Value string
	// Href makes the item a link to a page. Empty makes it a button.
	Href gx.URL
	// Keywords are more words that the search finds.
	Keywords string
	// Shortcut is the key hint at the end of the item.
	Shortcut string
	// Disabled takes the item out of the list of choices.
	Disabled bool
}

// CommandInputProps are the props of the island that draws the search
// input. The island filters the items of the list with the id List.
type CommandInputProps struct {
	// List is the id of the list of items.
	List string `json:"list"`
	// Empty is the id of the element that shows when no item matches.
	Empty string `json:"empty"`
	// Label is the accessible name of the input.
	Label string `json:"label"`
	// Placeholder is the text of the empty input.
	Placeholder string `json:"placeholder"`
	// Classes holds the classes of the parts. They are Go constants, so
	// the stylesheet build sees them.
	Classes InputClasses `json:"classes"`
}

// InputClasses are the classes of the parts that the island draws.
type InputClasses struct {
	Root  string `json:"root"`
	Input string `json:"input"`
}

const rootClass = "flex w-full max-w-md flex-col overflow-hidden rounded-md border border-border bg-popover text-popover-foreground shadow-md"

const listClass = "max-h-72 overflow-x-hidden overflow-y-auto p-1"

const groupClass = "overflow-hidden text-foreground"

const headingClass = "px-2 py-1.5 text-xs font-medium text-muted-foreground"

// itemClass is the classes of one item. The island marks the active item
// with data-active; with no script the focus ring marks it.
const itemClass = "flex w-full cursor-default items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-sm outline-none select-none hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground aria-disabled:pointer-events-none aria-disabled:opacity-50"

const shortcutClass = "ml-auto font-sans text-xs tracking-widest text-muted-foreground"

const emptyClass = "py-6 text-center text-sm"

var inputClasses = InputClasses{
	Root:  "block border-b border-border",
	Input: "h-10 w-full bg-transparent px-3 text-sm outline-none placeholder:text-muted-foreground",
}

func (p CommandProps) listID() string  { return p.Id + "-list" }
func (p CommandProps) emptyID() string { return p.Id + "-empty" }

// headingID returns the id of the heading of group i.
func (p CommandProps) headingID(i int) string { return p.Id + "-group-" + strconv.Itoa(i) }

// label returns the accessible name; a zero value is "Commands".
func (p CommandProps) label() string {
	if p.Label == "" {
		return "Commands"
	}
	return p.Label
}

// placeholder returns the text of the empty input.
func (p CommandProps) placeholder() string {
	if p.Placeholder == "" {
		return "Type a command or search..."
	}
	return p.Placeholder
}

// emptyText returns the text for no match.
func (p CommandProps) emptyText() string {
	if p.EmptyText == "" {
		return "No results found."
	}
	return p.EmptyText
}

// value returns the value of the command-select event.
func (it Item) value() string {
	if it.Value == "" {
		return it.Label
	}
	return it.Value
}

// disabled returns the aria-disabled value of an item.
func (it Item) disabled() string {
	if it.Disabled {
		return "true"
	}
	return "false"
}
