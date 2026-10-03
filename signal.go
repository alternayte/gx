package gx

import (
	"encoding/json"
	"net/http"
	"strings"
)

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
