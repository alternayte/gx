package gx

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alternayte/gx/internal/elementname"
)

// widget is the typed widget handler built by Widget (REQ-ISL-10).
type widget[In any, P any] struct {
	pattern string
	tag     string
	bind    func(*http.Request) (In, error)
	load    func(*Ctx, In) (P, error)
	view    func(P) Node
}

// Widget builds a widget from a loader and a view (REQ-ISL-10, DR-09). A
// widget is a component that a page of a different site uses as a custom
// element. This server renders it.
//
// The element sends its attributes as the query of the GET route of In, and
// puts the HTML of the answer into its shadow root. The fields of In are the
// attributes of the element (REQ-ISL-15). The loader makes the props; the
// host never sends props.
//
// Tag gives the widget its element name. Mount the widget in a Group with
// gx.AllowOrigins, or no page of a different origin can load it.
func Widget[In any, P any](load func(*Ctx, In) (P, error), view func(P) Node) *widget[In, P] {
	var zero In
	b, ok := any(&zero).(Binder)
	if !ok {
		panic("gx: Widget input type needs generated Pattern and Bind methods")
	}
	pattern := b.Pattern()
	if method, _, _ := strings.Cut(pattern, " "); method != http.MethodGet {
		panic("gx: Widget input type needs a GET route; " + pattern + " is not one")
	}
	bind := func(r *http.Request) (In, error) {
		var in In
		err := any(&in).(Binder).Bind(r)
		return in, err
	}
	return &widget[In, P]{pattern: pattern, bind: bind, load: load, view: view}
}

// Tag sets the name of the custom element, for example "acme-cart". It
// panics for a name that the browser does not accept for a custom element.
func (wd *widget[In, P]) Tag(name string) *widget[In, P] {
	if problem := elementname.Problem(name); problem != "" {
		panic("gx: Widget tag " + quoteTag(name) + ": " + problem + ". Use a name such as acme-cart.")
	}
	wd.tag = name
	return wd
}

// Pattern implements Handler.
func (wd *widget[In, P]) Pattern() string { return wd.pattern }

// widgetTag returns the element name of the widget, for the mount check.
func (wd *widget[In, P]) widgetTag() string { return wd.tag }

// ServeHTTP binds the attributes, checks the rules, runs the loader and
// answers with the HTML of the view as JSON.
func (wd *widget[In, P]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in, err := wd.bind(r)
	if err != nil {
		writeWidgetError(w, http.StatusBadRequest, "gx.bad_attribute", "")
		return
	}
	// The host is not trusted: its attributes pass the rules of the
	// input before the loader reads them (DR-07).
	if fv := RunRulesContext(r.Context(), &in); fv != nil {
		writeWidgetError(w, http.StatusBadRequest, fv.Key, fv.Field)
		return
	}
	props, err := wd.load(&Ctx{W: w, R: r}, in)
	if err != nil {
		status := statusFor(err)
		writeWidgetError(w, status, widgetErrorKey(status), "")
		return
	}
	writeWidgetJSON(w, http.StatusOK, widgetAnswer{Tag: wd.tag, HTML: StringRequest(r, wd.view(props))})
}

// widgetAnswer is the first answer of a widget route.
type widgetAnswer struct {
	Tag   string       `json:"tag,omitempty"`
	HTML  string       `json:"html,omitempty"`
	Error *widgetError `json:"error,omitempty"`
}

// widgetError is the error of a widget answer. It holds a status and a
// message key, and no text of the server: the host is a different origin
// (REQ-ISL-16).
type widgetError struct {
	Status int    `json:"status"`
	Key    string `json:"key"`
	Field  string `json:"field,omitempty"`
}

func widgetErrorKey(status int) string {
	switch status {
	case http.StatusNotFound:
		return "gx.not_found"
	case http.StatusForbidden:
		return "gx.forbidden"
	case http.StatusUnauthorized:
		return "gx.unauthorized"
	}
	return "gx.error"
}

func writeWidgetError(w http.ResponseWriter, status int, key, field string) {
	writeWidgetJSON(w, status, widgetAnswer{Error: &widgetError{Status: status, Key: key, Field: field}})
}

func writeWidgetJSON(w http.ResponseWriter, status int, a widgetAnswer) {
	body, err := json.Marshal(a)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	// The answer is for one user and one build.
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func quoteTag(name string) string {
	b, _ := json.Marshal(name)
	return string(b)
}
