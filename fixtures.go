package gx

import "sync"

// Fixtures names example prop sets for the dev gallery (REQ-AI-03). A file
// <Name>.fixtures.go declares one:
//
//	var Fixtures = gx.Fixtures[CardProps]{
//	    "Default": {Title: "Card"},
//	}
type Fixtures[Props any] map[string]Props

// Fixture is one gallery entry. A component without a fixtures file has
// Missing set and no Node.
type Fixture struct {
	Component string
	// Package is the import path of the component, for the dev gallery.
	Package string
	Name    string
	Node    func() Node
	Missing bool
}

var galleryState struct {
	mu       sync.Mutex
	fixtures []Fixture
}

// SetGallery installs the fixture list of the dev gallery (REQ-AI-03). The
// generated gxdev_gallery package supplies it; the scaffold's main calls
// this with gxdev.
func SetGallery(fixtures []Fixture) {
	galleryState.mu.Lock()
	defer galleryState.mu.Unlock()
	galleryState.fixtures = append([]Fixture(nil), fixtures...)
}

// Gallery returns a copy of the installed fixture list.
func Gallery() []Fixture {
	galleryState.mu.Lock()
	defer galleryState.mu.Unlock()
	return append([]Fixture(nil), galleryState.fixtures...)
}
