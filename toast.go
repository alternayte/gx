package gx

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// ToastKind is the kind of one toast (REQ-REG-11). A kind is also a
// ToastOption, so an action passes it directly:
// c.Toast("Saved", gx.ToastSuccess).
type ToastKind uint8

const (
	// ToastDefault is a neutral message with no icon.
	ToastDefault ToastKind = iota
	// ToastSuccess reports a finished operation.
	ToastSuccess
	// ToastInfo gives neutral information.
	ToastInfo
	// ToastWarning reports a result the user must check.
	ToastWarning
	// ToastError reports a failure. It renders as role="alert".
	ToastError
	// ToastLoading reports running work. It stays until a later toast with
	// the same ID replaces it, or the user closes it.
	ToastLoading
)

// String returns the kind as the data-kind value of the toast markup.
func (k ToastKind) String() string {
	switch k {
	case ToastSuccess:
		return "success"
	case ToastInfo:
		return "info"
	case ToastWarning:
		return "warning"
	case ToastError:
		return "error"
	case ToastLoading:
		return "loading"
	}
	return "default"
}

func (k ToastKind) toastOption(p *ToastPatch) { p.Kind = k }

// ToastOption sets one field of a toast (REQ-REG-11).
type ToastOption interface{ toastOption(*ToastPatch) }

type toastOptionFunc func(*ToastPatch)

func (f toastOptionFunc) toastOption(p *ToastPatch) { f(p) }

// ToastDescription adds a second line below the text.
func ToastDescription(text string) ToastOption {
	return toastOptionFunc(func(p *ToastPatch) { p.Description = text })
}

// ToastID names the toast. A later toast with the same ID replaces the
// earlier one in place, so a handler can show loading and then success.
func ToastID(id string) ToastOption {
	return toastOptionFunc(func(p *ToastPatch) { p.ID = id })
}

// ToastDuration sets how long the toast stays. The default is 4 seconds.
func ToastDuration(d time.Duration) ToastOption {
	return toastOptionFunc(func(p *ToastPatch) { p.Duration = d })
}

// ToastSticky keeps the toast until the user closes it.
var ToastSticky ToastOption = toastOptionFunc(func(p *ToastPatch) { p.Sticky = true })

// ToastLink adds one action button that navigates to a route value.
func ToastLink(label string, to interface{ URL() string }) ToastOption {
	return toastOptionFunc(func(p *ToastPatch) { p.Action = ToastAction{Label: label, URL: URL(to.URL())} })
}

// ToastAction is the action button of a toast: a link to URL.
type ToastAction struct {
	Label string
	URL   URL
}

// ToastPatch adds one toast to the toaster region (REQ-REG-11).
type ToastPatch struct {
	Text        string
	Kind        ToastKind
	Description string
	// ID makes a later toast with the same ID replace this one in place.
	ID string
	// Duration is the time before the toast leaves, or 0 for the default.
	Duration time.Duration
	// Sticky keeps the toast until the user closes it.
	Sticky bool
	// Action is the action button. The zero value renders none.
	Action ToastAction
}

// toastDuration is the default time a toast stays.
const toastDuration = 4 * time.Second

// Timeout returns the time before the toast leaves on its own. It is 0 for a
// sticky toast and for a loading toast: they never leave on their own.
func (p ToastPatch) Timeout() time.Duration {
	switch {
	case p.Sticky || p.Kind == ToastLoading:
		return 0
	case p.Duration > 0:
		return p.Duration
	}
	return toastDuration
}

// ToastAttrs returns the attributes the root element of a toast carries: the
// contract between the markup and the behaviour runtime (REQ-REG-11). A
// component that renders a toast spreads them on its root element.
func ToastAttrs(p ToastPatch) Attrs {
	role := "status"
	if p.Kind == ToastError {
		role = "alert"
	}
	attrs := make(Attrs, 0, 5)
	if p.ID != "" {
		attrs = append(attrs, Attr{Key: "id", Value: "gx-toast-" + p.ID})
	}
	return append(attrs,
		Attr{Key: "role", Value: role},
		Attr{Key: "data-gx-toast", Value: ""},
		Attr{Key: "data-kind", Value: p.Kind.String()},
		Attr{Key: "data-duration", Value: strconv.FormatInt(p.Timeout().Milliseconds(), 10)},
	)
}

// ToastNode is the plain markup of one toast. An app that sets Config.Toast
// renders its own component instead.
func ToastNode(p ToastPatch) Node {
	var b Builder
	b.Add(El("div", nil, Text(p.Text)))
	if p.Description != "" {
		b.Add(El("div", nil, Text(p.Description)))
	}
	if p.Action.Label != "" {
		b.Add(El("a", Attrs{{Key: "href", Value: string(p.Action.URL), Kind: AttrURL}}, Text(p.Action.Label)))
	}
	b.Add(El("button", Attrs{
		{Key: "type", Value: "button"},
		{Key: "data-gx-close", Value: ""},
		{Key: "aria-label", Value: "Close"},
	}, Text("×")))
	return El("div", ToastAttrs(p), b.Node())
}

// Toaster renders the plain region that Toast patches into (REQ-REG-11).
func Toaster() Node {
	return El("div", Attrs{
		{Key: "id", Value: "gx-toaster"},
		{Key: "role", Value: "region"},
		{Key: "aria-label", Value: "Notifications"},
		{Key: "aria-live", Value: "polite"},
		{Key: "data-gx-toaster", Value: ""},
	})
}

type toastKey struct{}

// withToast installs the app toast renderer for the request.
func (a *App) withToast(h http.Handler) http.Handler {
	if a.toast == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), toastKey{}, a.toast)))
	})
}

// RenderToast renders one toast for an adapter: with Config.Toast of the app
// that serves the request, or as ToastNode when the app sets none.
func RenderToast(r *http.Request, p ToastPatch) Node {
	if r != nil {
		if render, ok := r.Context().Value(toastKey{}).(func(ToastPatch) Node); ok {
			return render(p)
		}
	}
	return ToastNode(p)
}
