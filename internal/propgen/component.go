package propgen

import (
	"math/rand/v2"
	"reflect"
)

// Component is one component function of an app. Package gx has no
// reflection (checks/no-reflection.sh), so the calls are here.
type Component struct {
	Name, Package string
	// fn is the component function; wrap is the <Name>Wrap function of
	// its fixtures file, or the zero value.
	fn, wrap reflect.Value
}

// Props is one prop value of a component.
type Props struct{ v reflect.Value }

func isNode(t reflect.Type) bool { return t.PkgPath() == gxPath && t.Name() == "Node" }

// Find returns the component with the given name from the package-level
// values of its package. ok is false when the values hold no function
// from one props struct to a gx.Node under that name.
func Find(values map[string]reflect.Value, pkgPath, name string) (c Component, ok bool) {
	c = Component{Name: name, Package: pkgPath}
	fn := values[name]
	if !fn.IsValid() || fn.Kind() != reflect.Func {
		return c, false
	}
	ft := fn.Type()
	if ft.NumIn() != 1 || ft.In(0).Kind() != reflect.Struct || ft.NumOut() != 1 || !isNode(ft.Out(0)) {
		return c, false
	}
	c.fn = fn
	if wrap := values[name+"Wrap"]; wrap.IsValid() && wrap.Kind() == reflect.Func {
		wt := wrap.Type()
		if wt.NumIn() == 1 && isNode(wt.In(0)) && wt.NumOut() == 1 && isNode(wt.Out(0)) {
			c.wrap = wrap
		}
	}
	return c, true
}

// CannotMake names the props whose type has only the zero value in a prop
// set.
func (c Component) CannotMake() []string {
	var names []string
	t := c.fn.Type().In(0)
	for i := 0; i < t.NumField(); i++ {
		if f := t.Field(i); f.IsExported() && !CanMake(f.Type) {
			names = append(names, f.Name)
		}
	}
	return names
}

// Set returns the prop set of the component with the given index.
func (c Component) Set(seed uint64, index int, h Hooks) Props {
	a, b := Seed(seed, c.Package, c.Name, index)
	return Props{Set(c.fn.Type().In(0), rand.New(rand.NewPCG(a, b)), ModeOf(index), h)}
}

// Of returns the prop value v of the component, when v has the type of
// its props.
func (c Component) Of(v any) (Props, bool) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Type() != c.fn.Type().In(0) {
		return Props{}, false
	}
	// A copy that can be set.
	p := reflect.New(rv.Type()).Elem()
	p.Set(rv)
	return Props{p}, true
}

// Call calls the component with the props and returns its gx.Node, inside
// the <Name>Wrap function when the fixtures file has one. A panic of the
// component goes to the caller.
func (c Component) Call(p Props) any {
	out := c.fn.Call([]reflect.Value{p.v})[0]
	if c.wrap.IsValid() {
		out = c.wrap.Call([]reflect.Value{out})[0]
	}
	return out.Interface()
}

// Fixture writes the props as an entry value of a gx.Fixtures literal.
func (p Props) Fixture(pkgPath string, h Hooks) (src string, imports []string, err error) {
	return Fixture(p.v, pkgPath, h)
}

// ClearNodes sets each gx.Node of the props to nil.
func (p Props) ClearNodes() bool { return ClearNodes(p.v) }
