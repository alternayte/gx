package gx

import (
	"fmt"
	"net/http"
	"strings"
)

// Route is embedded in a route input struct. The tag carries the method and
// the pattern: "GET /products/{id}" (REQ-RTE-01).
type Route struct{}

// URL is a prebuilt URL for an href or src (REQ-RTE-05).
type URL string

// Binder is the generated route interface: a pattern and request binding.
// The gx generator writes Pattern and Bind.
type Binder interface {
	Pattern() string
	Bind(*http.Request) error
}

// Ctx is the per-request context (REQ-RTE-01).
type Ctx struct {
	W http.ResponseWriter
	R *http.Request
}

// Handler is one typed route: a pattern plus an http.Handler.
type Handler interface {
	http.Handler
	Pattern() string
}

// Collect returns its arguments as one route list (REQ-RTE-06).
func Collect(hs ...Handler) []Handler { return hs }

// Config holds app options. BasePath and adapters arrive later (REQ-RTE-18).
type Config struct {
	BasePath string
}

// App is an http.Handler that owns a ServeMux (REQ-RTE-18).
type App struct {
	mux      *http.ServeMux
	patterns map[string]bool
}

// New returns an empty app.
func New(cfg Config) *App {
	return &App{mux: http.NewServeMux(), patterns: map[string]bool{}}
}

// ServeHTTP serves the app.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

// Group mounts routes under a prefix. Middleware applies to the routes that
// follow it in the same call (REQ-RTE-06, REQ-RTE-09).
func (a *App) Group(prefix string, parts ...any) *App {
	var mw []func(http.Handler) http.Handler
	add := func(h Handler) {
		handler := http.Handler(h)
		for i := len(mw) - 1; i >= 0; i-- {
			handler = mw[i](handler)
		}
		pattern := joinPattern(prefix, h.Pattern())
		if a.patterns[pattern] {
			panic("gx: duplicate route " + pattern)
		}
		a.patterns[pattern] = true
		a.mux.Handle(pattern, handler)
	}
	for _, part := range parts {
		switch v := part.(type) {
		case func(http.Handler) http.Handler:
			mw = append(mw, v)
		case []Handler:
			for _, h := range v {
				add(h)
			}
		case Handler:
			add(v)
		default:
			panic(fmt.Sprintf("gx: Group does not accept %T", part))
		}
	}
	return a
}

// joinPattern joins a mount prefix and a "METHOD /path" pattern.
func joinPattern(prefix, pattern string) string {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		return pattern
	}
	if prefix == "" || prefix == "/" {
		return method + " /" + strings.TrimPrefix(path, "/")
	}
	return method + " " + strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
}

// page is the typed page handler built by Page (REQ-RTE-04).
type page[In any, P any] struct {
	pattern string
	bind    func(*http.Request) (In, error)
	load    func(*Ctx, In) (P, error)
	view    func(P) Node
}

// Page builds a typed page from a loader and a view. The route input type In
// carries the pattern and the generated Bind method.
func Page[In any, P any](load func(*Ctx, In) (P, error), view func(P) Node) *page[In, P] {
	var zero In
	b, ok := any(&zero).(Binder)
	if !ok {
		panic("gx: Page input type needs generated Pattern and Bind methods")
	}
	pattern := b.Pattern()
	bind := func(r *http.Request) (In, error) {
		var in In
		err := any(&in).(Binder).Bind(r)
		return in, err
	}
	return &page[In, P]{pattern: pattern, bind: bind, load: load, view: view}
}

// Pattern implements Handler.
func (p *page[In, P]) Pattern() string { return p.pattern }

// ServeHTTP binds the input, runs the loader and renders the view.
func (p *page[In, P]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	in, err := p.bind(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := p.load(&Ctx{W: w, R: r}, in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Render(w, p.view(data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
