package gx

import (
	"net/http"
	"strings"
)

// Navigation selects how a link between two pages with a shared layout
// loads (REQ-RTE-12).
type Navigation uint8

const (
	// FullNavigation loads every link as a full page.
	FullNavigation Navigation = iota
	// MorphNavigation fetches only the slot of the deepest shared layout.
	MorphNavigation
)

// Nav sets the navigation mode of the routes that follow it in a Group
// (REQ-RTE-12).
func Nav(mode Navigation) navOption { return navOption{mode: mode} }

type navOption struct{ mode Navigation }

// slotNode is the layout slot a partial navigation patches. The
// display:contents keeps the layout's own box model.
func slotNode(id string, children Node) Node {
	return El("div", Attrs{
		{Key: "data-gx-slot", Value: id},
		{Key: "style", Value: "display:contents"},
	}, children)
}

// chainLoader is a handler that can load its layout chain for a partial
// navigation.
type chainLoader interface {
	loadChain(*Ctx) (Node, []any, error)
	layoutIDs() []string
	wrapFrom(Node, []any, int) Node
}

// navHandler answers a click between two pages that share a layout with a
// slot patch instead of a full page (REQ-RTE-12).
type navHandler struct {
	inner   Handler
	layouts bool
}

func (h *navHandler) Pattern() string { return h.inner.Pattern() }

func (h *navHandler) hasStatic() bool {
	if sp, ok := h.inner.(interface{ hasStatic() bool }); ok {
		return sp.hasStatic()
	}
	return false
}

func (h *navHandler) staticInputs() ([]any, error) {
	if sp, ok := h.inner.(interface{ staticInputs() ([]any, error) }); ok {
		return sp.staticInputs()
	}
	return nil, nil
}

func (h *navHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	chain, ok := h.inner.(chainLoader)
	if !ok || r.Header.Get("Gx-Nav") == "" {
		h.inner.ServeHTTP(w, r)
		return
	}
	ctx := &Ctx{W: w, R: r}
	page, props, err := chain.loadChain(ctx)
	if err != nil {
		renderError(w, r, err)
		return
	}
	serverIDs := chain.layoutIDs()
	clientIDs := splitLayouts(r.Header.Get("Gx-Layouts"))
	common := commonPrefix(serverIDs, clientIDs)
	if len(serverIDs) > 0 && common == 0 {
		// No shared layout: fall back to a full load (REQ-RTE-12).
		w.Header().Set("Gx-Nav", "full")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = RenderRequest(w, r, chain.wrapFrom(page, props, 0))
		return
	}
	full := chain.wrapFrom(page, props, 0)
	head := HeadOf(full)
	subtree := chain.wrapFrom(page, props, common)
	res := &Response{Navigate: true, Head: &head}
	if common > 0 {
		res.Patches = append(res.Patches, ElementPatch{
			Mode:   ModeInner,
			Target: `[data-gx-slot="` + serverIDs[common-1] + `"]`,
			Node:   subtree,
		})
	} else {
		res.Patches = append(res.Patches, ElementPatch{Mode: ModeInner, Target: "body", Node: subtree})
	}
	adapter := AdapterOf(r)
	if adapter == nil {
		http.Error(w, "gx: navigation needs an adapter", http.StatusInternalServerError)
		return
	}
	if err := adapter.Respond(w, r, res); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// splitLayouts parses the comma list a client sends back.
func splitLayouts(header string) []string {
	if header == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(header, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// commonPrefix returns the number of equal leading entries.
func commonPrefix(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}
