package propgen

// Report is the answer of the dev route /_gx/fuzz: the prop sets of each
// component of the app, and what the render of each set gave.
type Report struct {
	Seed    uint64   `json:"seed"`
	Targets []Target `json:"targets"`
}

// Target is one component.
type Target struct {
	Component string `json:"component"`
	// Package is the import path of the component.
	Package string `json:"package"`
	// Skipped says why the component has no sets: the symbol table of
	// the app does not hold its function.
	Skipped string `json:"skipped,omitempty"`
	// CannotMake names the props whose type has only the zero value in a
	// prop set: an interface, a function, a channel.
	CannotMake []string `json:"cannotMake,omitempty"`
	Cases      []Case   `json:"cases"`
}

// Case is one prop set of a component.
type Case struct {
	Index int `json:"index"`
	// Fixture is the prop set as an entry value of a gx.Fixtures literal.
	// It is empty when Go source cannot hold the set; Unprintable then
	// says why.
	Fixture     string   `json:"fixture"`
	Imports     []string `json:"imports,omitempty"`
	Unprintable string   `json:"unprintable,omitempty"`
	// Panic is the panic value of the render, or empty.
	Panic string `json:"panic,omitempty"`
	// HTML is what the render wrote.
	HTML string `json:"html"`
	// Slots is true when a gx.Node of the set has a value. Bare is then
	// what the render wrote with each gx.Node set to nil.
	Slots bool   `json:"slots,omitempty"`
	Bare  string `json:"bare,omitempty"`
}
