package gx

import "sync"

// stylesheet is the built Tailwind stylesheet of the app (REQ-STY-01 to
// REQ-STY-03). The scaffold's main installs it with gx.SetStylesheet; the
// app serves it at /_gx/app.css and links it in the page head.
var stylesheetState struct {
	mu  sync.RWMutex
	css []byte
}

// SetStylesheet installs the app stylesheet.
func SetStylesheet(css []byte) {
	stylesheetState.mu.Lock()
	defer stylesheetState.mu.Unlock()
	stylesheetState.css = append([]byte(nil), css...)
}

// Stylesheet returns the installed stylesheet, or nil.
func Stylesheet() []byte {
	stylesheetState.mu.RLock()
	defer stylesheetState.mu.RUnlock()
	return append([]byte(nil), stylesheetState.css...)
}

// stylesheetLink returns the link tag of the installed stylesheet.
func stylesheetLink() string {
	if len(Stylesheet()) == 0 {
		return ""
	}
	return `<link rel="stylesheet" href="` + BasePath() + `/_gx/app.css">`
}
