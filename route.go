package gx

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/alternayte/gx/internal/widgetelement"
)

// Route is embedded in a route input struct. The tag carries the method and
// the pattern: "GET /products/{id}" (REQ-RTE-01).
type Route struct{}

var basePathValue atomic.Value

// SetBasePath sets the prefix of generated links (REQ-RTE-18).
func SetBasePath(path string) {
	basePathValue.Store(strings.TrimSuffix(path, "/"))
}

// BasePath returns the configured link prefix.
func BasePath() string {
	v, _ := basePathValue.Load().(string)
	return v
}

// URL is a prebuilt URL for an href or src (REQ-RTE-05).
type URL string

// URL implements the redirect target interface.
func (u URL) URL() string { return string(u) }

// ParamsFunc reads a path variable from a request (REQ-RTE-17).
type ParamsFunc func(*http.Request, string) string

type paramsKey struct{}

// Params returns middleware that installs a path variable reader for routers
// that do not fill r.PathValue (REQ-RTE-17).
func Params(fn ParamsFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), paramsKey{}, fn)))
		})
	}
}

// PathValue reads a path variable through the Params reader, or through
// r.PathValue when no reader is set.
func PathValue(r *http.Request, name string) string {
	if fn, ok := r.Context().Value(paramsKey{}).(ParamsFunc); ok {
		return fn(r, name)
	}
	return r.PathValue(name)
}

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

	res    *Response
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
	gxID() string
	gxLoad(*Ctx) (any, error)
	gxView(any, Node) Node
}

// layout is the value built by Layout.
type layout[P any] struct {
	id   string
	load func(*Ctx) (P, error)
	view func(P, Node) Node
}

// Layout builds a layout from an optional loader and a view that takes the
// loaded props and the page node (REQ-RTE-08). Pass a nil load when the
// layout needs no data.
func Layout[P any](load func(*Ctx) (P, error), view func(P, Node) Node) layout[P] {
	return layout[P]{id: layoutID(1), load: load, view: view}
}

// layoutID names a layout by its Layout call site. The id is stable for one
// binary, so the client can send back the layouts it holds (REQ-RTE-12).
func layoutID(skip int) string {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "layout"
	}
	// The function name holds the package path. It keeps two layouts apart
	// when their files have the same name and line.
	sum := fnv.New32a()
	if fn := runtime.FuncForPC(pc); fn != nil {
		sum.Write([]byte(fn.Name()))
	}
	return filepath.Base(file) + ":" + strconv.Itoa(line) + "-" + strconv.FormatUint(uint64(sum.Sum32()), 36)
}

func (l layout[P]) gxID() string { return l.id }

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

// Config holds app options. BasePath prefixes every generated link, and
// Adapter selects the hypermedia library (REQ-RTE-18, REQ-ACT-09).
// Package gx exports only the standard library plus an optional adapter
// runtime dependency.
type Config struct {
	BasePath string
	Adapter  Adapter
	// Toast renders one toast (REQ-REG-11). An app sets it to the Render
	// function of its installed toast item, so the toast markup and its
	// classes stay in app-owned source. Without it a toast is the plain
	// ToastNode.
	Toast func(ToastPatch) Node
	// Public holds the app's own static files, for example an embedded
	// public directory. A GET for a path that names a file in it answers
	// that file before any route, so the binary needs no file beside it
	// (NFR-08).
	Public fs.FS
	// Bus carries the values of shared signals between the replicas of
	// the app (REQ-ACT-22). With no bus, the in-memory bus of Gx carries
	// them inside one process.
	Bus Bus
	// Secret signs the room keys of shared signals (SI-17). Each replica
	// of the app needs the same secret. With no secret, the process makes
	// its own.
	Secret []byte
}

// App is an http.Handler that owns a ServeMux (REQ-RTE-18).
type App struct {
	mux        *http.ServeMux
	patterns   map[string]bool
	errorViews map[int]func(*Ctx) Node
	adapter    Adapter
	toast      func(ToastPatch) Node
	// routes records every mounted route for the static export
	// (REQ-EXP-01). It is read only by the dev-only export listing.
	routes []appRoute
	// assets lists the /_gx/ asset names the app serves.
	assets []string
	// public holds the app's own static files (NFR-08).
	public fs.FS
	// origins holds, for each mounted pattern of a group with
	// AllowOrigins, the origins that can call it (REQ-ISL-22).
	origins map[string]*originPolicy
	// widgetTags holds, for the element name of each mounted widget, the
	// configuration of its element file (REQ-ISL-10).
	widgetTags map[string]widgetelement.Config
	// tools holds each mounted tool by its name (REQ-AI-06, SI-07).
	tools map[string]mountedTool
	// hints holds, for the pattern of each page route, the Link values
	// of its last answer with status 200.
	hints sync.Map
	// rooms holds the rooms of the shared signals of this app
	// (REQ-ACT-21).
	rooms *roomHub
}

// appRoute is one mounted route as registered.
type appRoute struct {
	pattern string
	handler Handler
}

// New returns an empty app.
func New(cfg Config) *App {
	SetBasePath(cfg.BasePath)
	SetSecret(cfg.Secret)
	logBus(cfg.Bus)
	a := &App{mux: http.NewServeMux(), patterns: map[string]bool{}, errorViews: map[int]func(*Ctx) Node{}, adapter: cfg.Adapter, toast: cfg.Toast, public: cfg.Public, rooms: newRoomHub(cfg.Bus)}
	a.mux.Handle("GET /_gx/app.css", http.HandlerFunc(a.serveStylesheet))
	a.mux.Handle("GET "+routeSheetPath+"{file}", http.HandlerFunc(serveRouteSheet))
	a.mux.Handle("GET "+islandsPath+"{file...}", http.HandlerFunc(serveIsland))
	a.devRoutes()
	// The theme script needs no adapter: it only reads the stored theme.
	a.registerAsset("theme.js", themeRuntimeJS)
	// An island needs no adapter either (REQ-ISL-04).
	a.registerAsset("island.js", islandRuntimeJS)
	if cfg.Adapter != nil {
		SetAdapter(cfg.Adapter)
		a.registerAssets(cfg.Adapter)
	}
	return a
}

// registerAssets serves the Gx runtime and the adapter runtime files under
// /_gx/ (request lifecycle step 6). BasePath() prefixes the URLs the page
// uses; the mux pattern stays un-prefixed so a mount can strip a prefix
// (REQ-RTE-18).
func (a *App) registerAssets(adapter Adapter) {
	assets := map[string][]byte{
		"gx.js": coreRuntimeJS, "behavior.js": behaviorRuntimeJS,
		"tabs.js": tabsRuntimeJS, "toast.js": toastRuntimeJS, "overlay.js": overlayRuntimeJS,
	}
	for name, data := range adapter.Assets() {
		assets[name] = data
	}
	for name, data := range assets {
		a.registerAsset(name, data)
	}
}

// registerAsset serves one file under /_gx/ and lists it for the export.
func (a *App) registerAsset(name string, body []byte) {
	a.assets = append(a.assets, "/_gx/"+name)
	a.mux.Handle("GET /_gx/"+name, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", assetType(name))
		w.Header().Set("Cache-Control", "public, max-age=3600")
		// A script of the runtime is a public file. A widget loads the
		// island loader from the page of a different origin (REQ-ISL-20).
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = w.Write(body)
	}))
}

// serveStylesheet serves the app stylesheet installed with SetStylesheet
// (REQ-STY-01 to REQ-STY-03).
func (a *App) serveStylesheet(w http.ResponseWriter, r *http.Request) {
	css := Stylesheet()
	if len(css) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(css)
}

// assetType returns the content type of an adapter asset.
func assetType(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	default:
		return "application/octet-stream"
	}
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

// ServeHTTP serves the app with cross-origin protection (SI-03). A page
// response is buffered so the adapter runtime can join it; action responses
// stream through (request lifecycle step 6 and 7).
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.serveListedOrigin(w, r) {
		return
	}
	CSRF(http.HandlerFunc(a.serve)).ServeHTTP(w, r)
}

func (a *App) serve(w http.ResponseWriter, r *http.Request) {
	if name := a.publicFile(r); name != "" {
		http.ServeFileFS(w, r, a.public, name)
		return
	}
	// Buffer a page so the document shell, the stylesheet and the scripts
	// can join it (request lifecycle step 6); action streams pass through.
	if wantsEventStream(r) {
		a.mux.ServeHTTP(w, r)
		return
	}
	a.sendEarlyHints(w, r)
	r, needs := withRuntimeNeeds(r)
	b := newBufferedWriter()
	a.serveBuffered(b, r)
	// The mux set the pattern of the route on the request.
	needs.pattern = r.Pattern
	if needs.signals && a.adapter != nil && !a.adapter.Signals() {
		// gx.toml and Config.Adapter name different adapters, so the
		// compiler did not report the signals (REQ-ACT-09). The page
		// cannot work: say why.
		name := a.adapter.Name()
		http.Error(w, "gx: this page declares signals, and the "+name+" adapter has none. Set adapter = \""+name+
			"\" in gx.toml: the compiler then reports each signal (GX4006).", http.StatusInternalServerError)
		return
	}
	if needs.nonce == "" {
		// A policy around the whole app, or a page that wrote its own
		// response.
		needs.nonce = Nonce(r)
	}
	links := a.flush(w, r, b, needs)
	switch {
	case r.Pattern == "" || r.Header.Get("Gx-Nav") != "":
	case len(links) == 0:
		a.hints.Delete(r.Pattern)
	default:
		a.hints.Store(r.Pattern, links)
	}
}

// serveBuffered runs the route into the buffer. A panic of a loader, a view
// or a handler becomes the 500 answer of REQ-RTE-10: the buffer drops what
// the route wrote, so the browser gets the error component and no part of
// the broken page. A dev build keeps the panic for the overlay (REQ-DEV-06).
func (a *App) serveBuffered(b *bufferedWriter, r *http.Request) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if v == http.ErrAbortHandler || devMode.Load() {
			panic(v)
		}
		log.Printf("gx: panic serving %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
		*b = *newBufferedWriter()
		r = r.WithContext(context.WithValue(r.Context(), errorViewsKey{}, a.errorViews))
		renderError(b, r, fmt.Errorf("gx: panic: %v", v))
	}()
	a.mux.ServeHTTP(b, r)
}

// publicFile returns the name of the public file a request asks for, or ""
// (NFR-08).
func (a *App) publicFile(r *http.Request) string {
	if a.public == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return ""
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" || !fs.ValidPath(name) {
		return ""
	}
	info, err := fs.Stat(a.public, name)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return name
}

// Group mounts routes under a prefix. Middleware applies to the routes that
// follow it in the same call (REQ-RTE-06, REQ-RTE-09). gx.Nav selects the
// navigation mode of the routes that follow it (REQ-RTE-12). gx.AllowOrigins
// and gx.AllowCredentials list the origins that can call the routes that
// follow them from a different origin (REQ-ISL-22).
func (a *App) Group(prefix string, parts ...any) *App {
	var mw []func(http.Handler) http.Handler
	var layouts []layoutDef
	nav := FullNavigation
	var origins *originPolicy
	add := func(h Handler) {
		if shared, ok := h.(*sharedRoute); ok {
			// The rooms of a shared signal belong to the app that
			// mounts its routes (REQ-ACT-22).
			h = shared.forApp(a)
		}
		route := h
		if len(layouts) > 0 {
			route = &layoutHandler{inner: h, layouts: layouts, morph: nav == MorphNavigation}
		}
		if nav == MorphNavigation {
			route = &navHandler{inner: route}
		}
		// Middleware wraps the layout and the navigation, so it runs
		// before any loader of the page (REQ-RTE-09).
		handler := http.Handler(route)
		for i := len(mw) - 1; i >= 0; i-- {
			handler = mw[i](handler)
		}
		handler = a.withErrorViews(handler)
		handler = a.withAdapter(handler)
		handler = a.withToast(handler)
		if m, ok := h.(interface{ mountPrefix(string) }); ok {
			m.mountPrefix(prefix)
		}
		pattern := joinPattern(prefix, h.Pattern())
		if a.patterns[pattern] {
			panic("gx: duplicate route " + pattern)
		}
		if wd, ok := h.(interface {
			widgetTag() string
			widgetAttrs() []string
		}); ok {
			tag := wd.widgetTag()
			if tag == "" {
				panic("gx: the widget " + h.Pattern() + " has no tag. Add .Tag(\"acme-name\") to gx.Widget.")
			}
			if _, ok := a.widgetTags[tag]; ok {
				panic("gx: two widgets have the tag " + tag)
			}
			if a.widgetTags == nil {
				a.widgetTags = map[string]widgetelement.Config{}
				// The first widget of the app: serve the widget
				// script. An app with no widget has no such route.
				a.serveWidgetScript()
				a.serveWidgetFiles()
			}
			_, mounted, _ := strings.Cut(pattern, " ")
			a.widgetTags[tag] = widgetelement.Config{Tag: tag, Attrs: wd.widgetAttrs(), Self: true, Path: mounted}
		}
		if t, ok := h.(interface{ toolDef() *toolDef }); ok && t.toolDef() != nil {
			a.addTool(t.toolDef(), pattern, handler)
		}
		a.patterns[pattern] = true
		a.routes = append(a.routes, appRoute{pattern: pattern, handler: h})
		a.mux.Handle(pattern, handler)
		if origins != nil {
			a.allowOrigins(pattern, origins)
		}
	}
	for _, part := range parts {
		switch v := part.(type) {
		case func(http.Handler) http.Handler:
			mw = append(mw, v)
		case navOption:
			nav = v.mode
		case originsOption:
			origins = origins.with(v)
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
		// A pattern with no method, as a content route has, takes the
		// prefix too.
		if prefix == "" || prefix == "/" {
			return pattern
		}
		return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(pattern, "/")
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
	if r.Context().Err() != nil {
		// The client is gone: no one reads the page.
		return
	}
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
	// morph wraps every layout's children in a named slot (REQ-RTE-12).
	morph bool
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
	if _, ok := h.inner.(nodeHandler); !ok {
		// Actions and forms under a layout group answer themselves.
		h.inner.ServeHTTP(w, r)
		return
	}
	if h.morph && exportRequest(r) {
		// A static host answers no patch: the exported page has no
		// layout slot, and a link is a full load (REQ-EXP-02).
		plain := *h
		plain.morph = false
		h = &plain
	}
	ctx := &Ctx{W: w, R: r}
	page, props, err := h.loadChain(ctx)
	if r.Context().Err() != nil {
		// The client is gone: no layout renders.
		return
	}
	if err != nil {
		renderError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := RenderRequest(w, r, h.wrapFrom(page, props, 0)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// loadChain runs the page loader and every layout loader concurrently
// (REQ-RTE-08).
func (h *layoutHandler) loadChain(ctx *Ctx) (Node, []any, error) {
	nh, ok := h.inner.(nodeHandler)
	if !ok {
		return nil, nil, errors.New("gx: route does not load a node")
	}
	var wg sync.WaitGroup
	var page Node
	var pageErr error
	// A panic in a loader goroutine stops the process. The goroutine
	// keeps the panic value, and the request goroutine panics with it,
	// where net/http and the dev overlay recover it (REQ-DEV-06).
	var panicOnce sync.Once
	var panicked any
	keepPanic := func() {
		if v := recover(); v != nil {
			panicOnce.Do(func() { panicked = v })
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer keepPanic()
		page, pageErr = nh.LoadNode(ctx)
	}()
	props := make([]any, len(h.layouts))
	errs := make([]error, len(h.layouts))
	for i, l := range h.layouts {
		wg.Add(1)
		go func(i int, l layoutDef) {
			defer wg.Done()
			defer keepPanic()
			props[i], errs[i] = l.gxLoad(ctx)
		}(i, l)
	}
	wg.Wait()
	if panicked != nil {
		panic(panicked)
	}
	if pageErr != nil {
		return nil, nil, pageErr
	}
	for _, err := range errs {
		if err != nil {
			return nil, nil, err
		}
	}
	return page, props, nil
}

// layoutIDs returns the layout ids from the root layout inwards
// (REQ-RTE-12).
func (h *layoutHandler) layoutIDs() []string {
	out := make([]string, len(h.layouts))
	for i, l := range h.layouts {
		out[i] = l.gxID()
	}
	return out
}

// wrapFrom wraps the page in the layouts from index from outwards. With
// morph navigation on, each layout's children sit in a named slot so a
// partial navigation can patch the deepest shared layout (REQ-RTE-12).
func (h *layoutHandler) wrapFrom(page Node, props []any, from int) Node {
	n := page
	for i := len(h.layouts) - 1; i >= from; i-- {
		children := n
		if h.morph {
			children = slotNode(h.layouts[i].gxID(), n)
		}
		n = h.layouts[i].gxView(props[i], children)
	}
	return n
}
