package gx

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
)

// Adapter is the public hook between Gx and one hypermedia library
// (REQ-PLG-04). One adapter serves one app (P2).
type Adapter interface {
	// Name identifies the adapter, for example "datastar".
	Name() string
	// Signals reports whether the adapter supports client signals and
	// client expressions (REQ-ACT-09).
	Signals() bool
	// Runtime returns the script nodes every page needs (request
	// lifecycle step 6), or nil.
	Runtime() Node
	// Assets returns static files keyed by their name below /_gx/.
	Assets() map[string][]byte
	// Respond writes the commands of an action or a navigation.
	Respond(w http.ResponseWriter, r *http.Request, res *Response) error
	// ReadSignals decodes the request signals into dst, a pointer.
	ReadSignals(r *http.Request, dst any) error
}

var adapterMu sync.RWMutex
var adapterDefault Adapter

// SetAdapter sets the adapter of the process. New does this from
// Config.Adapter. Actions mounted outside a gx.App use the default.
func SetAdapter(a Adapter) {
	adapterMu.Lock()
	adapterDefault = a
	adapterMu.Unlock()
}

// AdapterOf returns the adapter of the request, or the process default.
func AdapterOf(r *http.Request) Adapter {
	if r != nil {
		if a, ok := r.Context().Value(adapterKey{}).(Adapter); ok {
			return a
		}
	}
	adapterMu.RLock()
	defer adapterMu.RUnlock()
	return adapterDefault
}

type adapterKey struct{}

// withAdapter installs the app adapter for the request.
func (a *App) withAdapter(h http.Handler) http.Handler {
	if a.adapter == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next := r.WithContext(context.WithValue(r.Context(), adapterKey{}, a.adapter))
		h.ServeHTTP(w, next)
	})
}

// wantsEventStream reports whether the request asks for patches rather than
// a page (request lifecycle step 7).
func wantsEventStream(r *http.Request) bool {
	if r.Header.Get("Datastar-Request") != "" {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

// runtimeScripts returns the adapter script tags for one page.
func (a *App) runtimeScripts() Node {
	if a.adapter == nil {
		return nil
	}
	return a.adapter.Runtime()
}

// bufferedWriter captures a page response so the app can add the adapter
// runtime and the CSRF data before the response flushes.
type bufferedWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newBufferedWriter() *bufferedWriter {
	return &bufferedWriter{header: http.Header{}}
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	return b.body.Write(p)
}

// flush injects the adapter runtime and writes the buffered response.
func (a *App) flush(w http.ResponseWriter, b *bufferedWriter, needs *runtimeNeeds) {
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	body := b.body.Bytes()
	ct := b.header.Get("Content-Type")
	if status == http.StatusOK && strings.Contains(ct, "text/html") {
		body = a.inject(body, needs)
	}
	for k, vs := range b.header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	if ct == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// inject adds the Gx runtime and the adapter scripts to a page. A page with
// no signals, actions, forms, navigation or behaviours ships no JS at all
// (NFR-04).
func (a *App) inject(page []byte, needs *runtimeNeeds) []byte {
	var b bytes.Buffer
	if link := stylesheetLink(); link != "" {
		b.WriteString(link)
	}
	if a.adapter != nil && needs != nil {
		// Signals, actions and server answers need both scripts: the
		// runtime carries the CSRF wrapper and applies the frames the
		// adapter receives (SI-03). A behaviour needs only the core
		// runtime, and a plain page needs neither.
		if needs.adapter || needs.core {
			b.WriteString(String(coreRuntime()))
		}
		if needs.adapter {
			b.WriteString(String(a.runtimeScripts()))
		}
	}
	add := b.Bytes()
	if len(add) == 0 {
		return page
	}
	for _, marker := range []string{"</head>", "</HEAD>", "</Head>"} {
		if i := bytes.Index(page, []byte(marker)); i >= 0 {
			out := make([]byte, 0, len(page)+len(add))
			out = append(out, page[:i]...)
			out = append(out, add...)
			out = append(out, page[i:]...)
			return out
		}
	}
	for _, marker := range []string{"</body>", "</BODY>", "</Body>"} {
		if i := bytes.Index(page, []byte(marker)); i >= 0 {
			out := make([]byte, 0, len(page)+len(add))
			out = append(out, page[:i]...)
			out = append(out, add...)
			out = append(out, page[i:]...)
			return out
		}
	}
	out := make([]byte, 0, len(page)+len(add))
	out = append(out, page...)
	out = append(out, add...)
	return out
}
