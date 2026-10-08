// Package secretscan finds a secret that encoding/json cannot redact: the
// key of a map (SI-04). encoding/json writes a map key of a string type as
// its text and calls no method of the key. The walk reads a value by
// reflection, as encoding/json does. It runs for a value that goes to an
// agent or a browser as JSON, and never on the render path of a template,
// so package gx keeps its rule of no reflection.
package secretscan

import "reflect"

// HasKeyOfType reports whether v holds a map whose key type is the type of
// key. It walks the value and not only its type: a map behind an interface,
// such as a value of a map[string]any, is found too.
func HasKeyOfType(v, key any) bool {
	if v == nil {
		return false
	}
	return walk(reflect.ValueOf(v), reflect.TypeOf(key), map[uintptr]bool{}, 0)
}

// maxDepth ends the walk of a value that holds itself through a path that
// the pointer set does not see.
const maxDepth = 64

func walk(v reflect.Value, key reflect.Type, seen map[uintptr]bool, depth int) bool {
	if !v.IsValid() || depth > maxDepth {
		return false
	}
	switch v.Kind() {
	case reflect.Interface:
		return walk(v.Elem(), key, seen, depth+1)
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return false
		}
		seen[v.Pointer()] = true
		return walk(v.Elem(), key, seen, depth+1)
	case reflect.Map:
		// An empty map with this key type has no secret to write.
		if v.Type().Key() == key && v.Len() > 0 {
			return true
		}
		for it := v.MapRange(); it.Next(); {
			if walk(it.Key(), key, seen, depth+1) || walk(it.Value(), key, seen, depth+1) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if walk(v.Index(i), key, seen, depth+1) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if walk(v.Field(i), key, seen, depth+1) {
				return true
			}
		}
	}
	return false
}
