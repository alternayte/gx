package gx

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Patch is one change an adapter sends to the client (REQ-ACT-04).
type Patch interface{ patch() }

// PatchMode selects how an element patch reaches the DOM.
type PatchMode uint8

const (
	// ModeMorph morphs the node into the existing element. It is the
	// default.
	ModeMorph PatchMode = iota
	// ModeInner replaces the children of the existing element.
	ModeInner
	// ModeAppend puts the node inside the existing element, at the end.
	ModeAppend
	// ModePrepend puts the node inside the existing element, at the start.
	ModePrepend
	// ModeReplace replaces the existing element with the node.
	ModeReplace
	// ModeRemove removes the existing element.
	ModeRemove
)

// ElementPatch changes one element, named by Target, a CSS selector.
type ElementPatch struct {
	Mode       PatchMode
	Target     string
	Node       Node
	Transition bool
}

func (ElementPatch) patch() {}

// SignalPatch sets the signals of one component scope (REQ-ACT-05).
type SignalPatch struct {
	Scope   string
	Signals any
}

func (SignalPatch) patch() {}

// RedirectPatch navigates the client to URL.
type RedirectPatch struct{ URL string }

func (RedirectPatch) patch() {}

func (ToastPatch) patch() {}

// Response is the ordered answer of an action, a form or a navigation.
type Response struct {
	Patches []Patch
	// Status is the HTTP status to answer with, or 0 for the default.
	Status int
	// Err is the handler error, shown by the dev overlay (REQ-DEV-06).
	Err error
	// Navigate marks a partial navigation response (REQ-RTE-12).
	Navigate bool
	// Head is the merged head of a partial navigation (REQ-RTE-12).
	Head *HeadProps
	// tool is the structured output that ToolResult set (REQ-AI-08).
	tool    any
	hasTool bool
}

// patchModeNode marks the mode of the patches that follow it in Ctx.Patch.
type patchModeNode struct{ mode PatchMode }

func (patchModeNode) node() {}

// transitionNode marks that the patches that follow it run inside a view
// transition (REQ-ACT-12).
type transitionNode struct{}

func (transitionNode) node() {}

// Append sends the following patches with append mode.
var Append Node = patchModeNode{ModeAppend}

// Prepend sends the following patches with prepend mode.
var Prepend Node = patchModeNode{ModePrepend}

// Replace sends the following patches with replace mode.
var Replace Node = patchModeNode{ModeReplace}

// Remove sends the following patches with remove mode.
var Remove Node = patchModeNode{ModeRemove}

// ViewTransition wraps the following patches in startViewTransition
// (REQ-ACT-12).
var ViewTransition Node = transitionNode{}

// Patch sends fragment patches by id. The default mode is morph.
func (c *Ctx) Patch(nodes ...Node) error {
	if c.res == nil {
		return errors.New("gx: Patch is only valid in an action or a form")
	}
	mode := ModeMorph
	transition := false
	for _, n := range nodes {
		switch t := n.(type) {
		case patchModeNode:
			mode = t.mode
			continue
		case transitionNode:
			transition = true
			continue
		}
		roots, err := patchRoots(n)
		if err != nil {
			return err
		}
		for _, el := range roots {
			id := attrValue(el, "id")
			if id == "" {
				return fmt.Errorf("gx: patch node <%s> has no id", el.name)
			}
			c.res.Patches = append(c.res.Patches, ElementPatch{
				Mode:       mode,
				Target:     idSelector(id),
				Node:       el,
				Transition: transition,
			})
		}
	}
	return nil
}

// SetSignals updates the signals of the invoking component instance
// (REQ-ACT-05).
func (c *Ctx) SetSignals(v any) error {
	if c.res == nil {
		return errors.New("gx: SetSignals is only valid in an action or a form")
	}
	c.res.Patches = append(c.res.Patches, SignalPatch{Scope: Scope(c.R), Signals: v})
	return nil
}

// Redirect answers with a client navigation to a route value (REQ-ACT-01).
func (c *Ctx) Redirect(to interface{ URL() string }) error {
	if c.res == nil {
		return errors.New("gx: Redirect is only valid in an action or a form")
	}
	c.res.Patches = append(c.res.Patches, RedirectPatch{URL: to.URL()})
	return nil
}

// Toast adds a toast to the toaster region (REQ-ACT-01). A kind is an
// option: c.Toast("Saved", gx.ToastSuccess).
func (c *Ctx) Toast(text string, opts ...ToastOption) error {
	if c.res == nil {
		return errors.New("gx: Toast is only valid in an action or a form")
	}
	p := ToastPatch{Text: text}
	for _, opt := range opts {
		opt.toastOption(&p)
	}
	c.res.Patches = append(c.res.Patches, p)
	return nil
}

// Scope returns the signal scope of the invoking component instance, or ""
// (REQ-ACT-06).
func Scope(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Header.Get("Gx-Scope")
}

// patchRoots returns the top-level elements of a patch node.
func patchRoots(n Node) ([]*elNode, error) {
	switch t := n.(type) {
	case *elNode:
		return []*elNode{t}, nil
	case fragNode:
		var out []*elNode
		for _, child := range t {
			roots, err := patchRoots(child)
			if err != nil {
				return nil, err
			}
			out = append(out, roots...)
		}
		if len(out) == 0 {
			return nil, errors.New("gx: patch node is empty")
		}
		return out, nil
	default:
		return nil, fmt.Errorf("gx: patch node %T is not an element", n)
	}
}

// attrValue returns the value of a named attribute, or "".
func attrValue(el *elNode, name string) string {
	for _, a := range el.attrs {
		if a.Key == name && a.Kind != AttrBool {
			return a.Value
		}
	}
	return ""
}

// idSelector returns the CSS selector of an element id. An id can hold an
// instance key with a dot or a space, which has a meaning in a selector, so
// each such character gets a CSS escape (REQ-ACT-14).
func idSelector(id string) string {
	var b strings.Builder
	b.Grow(len(id) + 1)
	b.WriteByte('#')
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r >= 0x80:
			b.WriteRune(r)
		case r == '-' && len(id) > 1:
			b.WriteRune(r)
		case r >= '0' && r <= '9' && i > 0 && !(i == 1 && id[0] == '-'):
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || (r >= '0' && r <= '9'):
			// A control character or a leading digit takes the code
			// point form, which a space ends.
			b.WriteByte('\\')
			b.WriteString(strconv.FormatInt(int64(r), 16))
			b.WriteByte(' ')
		default:
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	return b.String()
}
