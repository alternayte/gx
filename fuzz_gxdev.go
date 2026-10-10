//go:build gxdev

package gx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/alternayte/gx/internal/propgen"
)

// fuzzHooks are the node operations of the prop generator.
var fuzzHooks = propgen.Hooks{
	Text: func(s string) any { return Text(s) },
	HTML: func(node any) (string, error) {
		n, ok := node.(Node)
		if !ok {
			return "", fmt.Errorf("the value is not a gx.Node")
		}
		var b bytes.Buffer
		err := RenderNode(&b, n)
		return b.String(), err
	},
}

// fuzzTarget is one component of the gallery. ok is false when the symbol
// table of the app does not hold its function.
type fuzzTarget struct {
	propgen.Component
	ok bool
}

// fuzzTargets returns the components of the gallery, which holds each
// component of the app, with their functions from the symbol table. An
// empty component selects each one.
func fuzzTargets(component, pkg string) []fuzzTarget {
	devState.mu.RLock()
	pkgs := devState.pkgs
	devState.mu.RUnlock()
	seen := map[string]bool{}
	var out []fuzzTarget
	for _, f := range Gallery() {
		if (component != "" && f.Component != component) || (pkg != "" && f.Package != pkg) {
			continue
		}
		id := f.Package + "\x00" + f.Component
		if seen[id] {
			continue
		}
		seen[id] = true
		t := fuzzTarget{Component: propgen.Component{Name: f.Component, Package: f.Package}}
		if p := pkgs[f.Package]; p != nil {
			t.Component, t.ok = propgen.Find(p.Values, f.Package, f.Component)
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package != out[j].Package {
			return out[i].Package < out[j].Package
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// render returns the HTML of the component for the props, or the panic
// value of the render. The request is in scope, as for a page: with no
// request, a link with an empty address is the link of the current page.
func (t fuzzTarget) render(r *http.Request, props propgen.Props) (html, panicked string) {
	defer func() {
		if r := recover(); r != nil {
			html, panicked = "", fmt.Sprint(r)
		}
	}()
	n, _ := t.Call(props).(Node)
	if n == nil {
		n = Frag()
	}
	var b bytes.Buffer
	if err := writeRequest(&b, r, n); err != nil {
		return "", "the render returns the error: " + err.Error()
	}
	return b.String(), ""
}

// fuzzReport renders n prop sets of each selected component (REQ-AI-11).
func fuzzReport(r *http.Request, component, pkg string, seed uint64, n int) propgen.Report {
	report := propgen.Report{Seed: seed, Targets: []propgen.Target{}}
	for _, t := range fuzzTargets(component, pkg) {
		target := propgen.Target{Component: t.Name, Package: t.Package, Cases: []propgen.Case{}}
		if !t.ok {
			target.Skipped = "the symbol table of the app has no component function " + t.Name + "; call gx.SetDevSymbols(gxdev_symbols.Packages())"
			report.Targets = append(report.Targets, target)
			continue
		}
		target.CannotMake = t.CannotMake()
		for index := 0; index < n; index++ {
			props := t.Set(seed, index, fuzzHooks)
			c := propgen.Case{Index: index}
			src, imports, err := props.Fixture(t.Package, fuzzHooks)
			if err != nil {
				c.Unprintable = err.Error()
			} else {
				c.Fixture, c.Imports = src, imports
			}
			c.HTML, c.Panic = t.render(r, props)
			if c.Panic == "" && props.ClearNodes() {
				c.Slots = true
				c.Bare, _ = t.render(r, props)
			}
			target.Cases = append(target.Cases, c)
		}
		report.Targets = append(report.Targets, target)
	}
	return report
}

// fuzzRoutes registers the two routes of `gx fuzz`: the report, and the
// document of one prop set for the audit in Chrome.
func (a *App) fuzzRoutes() {
	args := func(r *http.Request) (component, pkg string, seed uint64, n int, err error) {
		q := r.URL.Query()
		if seed, err = strconv.ParseUint(q.Get("seed"), 10, 64); err != nil {
			return "", "", 0, 0, fmt.Errorf("seed: %w", err)
		}
		if n, err = strconv.Atoi(q.Get("n")); err != nil || n < 0 || n > 10000 {
			return "", "", 0, 0, fmt.Errorf("n: give a number from 0 to 10000")
		}
		return q.Get("component"), q.Get("package"), seed, n, nil
	}
	a.mux.Handle("GET /_gx/fuzz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		component, pkg, seed, n, err := args(r)
		if err != nil {
			http.Error(w, "gx: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fuzzReport(r, component, pkg, seed, n))
	}))
	// n is the index of the one set here.
	a.mux.Handle("GET /_gx/fuzz/render", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		component, pkg, seed, index, err := args(r)
		if err != nil {
			http.Error(w, "gx: "+err.Error(), http.StatusBadRequest)
			return
		}
		targets := fuzzTargets(component, pkg)
		if component == "" || len(targets) != 1 || !targets[0].ok {
			http.Error(w, "gx: no component "+component, http.StatusNotFound)
			return
		}
		t := targets[0]
		html, panicked := t.render(r, t.Set(seed, index, fuzzHooks))
		if panicked != "" {
			http.Error(w, "gx: the render panics: "+panicked, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		f := Fixture{Component: t.Name, Package: t.Package, Name: "fuzz-" + strconv.Itoa(index), Node: func() Node { return Raw(SafeHTML(html)) }} //gx:trusted the HTML is what the component rendered
		// The element around the component is a div: a component can
		// hold the main element of a page.
		_ = RenderRequest(w, r, galleryFixtureHTML(f, r.URL.Query().Get("theme"), "div"))
	}))
}
