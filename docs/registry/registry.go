// Package registry lists the registry items of the docs site and renders
// their examples (REQ-DOC-02). `just docs-gen` writes items_gen.go from the
// registry source; the dev gallery list is gxdev-only and cannot ship.
package registry

import "github.com/alternayte/gx"

// Item is one registry item.
type Item struct {
	Name        string
	Title       string
	Group       string
	Description string
	// Block is true for a block. A block preview fills its frame.
	Block bool
	// IconSet is true when every component of the item is one icon.
	IconSet  bool
	Examples []Example
}

// Example is one live preview of an item.
type Example struct {
	// Name is the URL segment of the example.
	Name  string
	Title string
	// Component is the component of the first fixture.
	Component string
	// Fixtures lists the fixtures the example renders, as
	// "Component/Fixture".
	Fixtures []string
	// Toast is true when the example is one toast that an action pushes.
	// The preview shows it on a button press.
	Toast bool
	Node  func() gx.Node
}

// Find returns the item with the given name.
func Find(name string) (Item, bool) {
	for _, it := range Items {
		if it.Name == name {
			return it, true
		}
	}
	return Item{}, false
}

// Example returns the example of the item with the given name.
func (it Item) Example(name string) (Example, bool) {
	for _, ex := range it.Examples {
		if ex.Name == name {
			return ex, true
		}
	}
	return Example{}, false
}
