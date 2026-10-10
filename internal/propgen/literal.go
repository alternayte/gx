package propgen

import (
	"fmt"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Fixture writes the prop value v as the value of one entry of a
// gx.Fixtures literal: a composite literal with no type, for example
// `{Title: "Card", Count: 3}`. pkgPath is the package of the fixtures file.
// imports holds the import of each package that the source names: the
// import path, or `name "path"` when two packages of the source have one
// name.
//
// A gx.Node becomes a gx.Raw call with the HTML that the node renders. The
// error names the prop that Go source cannot hold: a function, a channel
// or a field of a different package that is not exported.
func Fixture(v reflect.Value, pkgPath string, h Hooks) (src string, imports []string, err error) {
	if v.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("the props are a %s, not a struct", v.Kind())
	}
	w := writer{pkgPath: pkgPath, hooks: h, imports: map[string]string{}}
	body, err := w.fields(v, "")
	if err != nil {
		return "", nil, err
	}
	for path, name := range w.imports {
		if name == "" {
			imports = append(imports, path)
		} else {
			// Two packages have one name: the second has a name of
			// its own in the import.
			imports = append(imports, name+" "+strconv.Quote(path))
		}
	}
	sort.Strings(imports)
	return "{" + body + "}", imports, nil
}

type writer struct {
	pkgPath string
	hooks   Hooks
	// imports holds, for each import path, the name of the import when
	// it is not the name of the package.
	imports map[string]string
	// names holds the import path of each package name in use.
	names map[string]string
}

// use returns the name that the source has for a package.
func (w *writer) use(path, name string) string {
	if alias, ok := w.imports[path]; ok {
		if alias != "" {
			return alias
		}
		return name
	}
	if w.names == nil {
		w.names = map[string]string{}
	}
	alias := ""
	for n := 2; w.names[name+alias] != ""; n++ {
		alias = strconv.Itoa(n)
	}
	w.names[name+alias] = path
	if alias != "" {
		alias = name + alias
	}
	w.imports[path] = alias
	if alias != "" {
		return alias
	}
	return name
}

// cannot is the error of a value with no Go source.
func cannot(path, what string) error {
	return fmt.Errorf("prop %s: Go source cannot hold %s", strings.TrimPrefix(path, "."), what)
}

// qualify returns the name of a declared type as the fixtures file writes
// it.
func (w *writer) qualify(t reflect.Type, path string) (string, error) {
	if strings.Contains(t.Name(), "[") {
		return "", cannot(path, "a value of the generic type "+t.String())
	}
	if t.PkgPath() == "" || t.PkgPath() == w.pkgPath {
		return t.Name(), nil
	}
	if !token.IsExported(t.Name()) {
		return "", cannot(path, "a value of the type "+t.String()+", which its package does not export")
	}
	// reflect gives the package name before the dot.
	name := t.String()
	return w.use(t.PkgPath(), name[:strings.Index(name, ".")]) + "." + t.Name(), nil
}

// typeExpr returns the Go source of a type.
func (w *writer) typeExpr(t reflect.Type, path string) (string, error) {
	if t.Name() != "" {
		return w.qualify(t, path)
	}
	switch t.Kind() {
	case reflect.Slice:
		elem, err := w.typeExpr(t.Elem(), path)
		return "[]" + elem, err
	case reflect.Array:
		elem, err := w.typeExpr(t.Elem(), path)
		return "[" + strconv.Itoa(t.Len()) + "]" + elem, err
	case reflect.Pointer:
		elem, err := w.typeExpr(t.Elem(), path)
		return "*" + elem, err
	case reflect.Map:
		key, err := w.typeExpr(t.Key(), path)
		if err != nil {
			return "", err
		}
		elem, err := w.typeExpr(t.Elem(), path)
		return "map[" + key + "]" + elem, err
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return "any", nil
		}
	}
	return "", cannot(path, "a value of the type "+t.String()+", which has no name")
}

// fields writes the non-zero fields of a struct value.
func (w *writer) fields(v reflect.Value, path string) (string, error) {
	t := v.Type()
	var parts []string
	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		fv := v.Field(i)
		if fv.IsZero() {
			continue
		}
		fpath := path + "." + f.Name
		if !f.IsExported() && t.PkgPath() != w.pkgPath {
			return "", cannot(fpath, "a field that is not exported")
		}
		src, err := w.value(fv, fpath, 0)
		if err != nil {
			return "", err
		}
		parts = append(parts, f.Name+": "+src)
	}
	return strings.Join(parts, ", "), nil
}

// value writes v for a place whose static type is the type of v.
func (w *writer) value(v reflect.Value, path string, depth int) (string, error) {
	if depth > 32 {
		return "", cannot(path, "a value that holds itself")
	}
	t := v.Type()
	if t.PkgPath() == gxPath && t.Name() == "Node" {
		if v.IsNil() {
			return "nil", nil
		}
		if !v.CanInterface() || w.hooks.HTML == nil {
			return "", cannot(path, "a node in a field that is not exported")
		}
		html, err := w.hooks.HTML(v.Interface())
		if err != nil {
			return "", fmt.Errorf("prop %s: the node does not render: %w", strings.TrimPrefix(path, "."), err)
		}
		return w.use(gxPath, "gx") + ".Raw(" + strconv.Quote(html) + ")", nil
	}
	if t == reflect.TypeFor[time.Time]() {
		if v.IsZero() {
			return w.use("time", "time") + ".Time{}", nil
		}
		if !v.CanInterface() {
			return "", cannot(path, "a time in a field that is not exported")
		}
		// The time keeps its zone: a page shows the time of day of
		// the zone.
		tm := v.Interface().(time.Time)
		pkg := w.use("time", "time")
		zone := pkg + ".UTC"
		if name, offset := tm.Zone(); tm.Location() != time.UTC {
			zone = fmt.Sprintf("%s.FixedZone(%s, %d)", pkg, strconv.Quote(name), offset)
		}
		return fmt.Sprintf("%s.Date(%d, %d, %d, %d, %d, %d, %d, %s)", pkg,
			tm.Year(), int(tm.Month()), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), tm.Nanosecond(), zone), nil
	}
	switch t.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if f != f || f > 1.7e308 || f < -1.7e308 {
			return "", cannot(path, "a number that is not finite")
		}
		return strconv.FormatFloat(f, 'g', -1, t.Bits()), nil
	case reflect.String:
		return strconv.Quote(v.String()), nil
	case reflect.Struct:
		name, err := w.typeExpr(t, path)
		if err != nil {
			return "", err
		}
		body, err := w.fields(v, path)
		return name + "{" + body + "}", err
	case reflect.Pointer:
		if v.IsNil() {
			return "nil", nil
		}
		elem, err := w.value(v.Elem(), path, depth+1)
		if err != nil {
			return "", err
		}
		if v.Elem().Kind() == reflect.Struct && v.Elem().Type() != reflect.TypeFor[time.Time]() {
			return "&" + elem, nil
		}
		name, err := w.typeExpr(t.Elem(), path)
		if err != nil {
			return "", err
		}
		// Go has no address of a number or a string.
		return "func() *" + name + " { v := " + name + "(" + elem + "); return &v }()", nil
	case reflect.Slice, reflect.Array:
		if t.Kind() == reflect.Slice && v.IsNil() {
			return "nil", nil
		}
		name, err := w.typeExpr(t, path)
		if err != nil {
			return "", err
		}
		parts := make([]string, v.Len())
		for i := range parts {
			if parts[i], err = w.value(v.Index(i), path+"["+strconv.Itoa(i)+"]", depth+1); err != nil {
				return "", err
			}
		}
		return name + "{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Map:
		if v.IsNil() {
			return "nil", nil
		}
		name, err := w.typeExpr(t, path)
		if err != nil {
			return "", err
		}
		var parts []string
		iter := v.MapRange()
		for iter.Next() {
			key, err := w.value(iter.Key(), path+"[key]", depth+1)
			if err != nil {
				return "", err
			}
			elem, err := w.value(iter.Value(), path+"["+key+"]", depth+1)
			if err != nil {
				return "", err
			}
			parts = append(parts, key+": "+elem)
		}
		sort.Strings(parts)
		return name + "{" + strings.Join(parts, ", ") + "}", nil
	case reflect.Interface:
		if v.IsNil() {
			return "nil", nil
		}
		return w.dynamic(v.Elem(), path, depth+1)
	case reflect.Func:
		if v.IsNil() {
			return "nil", nil
		}
		return "", cannot(path, "a function")
	case reflect.Chan:
		if v.IsNil() {
			return "nil", nil
		}
		return "", cannot(path, "a channel")
	}
	return "", cannot(path, "a value of the type "+t.String())
}

// dynamic writes the value of an interface: the source must give the type,
// because the place does not.
func (w *writer) dynamic(v reflect.Value, path string, depth int) (string, error) {
	src, err := w.value(v, path, depth)
	if err != nil {
		return "", err
	}
	t := v.Type()
	switch t.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Pointer:
		// The literal names its type.
		return src, nil
	}
	if t.Name() != "" && t.PkgPath() == "" {
		switch t.Kind() {
		case reflect.Bool, reflect.String:
			return src, nil
		case reflect.Int:
			return src, nil
		case reflect.Float64:
			if strings.ContainsAny(src, ".e") {
				return src, nil
			}
		}
	}
	name, err := w.typeExpr(t, path)
	if err != nil {
		return "", err
	}
	return name + "(" + src + ")", nil
}
