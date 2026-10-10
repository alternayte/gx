//go:build gxdev

package gx

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync"
)

// devProps holds the props of the last render of each component, by
// package path and name (REQ-AI-12). The dev client saves one as a
// fixture. Each render gets the next number of seq, so the client can ask
// for the components that rendered after a point in time.
var devProps struct {
	mu   sync.Mutex
	seq  uint64
	last map[string]keptProps
}

type keptProps struct {
	props any
	seq   uint64
}

// devKeepProps records the argument of a generated function with one
// argument: a component gets its props so. A fragment function with one
// argument is recorded too; the capture reads only the names of
// components.
func devKeepProps(pkgPath, name string, args []any) {
	if len(args) != 1 {
		return
	}
	devProps.mu.Lock()
	defer devProps.mu.Unlock()
	if devProps.last == nil {
		devProps.last = map[string]keptProps{}
	}
	devProps.seq++
	devProps.last[pkgPath+"\x00"+name] = keptProps{props: args[0], seq: devProps.seq}
}

// propsRoutes registers the two routes of the capture: the list of the
// components that rendered, and the kept props of one component as the Go
// source of a fixture.
func (a *App) propsRoutes() {
	a.mux.Handle("GET /_gx/dev/props", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)
		type component struct {
			Component string `json:"component"`
			Package   string `json:"package"`
		}
		out := struct {
			// Mark is the number of the last render. A later request with
			// since=<mark> lists the components that rendered after it.
			Mark       uint64      `json:"mark"`
			Components []component `json:"components"`
		}{Components: []component{}}
		devProps.mu.Lock()
		out.Mark = devProps.seq
		for _, t := range fuzzTargets("", "") {
			kept, ok := devProps.last[t.Package+"\x00"+t.Name]
			if !ok || kept.seq <= since || !t.ok {
				continue
			}
			if _, ok := t.Of(kept.props); ok {
				out.Components = append(out.Components, component{Component: t.Name, Package: t.Package})
			}
		}
		devProps.mu.Unlock()
		sort.Slice(out.Components, func(i, j int) bool {
			if out.Components[i].Component != out.Components[j].Component {
				return out.Components[i].Component < out.Components[j].Component
			}
			return out.Components[i].Package < out.Components[j].Package
		})
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(out)
	}))
	a.mux.Handle("GET /_gx/dev/props/fixture", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		answer := func(status int, v any) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(v)
		}
		name := q.Get("component")
		targets := fuzzTargets(name, q.Get("package"))
		devProps.mu.Lock()
		var kept keptProps
		found := false
		if name != "" && len(targets) == 1 && targets[0].ok {
			kept, found = devProps.last[targets[0].Package+"\x00"+name]
		}
		devProps.mu.Unlock()
		if !found {
			answer(http.StatusNotFound, map[string]string{"error": "the app has no kept props of the component " + name + "; load a page that shows it"})
			return
		}
		props, ok := targets[0].Of(kept.props)
		if !ok {
			answer(http.StatusNotFound, map[string]string{"error": "the kept value of " + name + " is not its props"})
			return
		}
		src, imports, err := props.Fixture(targets[0].Package, fuzzHooks)
		if err != nil {
			// Go source cannot hold the value: a function, a channel.
			answer(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		answer(http.StatusOK, map[string]any{"fixture": src, "imports": imports})
	}))
}
