// Package ruleiter iterates slice values for the rule engine. It lives
// outside package gx so the runtime render path stays free of reflection.
package ruleiter

import "reflect"

// Each calls fn for every element of a slice or array value. Non-slice
// values are ignored.
func Each(v any, fn func(any) error) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
		return nil
	}
	for i := 0; i < rv.Len(); i++ {
		if err := fn(rv.Index(i).Interface()); err != nil {
			return err
		}
	}
	return nil
}
