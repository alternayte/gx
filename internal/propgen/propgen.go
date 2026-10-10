// Package propgen makes prop values for a component from its prop type and
// writes a prop value as Go source. `gx fuzz` renders the values
// (REQ-AI-11); the capture of the dev client writes a value into a fixtures
// file (REQ-AI-12). The package runs in a dev build of an app only.
package propgen

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"time"
)

// gxPath is the import path of package gx. The package cannot import gx,
// so it finds the types of gx by name.
const gxPath = "github.com/alternayte/gx"

// Hooks are the two operations on a gx.Node that the package needs.
type Hooks struct {
	// Text returns gx.Text(s).
	Text func(s string) any
	// HTML returns the HTML that a gx.Node renders.
	HTML func(node any) (string, error)
}

// Mode selects the strings of one prop set.
type Mode int

const (
	// Zero is the zero value of each prop: each string is empty.
	Zero Mode = iota
	// Long gives each string a long value.
	Long
	// Markup gives each string a value with markup characters.
	Markup
	// Mixed selects one of the kinds for each string at random.
	Mixed
)

// ModeOf returns the mode of the prop set with the given index. The first
// three sets hold the empty value, a long string and markup characters in
// each string; each later set is mixed.
func ModeOf(index int) Mode {
	if index < 0 || index > int(Mixed) {
		return Mixed
	}
	return Mode(index)
}

// LongString and MarkupString are the two fixed string values.
var (
	LongString   = strings.TrimSpace(strings.Repeat("Lorem ipsum dolor sit amet ", 80))
	MarkupString = `<b>"x" & 'y'</b><script>alert(1)</script></div>`
)

var words = []string{"a", "Save", "Order 1042", "Zürich", "two words", "0"}

// maxDepth stops a prop type that holds itself.
const maxDepth = 4

// Seed returns the two seed words of the generator of one prop set. The
// sets of one component do not change when a different component joins
// the run.
func Seed(seed uint64, pkgPath, component string, index int) (uint64, uint64) {
	// FNV-1a.
	h := uint64(14695981039346656037)
	for _, s := range []string{pkgPath, "\x00", component} {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
	}
	return seed, h + uint64(index)
}

// Set returns a value of the prop type t.
func Set(t reflect.Type, rng *rand.Rand, mode Mode, h Hooks) reflect.Value {
	v := reflect.New(t).Elem()
	if mode == Zero {
		return v
	}
	g := generator{rng: rng, mode: mode, hooks: h}
	g.fill(v, 0)
	return v
}

type generator struct {
	rng   *rand.Rand
	mode  Mode
	hooks Hooks
}

func (g *generator) str() string {
	mode := g.mode
	if mode == Mixed {
		mode = Mode(g.rng.IntN(4))
	}
	switch mode {
	case Zero:
		return ""
	case Long:
		return LongString
	case Markup:
		return MarkupString
	}
	return words[g.rng.IntN(len(words))]
}

func pick[T any](g *generator, values ...T) T {
	return values[g.rng.IntN(len(values))]
}

// fill sets v, which is addressable, to a generated value.
func (g *generator) fill(v reflect.Value, depth int) {
	t := v.Type()
	if depth > maxDepth || !v.CanSet() {
		return
	}
	if t.PkgPath() == gxPath {
		switch t.Name() {
		case "Node":
			if g.hooks.Text != nil && g.rng.IntN(4) != 0 {
				v.Set(reflect.ValueOf(g.hooks.Text(g.str())))
			}
			return
		case "Attrs":
			if g.rng.IntN(2) == 0 {
				return
			}
			attrs := reflect.MakeSlice(t, 1, 1)
			attrs.Index(0).FieldByName("Key").SetString("data-fuzz")
			attrs.Index(0).FieldByName("Value").SetString(g.str())
			v.Set(attrs)
			return
		case "URL":
			v.SetString(pick(g, "", "/fuzz", "/fuzz?a=1&b=2"))
			return
		case "Key":
			v.SetString(pick(g, "", "k1"))
			return
		case "SafeHTML", "Style", "Room":
			// The app vouches for these values; a random value is not
			// an input that a caller can give.
			return
		}
	}
	if t == reflect.TypeFor[time.Time]() {
		if g.rng.IntN(2) == 0 {
			v.Set(reflect.ValueOf(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)))
		}
		return
	}
	switch t.Kind() {
	case reflect.String:
		v.SetString(g.str())
	case reflect.Bool:
		v.SetBool(g.rng.IntN(2) == 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// The numbers are small: a component can use one as a length.
		v.SetInt(pick[int64](g, 0, 1, -1, 2, 10, 100))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(pick[uint64](g, 0, 1, 2, 10, 100))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(pick(g, 0, 1, -1, 0.5, 1234.5))
	case reflect.Slice:
		n := pick(g, 0, 1, 3)
		if n == 0 {
			return
		}
		s := reflect.MakeSlice(t, n, n)
		for i := 0; i < n; i++ {
			g.fill(s.Index(i), depth+1)
		}
		v.Set(s)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			g.fill(v.Index(i), depth+1)
		}
	case reflect.Map:
		n := g.rng.IntN(3)
		if n == 0 {
			return
		}
		m := reflect.MakeMapWithSize(t, n)
		for i := 0; i < n; i++ {
			key := reflect.New(t.Key()).Elem()
			g.fill(key, depth+1)
			elem := reflect.New(t.Elem()).Elem()
			g.fill(elem, depth+1)
			m.SetMapIndex(key, elem)
		}
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			g.fill(v.Field(i), depth+1)
		}
	case reflect.Pointer:
		if g.rng.IntN(2) == 0 {
			return
		}
		p := reflect.New(t.Elem())
		g.fill(p.Elem(), depth+1)
		v.Set(p)
	}
	// An interface, a function and a channel stay nil.
}

// CanMake reports whether Set can give a value other than the zero value to
// a prop of the type t. It cannot for an interface other than gx.Node, a
// function, a channel, a value that the app vouches for, and a struct with
// no exported field. `gx fuzz` does not render a component with a required
// prop of such a type: the zero value is not an input that a caller gives.
func CanMake(t reflect.Type) bool { return canMake(t, 0) }

func canMake(t reflect.Type, depth int) bool {
	if depth > maxDepth {
		return false
	}
	if t.PkgPath() == gxPath {
		switch t.Name() {
		case "Node", "Attrs", "URL", "Key":
			return true
		case "SafeHTML", "Style", "Room":
			return false
		}
	}
	if t == reflect.TypeFor[time.Time]() {
		return true
	}
	switch t.Kind() {
	case reflect.Interface, reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128, reflect.Invalid:
		return false
	case reflect.Slice, reflect.Array, reflect.Pointer:
		return canMake(t.Elem(), depth+1)
	case reflect.Map:
		return canMake(t.Key(), depth+1) && canMake(t.Elem(), depth+1)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if f := t.Field(i); f.IsExported() && canMake(f.Type, depth+1) {
				return true
			}
		}
		return false
	}
	return true
}

// ClearNodes sets each gx.Node of the prop value v to nil, and reports
// whether one had a value. v is addressable. `gx fuzz` renders the result
// to see whether a defect of the tree comes from the content of a slot or
// from the component.
func ClearNodes(v reflect.Value) bool { return clearNodes(v, 0) }

func clearNodes(v reflect.Value, depth int) bool {
	if depth > 32 {
		return false
	}
	t := v.Type()
	if t.PkgPath() == gxPath && t.Name() == "Node" {
		if v.IsNil() || !v.CanSet() {
			return false
		}
		v.SetZero()
		return true
	}
	changed := false
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			changed = clearNodes(v.Field(i), depth+1) || changed
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			changed = clearNodes(v.Index(i), depth+1) || changed
		}
	case reflect.Pointer:
		if !v.IsNil() {
			changed = clearNodes(v.Elem(), depth+1)
		}
	}
	return changed
}
