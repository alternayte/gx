package gx

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Key identifies one component instance (REQ-ACT-06).
type Key string

// InstanceKey returns the key of a component call site from its key
// expression.
func InstanceKey(v any) Key { return Key(TextValue(v)) }

// ChildKey returns the key of an unkeyed call site under parent. The same
// call site gives the same key on every render (REQ-ACT-06).
func ChildKey(parent Key, index int) Key {
	part := strconv.Itoa(index)
	if parent == "" {
		return Key(part)
	}
	return parent + Key("."+part)
}

// ScopeKey returns the instance key inside a scope for a component base,
// for example "42" for base "cart.Cart" and scope "cart.Cart.42"
// (REQ-ACT-06). It returns "" when the scope is not that component.
func ScopeKey(scope, base string) Key {
	if scope == base {
		return ""
	}
	prefix := base + "."
	if !strings.HasPrefix(scope, prefix) {
		return ""
	}
	return Key(scope[len(prefix):])
}

// ScopeString returns the signal namespace of a component instance: the
// component path plus its key (REQ-ACT-06).
func ScopeString(base string, key Key) string {
	if key == "" {
		return base
	}
	return base + "." + string(key)
}

// SignalJSON renders the data-signals JSON of one component instance from
// the initial values of its signals (REQ-ACT-05).
func SignalJSON(base string, key Key, values map[string]any) string {
	root := map[string]any{}
	node := root
	for _, part := range scopePath(base, key) {
		next := map[string]any{}
		node[part] = next
		node = next
	}
	for name, v := range values {
		node[name] = v
	}
	data, err := json.Marshal(root)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// SignalPath returns the adapter signal reference of one signal in one
// component instance, for example $["cart"]["Cart"]["42"]["qty"]
// (REQ-ACT-07).
func SignalPath(base string, key Key, name string) string {
	return "$" + SignalRefPath(base, key, name)
}

// SignalName returns the dotted name of one signal, as data-bind takes it
// (REQ-ACT-07), for example cart.Cart.42.qty.
func SignalName(base string, key Key, name string) string {
	return ScopeString(base, key) + "." + name
}

// SignalRefPath returns the bracket path of one signal, without the leading
// $, for example ["cart"]["Cart"]["42"]["qty"].
func SignalRefPath(base string, key Key, name string) string {
	var b strings.Builder
	for _, part := range append(scopePath(base, key), name) {
		b.WriteByte('[')
		b.WriteString(strconv.Quote(part))
		b.WriteByte(']')
	}
	return b.String()
}

// SignalRef is a prop that carries a parent signal reference into a child
// component (REQ-ACT-07).
type SignalRef[T any] string

// Ref builds a signal reference value from a bracket path.
func Ref[T any](path string) SignalRef[T] { return SignalRef[T](path) }

// RefPath returns the adapter reference of a signal ref, with the leading
// $, so a child can share a parent signal.
func RefPath[T any](r SignalRef[T]) string { return "$" + string(r) }

// scopePath splits a component scope into its path segments. Dots separate
// the component path and the key parts.
func scopePath(base string, key Key) []string {
	parts := []string{}
	for _, p := range strings.Split(base, ".") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if key != "" {
		for _, p := range strings.Split(string(key), ".") {
			if p != "" {
				parts = append(parts, p)
			}
		}
	}
	return parts
}

// FragmentID returns the id of one fragment instance: component-scoped, and
// keyed when the instance has a key (REQ-AUT-13).
func FragmentID(component, name string, key Key) string {
	id := component + "-" + name
	if key != "" {
		id += "-" + string(key)
	}
	return id
}

// Signals decodes the request signals through the app adapter
// (REQ-ACT-03). It returns nil when no adapter is set.
func Signals(r *http.Request) (map[string]any, error) {
	adapter := AdapterOf(r)
	if adapter == nil {
		return nil, nil
	}
	m := map[string]any{}
	if err := adapter.ReadSignals(r, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// BindSignal fills dst from the named signal inside scope, for example
// scope "cart.Cart" and name "qty" (REQ-ACT-03). A signal that is not in the
// request leaves dst unchanged.
func BindSignal(signals map[string]any, scope, name string, dst any) error {
	v, ok := signalValue(signals, scope, name)
	if !ok {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return &BindError{Err: err}
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return &BindError{Err: err}
	}
	return nil
}

// signalValue walks the dotted scope path and returns the named signal.
func signalValue(signals map[string]any, scope, name string) (any, bool) {
	var node any = signals
	for _, part := range scopeParts(scope) {
		m, ok := node.(map[string]any)
		if !ok {
			return nil, false
		}
		node, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := m[name]
	return v, ok
}

// scopeParts splits a scope path. Signal names are matched exactly, so the
// scope is split on dots only.
func scopeParts(scope string) []string {
	if scope == "" {
		return nil
	}
	parts := strings.Split(scope, ".")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
