// Package secretscan finds a secret that encoding/json cannot redact: the
// key of a map (SI-04). encoding/json writes a map key of a string type as
// its text and calls no method of the key. The walk reads types by
// reflection, as encoding/json does. It runs for a value that goes to an
// agent or a browser as JSON, and never on the render path of a template,
// so package gx keeps its rule of no reflection.
package secretscan

import "reflect"

// HasKeyOfType reports whether the type of v holds a map whose key type is
// the type of key.
func HasKeyOfType(v, key any) bool {
	if v == nil {
		return false
	}
	return walk(reflect.TypeOf(v), reflect.TypeOf(key), map[reflect.Type]bool{})
}

func walk(t, key reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Map:
		return t.Key() == key || walk(t.Key(), key, seen) || walk(t.Elem(), key, seen)
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return walk(t.Elem(), key, seen)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if walk(t.Field(i).Type, key, seen) {
				return true
			}
		}
	}
	return false
}
