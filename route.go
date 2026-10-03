package gx

import (
	"context"
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

// URL implements the redirect target interface.
func (u URL) URL() string { return string(u) }

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

// statusError carries an HTTP status (REQ-RTE-10).
type statusError struct {
	status int
	msg    string
}

func (e *statusError) Error() string { return e.msg }

// NotFound tells the page to answer 404.
func NotFound() error { return &statusError{status: http.StatusNotFound, msg: "gx: not found"} }

// Forbidden tells the page to answer 403.
func Forbidden() error { return &statusError{status: http.StatusForbidden, msg: "gx: forbidden"} }

type redirectError struct{ url string }

func (e *redirectError) Error() string { return "gx: redirect to " + e.url }

// Redirect tells the page to answer 303 with a Location (REQ-RTE-10).
func Redirect[In interface{ URL() string }](to In) error {
	return &redirectError{url: to.URL()}
}

// statusFor maps an error to an HTTP status.
func statusFor(err error) int {
	var be *BindError
	if errors.As(err, &be) {
		return http.StatusBadRequest
	}
	var se *statusError
	if errors.As(err, &se) {
		return se.status
	}
	var re *redirectError
	if errors.As(err, &re) {
		return http.StatusSeeOther
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
	mux        *http.ServeMux
	patterns   map[string]bool
	errorViews map[int]func(*Ctx) Node
}

// New returns an empty app.
func New(cfg Config) *App {
	return &App{mux: http.NewServeMux(), patterns: map[string]bool{}, errorViews: map[int]func(*Ctx) Node{}}
}

// Errors sets the error components per status (REQ-RTE-10).
func (a *App) Errors(notFound, forbidden, serverError func(*Ctx) Node) *App {
	a.errorViews[http.StatusNotFound] = notFound
	a.errorViews[http.StatusForbidden] = forbidden
	a.errorViews[http.StatusInternalServerError] = serverError
	return a
}

type errorViewsKey struct{}

// withErrorViews passes the error components to the request.
func (a *App) withErrorViews(h http.Handler) http.Handler {
	if len(a.errorViews) == 0 {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), errorViewsKey{}, a.errorViews)))
	})
}

// renderError answers with a redirect, an error component or a plain status.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	var re *redirectError
	if errors.As(err, &re) {
		http.Redirect(w, r, re.url, http.StatusSeeOther)
		return
	}
	status := statusFor(err)
	views, _ := r.Context().Value(errorViewsKey{}).(map[int]func(*Ctx) Node)
	if view := views[status]; view != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		_ = RenderRequest(w, r, view(&Ctx{W: w, R: r}))
		return
	}
	http.Error(w, http.StatusText(status), status)
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
		handler = a.withErrorViews(handler)
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
	static  func() ([]In, error)
}

// Static records the inputs a static export renders (REQ-RTE-15).
func (p *page[In, P]) Static(fn func() ([]In, error)) *page[In, P] {
	p.static = fn
	return p
}

func (p *page[In, P]) hasStatic() bool { return p.static != nil }

func (p *page[In, P]) staticInputs() ([]any, error) {
	if p.static == nil {
		return nil, nil
	}
	ins, err := p.static()
	if err != nil {
		return nil, err
	}
	out := make([]any, len(ins))
	for i := range ins {
		out[i] = ins[i]
	}
	return out, nil
}

// StaticInputs reports the export inputs of a route, when it lists any.
func StaticInputs(h Handler) ([]any, bool, error) {
	sp, ok := h.(interface {
		staticInputs() ([]any, error)
		hasStatic() bool
	})
	if !ok || !sp.hasStatic() {
		return nil, false, nil
	}
	ins, err := sp.staticInputs()
	return ins, true, err
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
		renderError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := RenderRequest(w, r, n); err != nil {
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

func (h *layoutHandler) hasStatic() bool {
	if sp, ok := h.inner.(interface{ hasStatic() bool }); ok {
		return sp.hasStatic()
	}
	return false
}

func (h *layoutHandler) staticInputs() ([]any, error) {
	if sp, ok := h.inner.(interface{ staticInputs() ([]any, error) }); ok {
		return sp.staticInputs()
	}
	return nil, nil
}

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
		renderError(w, r, pageErr)
		return
	}
	for _, err := range errs {
		if err != nil {
			renderError(w, r, err)
			return
		}
	}
	n := page
	for i := len(h.layouts) - 1; i >= 0; i-- {
		n = h.layouts[i].gxView(props[i], n)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := RenderRequest(w, r, n); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
