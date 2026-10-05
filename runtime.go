package gx

import (
	"context"
	_ "embed"
	"net/http"
	"strings"
)

// coreRuntimeJS is the built Gx browser runtime. `just runtime` rebuilds it
// from runtime/js/gx.ts; `checks/runtime.sh` fails when it is stale.
//
//go:embed runtime/js/gx.js
var coreRuntimeJS []byte

// behaviorRuntimeJS is the built component behaviour runtime (REQ-REG-07).
//
//go:embed runtime/js/behavior.js
var behaviorRuntimeJS []byte

// tabsRuntimeJS, toastRuntimeJS and overlayRuntimeJS are the other built
// behaviour modules (REQ-REG-07). A page loads each one by its own marker.
//
//go:embed runtime/js/tabs.js
var tabsRuntimeJS []byte

//go:embed runtime/js/toast.js
var toastRuntimeJS []byte

//go:embed runtime/js/overlay.js
var overlayRuntimeJS []byte

// themeRuntimeJS is the built theme script. It is a classic script: it runs
// before the first paint and sets the stored theme class on the html
// element.
//
//go:embed runtime/js/theme.js
var themeRuntimeJS []byte

// themeRuntime returns the script tag of the theme script. It has no defer
// and no module type, so the parser runs it before it paints.
func themeRuntime() Node {
	return El("script", Attrs{
		{Key: "src", Value: BasePath() + "/_gx/theme.js", Kind: AttrURL},
	})
}

// coreRuntime returns the script tag of the Gx browser runtime.
func coreRuntime() Node {
	return El("script", Attrs{
		{Key: "type", Value: "module"},
		{Key: "src", Value: BasePath() + "/_gx/gx.js", Kind: AttrURL},
	})
}

// behaviorRuntime returns the script tag of one behaviour module
// (REQ-REG-07): behavior, tabs, toast or overlay.
func behaviorRuntime(module string) Node {
	return El("script", Attrs{
		{Key: "type", Value: "module"},
		{Key: "src", Value: BasePath() + "/_gx/" + module + ".js", Kind: AttrURL},
	})
}

// runtimeNeeds records which optional scripts one page uses (NFR-04). The
// app scans the rendered node tree and injects only what the page needs.
type runtimeNeeds struct {
	// adapter is true when the page uses signals, actions or client
	// expressions.
	adapter bool
	// core is true when the page uses a form, layout-aware navigation or
	// a page-shell behaviour.
	core bool
	// behavior is true when the page uses roving tabindex, a focus trap,
	// dismiss, or open and close (REQ-REG-07).
	behavior bool
	// tabs is true when the page has a tab set.
	tabs bool
	// toast is true when the page has the toaster region. A toast arrives
	// by a later patch, so the region decides.
	toast bool
	// overlay is true when the page has placed content or a sub-menu.
	overlay bool
	// theme is true when the page has a theme control; the stored theme
	// then applies before the first paint.
	theme bool
	// ownDocument is true when the page writes its own html element.
	ownDocument bool
	// shell holds the head of a fragment page for the document shell.
	shell *shellParts
}

// shellParts is what a rendered fragment hands to the document shell.
type shellParts struct {
	head  string
	props HeadProps
}

type runtimeNeedsKey struct{}

// withRuntimeNeeds installs a recorder on the request. RenderRequest fills
// it, the app reads it before the response flushes (NFR-04).
func withRuntimeNeeds(r *http.Request) (*http.Request, *runtimeNeeds) {
	needs := &runtimeNeeds{}
	return r.WithContext(context.WithValue(r.Context(), runtimeNeedsKey{}, needs)), needs
}

// runtimeNeedsOf returns the recorder of the request, or nil.
func runtimeNeedsOf(r *http.Request) *runtimeNeeds {
	if r == nil {
		return nil
	}
	needs, _ := r.Context().Value(runtimeNeedsKey{}).(*runtimeNeeds)
	return needs
}

// scanRuntimeNeeds walks a page and records the markers its elements carry
// (NFR-04).
func scanRuntimeNeeds(n Node) runtimeNeeds {
	var needs runtimeNeeds
	var walk func(Node)
	walk = func(n Node) {
		switch t := n.(type) {
		case fragNode:
			for _, c := range t {
				walk(c)
			}
		case rawNode:
			// Highlighted code frames are raw and carry the copy
			// button; trusted raw HTML may carry other Gx markers.
			if strings.Contains(string(t), "data-gx-") {
				needs.core = true
			}
		case *elNode:
			if t.name == "html" {
				needs.ownDocument = true
			}
			for _, a := range t.attrs {
				if a.Key == "data-gx-theme" {
					needs.theme = true
				}
				switch {
				case adapterMarker(a.Key):
					needs.adapter = true
					needs.core = true
				case interactiveMarker(a.Key):
					needs.adapter = true
					needs.core = true
				case behaviorMarker(a.Key):
					needs.core = true
				case componentBehaviorMarker(a.Key):
					needs.behavior = true
				case tabsMarker(a.Key):
					needs.tabs = true
				case a.Key == "data-gx-toaster":
					needs.toast = true
				case overlayMarker(a.Key):
					needs.overlay = true
				}
			}
			for _, c := range t.children {
				walk(c)
			}
		}
	}
	walk(n)
	return needs
}

// adapterMarker reports whether an attribute activates the hypermedia
// adapter: the attributes the client expression transpiler emits
// (REQ-ACT-07).
func adapterMarker(key string) bool {
	switch key {
	case "data-signals", "data-bind", "data-show", "data-text":
		return true
	}
	for _, prefix := range []string{"data-on", "data-attr:", "data-class:"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// interactiveMarker reports whether an attribute makes a page talk to the
// server: a form, live validation or layout-aware navigation. The answer
// arrives as patches, so the adapter joins the page too (REQ-RTE-12,
// REQ-FRM-05).
func interactiveMarker(key string) bool {
	switch key {
	case "data-gx-slot", "data-gx-form", "data-gx-validate", "data-gx-validate-url":
		return true
	}
	return false
}

// behaviorMarker reports whether an attribute activates a page-shell
// behaviour of the core runtime (REQ-CNT-05, REQ-CNT-06, REQ-CNT-07).
func behaviorMarker(key string) bool {
	switch key {
	case "data-gx-copy", "data-gx-theme", "data-gx-menu",
		"data-gx-search", "data-gx-search-open", "data-gx-search-close",
		"data-gx-search-form", "data-gx-search-input", "data-gx-search-results",
		"data-gx-search-src", "data-gx-toc", "data-gx-toc-target":
		return true
	}
	return false
}

// componentBehaviorMarker reports whether an attribute activates the
// behavior module (REQ-REG-07).
func componentBehaviorMarker(key string) bool {
	switch key {
	case "data-gx-behavior", "data-gx-roving", "data-gx-roving-item", "data-gx-trap", "data-gx-dismiss",
		"data-gx-open", "data-gx-close":
		return true
	}
	return false
}

// tabsMarker reports whether an attribute activates the tabs module
// (REQ-REG-07). The docs kit tabs carry the same markers (REQ-CNT-05).
func tabsMarker(key string) bool {
	switch key {
	case "data-gx-tabs", "data-gx-tab", "data-gx-tab-item", "data-gx-tab-panel":
		return true
	}
	return false
}

// overlayMarker reports whether an attribute activates the overlay module
// (REQ-REG-07): placed content, a sub-menu or the area of a context menu.
func overlayMarker(key string) bool {
	switch key {
	case "data-gx-place", "data-gx-sub", "data-gx-contextmenu":
		return true
	}
	return false
}
