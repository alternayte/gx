package gx

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

	onceMu sync.Mutex
	once   map[string]*onceEntry
}

type onceEntry struct {
	once sync.Once
	val  any
	err  error
}

// Once runs fn once per request and returns its value (REQ-RTE-08). It keys
// on the call site, so share one call site or extract a helper.
func Once[T any](c *Ctx, fn func() (T, error)) (T, error) {
	_, file, line, _ := runtime.Caller(1)
	key := file + ":" + strconv.Itoa(line)
	c.onceMu.Lock()
	if c.once == nil {
		c.once = map[string]*onceEntry{}
	}
	e, ok := c.once[key]
	if !ok {
		e = &onceEntry{}
		c.once[key] = e
	}
	c.onceMu.Unlock()
	e.once.Do(func() { e.val, e.err = fn() })
	if e.err != nil {
		var zero T
		return zero, e.err
	}
	return e.val.(T), nil
}

// BindError marks a request whose input did not bind; the page answers 400.
type BindError struct {
	Err error
}

func (e *BindError) Error() string { return e.Err.Error() }
func (e *BindError) Unwrap() error { return e.Err }

// statusFor maps an error to an HTTP status.
func statusFor(err error) int {
	var be *BindError
	if errors.As(err, &be) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// layoutDef is an untyped layout value.
type layoutDef interface {
	gxLoad(*Ctx) (any, error)
	gxView(any, Node) Node
}

// layout is the value built by Layout.
type layout[P any] struct {
	load func(*Ctx) (P, error)
	view func(P, Node) Node
}

// Layout builds a layout from an optional loader and a view that takes the
// loaded props and the page node (REQ-RTE-08). Pass a nil load when the
// layout needs no data.
func Layout[P any](load func(*Ctx) (P, error), view func(P, Node) Node) layout[P] {
	return layout[P]{load: load, view: view}
}

func (l layout[P]) gxLoad(c *Ctx) (any, error) {
	if l.load == nil {
		return nil, nil
	}
	return l.load(c)
}

func (l layout[P]) gxView(props any, children Node) Node {
	var p P
	if props != nil {
		p = props.(P)
	}
	return l.view(p, children)
}

// nodeHandler is a handler that can produce its node for a layout chain.
type nodeHandler interface {
	Handler
	LoadNode(*Ctx) (Node, error)
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
	var layouts []layoutDef
	add := func(h Handler) {
		handler := http.Handler(h)
		for i := len(mw) - 1; i >= 0; i-- {
			handler = mw[i](handler)
		}
		if len(layouts) > 0 {
			handler = &layoutHandler{inner: h, layouts: layouts}
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
		case layoutDef:
			layouts = append(layouts, v)
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

// LoadNode binds the input, runs the loader and returns the page node.
func (p *page[In, P]) LoadNode(ctx *Ctx) (Node, error) {
	in, err := p.bind(ctx.R)
	if err != nil {
		return nil, &BindError{Err: err}
	}
	data, err := p.load(ctx, in)
	if err != nil {
		return nil, err
	}
	return p.view(data), nil
}

// ServeHTTP binds the input, runs the loader and renders the view.
func (p *page[In, P]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := &Ctx{W: w, R: r}
	n, err := p.LoadNode(ctx)
	if err != nil {
		http.Error(w, err.Error(), statusFor(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Render(w, n); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// layoutHandler runs the page loader and every layout loader concurrently,
// then wraps the page node in the layout chain (REQ-RTE-08).
type layoutHandler struct {
	inner   Handler
	layouts []layoutDef
}

func (h *layoutHandler) Pattern() string { return h.inner.Pattern() }

func (h *layoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nh, ok := h.inner.(nodeHandler)
	if !ok {
		h.inner.ServeHTTP(w, r)
		return
	}
	ctx := &Ctx{W: w, R: r}
	var wg sync.WaitGroup
	var page Node
	var pageErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		page, pageErr = nh.LoadNode(ctx)
	}()
	props := make([]any, len(h.layouts))
	errs := make([]error, len(h.layouts))
	for i, l := range h.layouts {
		wg.Add(1)
		go func(i int, l layoutDef) {
			defer wg.Done()
			props[i], errs[i] = l.gxLoad(ctx)
		}(i, l)
	}
	wg.Wait()
	if pageErr != nil {
		http.Error(w, pageErr.Error(), statusFor(pageErr))
		return
	}
	for _, err := range errs {
		if err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
	}
	n := page
	for i := len(h.layouts) - 1; i >= 0; i-- {
		n = h.layouts[i].gxView(props[i], n)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Render(w, n); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
