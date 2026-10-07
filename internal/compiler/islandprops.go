package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"go/types"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// islandKind is the JSON form of one Go type in the props of an island
// (REQ-ISL-02).
type islandKind uint8

const (
	islandString islandKind = iota
	islandBool
	islandInt
	islandUint
	islandFloat
	islandTime
	islandEnum
	islandList
	islandMap
	islandPointer
	islandObject
	// islandSignal is a gx.SignalRef[T] prop: the island reads and writes
	// the signal through its context (REQ-ISL-04). elem is the shape of T.
	islandSignal
)

// islandShape is one mapped type. The Go encoder and the TypeScript type
// are written from the same shape, so the two cannot differ.
type islandShape struct {
	kind islandKind
	typ  types.Type
	// elem is the element of a list, the value of a map or the target of a
	// pointer.
	elem *islandShape
	// object is set for a struct.
	object *islandObjectType
	// alias is the TypeScript name of a string enum, and values holds its
	// members.
	alias  string
	values []string
	// arrayLen is the length of a Go array, or -1.
	arrayLen int64
}

// islandObjectType is one mapped struct.
type islandObjectType struct {
	typ types.Type
	// name is the TypeScript interface name; "" for a struct type with no
	// name, which is written in place.
	name string
	// fn is the Go function that encodes the struct; "" when the encoder
	// is written in place.
	fn     string
	fields []islandField
}

type islandField struct {
	goName   string
	jsonName string
	// absent says when the encoder leaves the key out: "" for never, "nil"
	// for a nil pointer, "empty" for omitempty and "zero" for omitzero.
	absent string
	// shape is the shape of the value. For a pointer field it is the shape
	// of the target, because a nil pointer is an absent key.
	shape *islandShape
}

// islandTypeError is a type that has no mapping.
type islandTypeError struct {
	path   string
	typ    string
	why    string
	secret bool
}

func (e *islandTypeError) Error() string { return e.path + ": " + e.why }

// islandCode is the generated code of one island.
type islandCode struct {
	goSrc []byte
	tsSrc []byte
}

// islandMapper maps the props struct of one island.
type islandMapper struct {
	island *Island
	pkg    *types.Package
	// objects holds the named structs by their type string, so a type that
	// holds itself maps once.
	objects map[string]*islandObjectType
	order   []*islandObjectType
	// enums holds the string enums by their type string.
	enums     map[string]*islandShape
	enumOrder []*islandShape
	// tsNames holds the TypeScript names in use and the type of each.
	tsNames map[string]string
	// open holds the unnamed or unreachable structs on the current path: a
	// struct that is written in place cannot hold itself.
	open    map[string]bool
	imports map[string]string // import path -> qualifier
}

// goOutputPath and propsOutputPath name the two generated files of an island.
func (i *Island) goOutputPath() string {
	return strings.TrimSuffix(i.File, ".ts") + "_gx.go"
}

func (i *Island) propsOutputPath() string {
	return strings.TrimSuffix(i.File, ".ts") + ".props.ts"
}

// islandID is the name the server and the browser use for an island: the
// import path of its package and its name.
func islandID(pkgPath, name string) string { return pkgPath + "/" + name }

// analyzeIslands maps the props struct of every island and writes its
// generated code (REQ-ISL-02). A type with no mapping is GX6002 at the field
// of the props struct that holds it; a gx.Secret is GX7002 (SI-04).
func (l *loader) analyzeIslands(res *typesResult, dirs []string) []Diagnostic {
	byPath := map[string]*packages.Package{}
	for _, pkg := range res.pkgs {
		if pkg.Types != nil {
			byPath[pkg.PkgPath] = pkg
		}
	}
	var diags []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		if p.Module == nil {
			continue
		}
		pkg := byPath[modulePathOf(p.Module, dir)]
		if pkg == nil {
			continue
		}
		for _, name := range islandNames(p) {
			isl := p.Islands[name]
			if !isl.HasProps {
				continue
			}
			obj, _ := pkg.Types.Scope().Lookup(isl.Name + "Props").(*types.TypeName)
			if obj == nil {
				continue
			}
			st, ok := obj.Type().Underlying().(*types.Struct)
			if !ok {
				continue
			}
			m := &islandMapper{
				island:  isl,
				pkg:     pkg.Types,
				objects: map[string]*islandObjectType{},
				enums:   map[string]*islandShape{},
				tsNames: map[string]string{},
				open:    map[string]bool{},
				imports: map[string]string{},
			}
			root, fieldErrs := m.root(obj.Type(), st)
			for _, fe := range fieldErrs {
				pos := pkg.Fset.Position(fe.field.Pos())
				d := Diagnostic{
					Code: CodeIslandType,
					File: pos.Filename,
					Line: pos.Line,
					Col:  pos.Column,
					Msg:  "island prop " + Quoted(fe.err.path) + " has type " + Quoted(fe.err.typ) + ": " + fe.err.why,
					Fix:  "use a string, a number, a bool, time.Time, a slice, a map with string keys, a struct or a pointer to one of them",
				}
				if fe.err.secret {
					d.Code = CodeSecret
					d.Msg = "island prop " + Quoted(fe.err.path) + " has type gx.Secret; a secret cannot cross to the client"
					d.Fix = ""
				}
				diags = append(diags, d)
			}
			if len(fieldErrs) > 0 {
				continue
			}
			if res.islands == nil {
				res.islands = map[*Island]islandCode{}
			}
			res.islands[isl] = islandCode{
				goSrc: m.goSource(pkg.Types.Name(), islandID(pkg.PkgPath, isl.Name), root),
				tsSrc: m.tsSource(root),
			}
		}
	}
	return diags
}

// islandFieldError is the error of one field of the props struct.
type islandFieldError struct {
	field *types.Var
	err   *islandTypeError
}

// root maps the props struct. It reports one error for each of its fields
// that holds a type with no mapping.
func (m *islandMapper) root(t types.Type, st *types.Struct) (*islandObjectType, []islandFieldError) {
	obj := &islandObjectType{typ: t, name: "Props", fn: m.funcName(0)}
	key := types.TypeString(t, nil)
	m.objects[key] = obj
	m.tsNames["Props"] = key
	// The fixed part of a props file declares these names, so a Go type
	// with one of them gets a different TypeScript name.
	for _, name := range islandReservedTS {
		m.tsNames[name] = "gx:" + name
	}
	var errs []islandFieldError
	seen := map[string]bool{}
	for i := 0; i < st.NumFields(); i++ {
		field, ok, err := m.field(st, i, "", seen)
		if err != nil {
			errs = append(errs, islandFieldError{field: st.Field(i), err: err})
			continue
		}
		if ok {
			obj.fields = append(obj.fields, field)
		}
	}
	return obj, errs
}

func (m *islandMapper) funcName(n int) string {
	return "_gx" + m.island.Name + "JSON" + strconv.Itoa(n)
}

// field maps field i of a struct. ok is false for a field that JSON leaves
// out. seen holds the JSON names of the struct so far.
func (m *islandMapper) field(st *types.Struct, i int, path string, seen map[string]bool) (islandField, bool, *islandTypeError) {
	v := st.Field(i)
	tag, tagged := reflect.StructTag(st.Tag(i)).Lookup("json")
	if tag == "-" {
		return islandField{}, false, nil
	}
	name, opts, _ := strings.Cut(tag, ",")
	fieldPath := path + v.Name()
	typ := types.TypeString(v.Type(), m.display)
	if v.Embedded() && (!tagged || name == "") {
		return islandField{}, false, &islandTypeError{path: fieldPath, typ: typ,
			why: "an embedded field has no mapping; give the field a name"}
	}
	if !v.Exported() {
		return islandField{}, false, nil
	}
	f := islandField{goName: v.Name(), jsonName: name}
	if f.jsonName == "" {
		f.jsonName = v.Name()
	}
	omit := ""
	for _, opt := range strings.Split(opts, ",") {
		switch opt {
		case "":
		case "omitempty", "omitzero":
			if omit == "" || opt == "omitzero" {
				omit = opt
			}
		default:
			return islandField{}, false, &islandTypeError{path: fieldPath, typ: typ,
				why: "the json option " + Quoted(opt) + " has no mapping"}
		}
	}
	if seen[f.jsonName] {
		return islandField{}, false, &islandTypeError{path: fieldPath, typ: typ,
			why: "a second field has the JSON name " + Quoted(f.jsonName)}
	}
	seen[f.jsonName] = true
	shape, err := m.shape(v.Type(), fieldPath)
	if err != nil {
		return islandField{}, false, err
	}
	f.shape = shape
	switch {
	case shape.kind == islandPointer:
		f.absent, f.shape = "nil", shape.elem
	case omit == "omitzero":
		if zeroCheck(shape, "v") == "" {
			return islandField{}, false, &islandTypeError{path: fieldPath, typ: typ,
				why: "omitzero needs a type that Go can compare or that has an IsZero method"}
		}
		f.absent = "zero"
	case omit == "omitempty":
		switch emptyCheck(shape, "v") {
		case "":
			// A struct and a time are never empty.
		case "false":
			return islandField{}, false, nil // an array of length 0 is always empty
		default:
			f.absent = "empty"
		}
	}
	return f, true, nil
}

// display writes a package by its name in a message.
func (m *islandMapper) display(p *types.Package) string {
	if p == m.pkg {
		return ""
	}
	return p.Name()
}

// qualify writes a package by its import qualifier in generated Go, and
// records the import.
func (m *islandMapper) qualify(p *types.Package) string {
	if p == m.pkg {
		return ""
	}
	if q, ok := m.imports[p.Path()]; ok {
		return q
	}
	q := "_gxp" + strconv.Itoa(len(m.imports)+1)
	m.imports[p.Path()] = q
	return q
}

// shape maps one type. path names the value for a message.
func (m *islandMapper) shape(t types.Type, path string) (*islandShape, *islandTypeError) {
	t = types.Unalias(t)
	fail := func(why string) (*islandShape, *islandTypeError) {
		return nil, &islandTypeError{path: path, typ: types.TypeString(t, m.display), why: why}
	}
	named, _ := t.(*types.Named)
	if named != nil {
		obj := named.Obj()
		if obj.Pkg() != nil {
			switch {
			case obj.Pkg().Path() == "time" && obj.Name() == "Time":
				return &islandShape{kind: islandTime, typ: t}, nil
			case obj.Pkg().Path() == gxPkgPath && obj.Name() == "Secret":
				return nil, &islandTypeError{path: path, typ: "gx.Secret", secret: true}
			case obj.Pkg().Path() == gxPkgPath && obj.Name() == "SignalRef" && named.TypeArgs().Len() == 1:
				elem, err := m.shape(named.TypeArgs().At(0), path)
				if err != nil {
					return nil, err
				}
				return &islandShape{kind: islandSignal, typ: t, elem: elem}, nil
			}
		}
		for _, method := range []string{"MarshalJSON", "MarshalText"} {
			if hasMethod(named, method) {
				return fail("a type with a " + method + " method has no mapping, because its JSON form is not known")
			}
		}
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			if named != nil {
				if values := enumValues(named); len(values) > 0 {
					return m.enum(named, values), nil
				}
			}
			return &islandShape{kind: islandString, typ: t}, nil
		case u.Info()&types.IsBoolean != 0:
			return &islandShape{kind: islandBool, typ: t}, nil
		case u.Kind() == types.Uintptr:
			return fail("a uintptr has no mapping")
		case u.Info()&types.IsUnsigned != 0:
			return &islandShape{kind: islandUint, typ: t}, nil
		case u.Info()&types.IsInteger != 0:
			return &islandShape{kind: islandInt, typ: t}, nil
		case u.Info()&types.IsFloat != 0:
			return &islandShape{kind: islandFloat, typ: t}, nil
		}
		return fail("this type has no mapping")
	case *types.Slice:
		if b, ok := u.Elem().Underlying().(*types.Basic); ok && b.Kind() == types.Uint8 {
			return fail("a byte slice has no mapping; use a string")
		}
		elem, err := m.shape(u.Elem(), path+"[]")
		if err != nil {
			return nil, err
		}
		return &islandShape{kind: islandList, typ: t, elem: elem, arrayLen: -1}, nil
	case *types.Array:
		elem, err := m.shape(u.Elem(), path+"[]")
		if err != nil {
			return nil, err
		}
		return &islandShape{kind: islandList, typ: t, elem: elem, arrayLen: u.Len()}, nil
	case *types.Map:
		if b, ok := u.Key().Underlying().(*types.Basic); !ok || b.Info()&types.IsString == 0 {
			return fail("a map needs string keys")
		}
		elem, err := m.shape(u.Elem(), path+"[key]")
		if err != nil {
			return nil, err
		}
		return &islandShape{kind: islandMap, typ: t, elem: elem}, nil
	case *types.Pointer:
		if _, ok := u.Elem().Underlying().(*types.Pointer); ok {
			return fail("a pointer to a pointer has no mapping")
		}
		elem, err := m.shape(u.Elem(), path)
		if err != nil {
			return nil, err
		}
		return &islandShape{kind: islandPointer, typ: t, elem: elem}, nil
	case *types.Struct:
		return m.object(t, named, u, path)
	case *types.Interface:
		return fail("an interface has no mapping, because its JSON form is not known")
	}
	return fail("this type has no mapping")
}

// object maps a struct type.
func (m *islandMapper) object(t types.Type, named *types.Named, st *types.Struct, path string) (*islandShape, *islandTypeError) {
	key := types.TypeString(t, nil)
	if obj, ok := m.objects[key]; ok {
		return &islandShape{kind: islandObject, typ: t, object: obj}, nil
	}
	obj := &islandObjectType{typ: t}
	// A struct gets its own TypeScript interface and its own Go function
	// when the generated code can name its type.
	reachable := named != nil && (named.Obj().Pkg() == m.pkg || named.Obj().Exported()) && named.Obj().Parent() != nil &&
		named.Obj().Parent() == named.Obj().Pkg().Scope()
	if reachable {
		obj.name = m.tsName(tsTypeName(named), named.Obj().Pkg(), key)
		obj.fn = m.funcName(len(m.order) + 1)
		m.objects[key] = obj
		m.order = append(m.order, obj)
	} else {
		if m.open[key] {
			return nil, &islandTypeError{path: path, typ: types.TypeString(t, m.display),
				why: "a struct type that holds itself needs an exported name"}
		}
		m.open[key] = true
		defer delete(m.open, key)
	}
	seen := map[string]bool{}
	for i := 0; i < st.NumFields(); i++ {
		field, ok, err := m.field(st, i, path+".", seen)
		if err != nil {
			return nil, err
		}
		if ok {
			obj.fields = append(obj.fields, field)
		}
	}
	return &islandShape{kind: islandObject, typ: t, object: obj}, nil
}

// enum maps a named string type that has constants.
func (m *islandMapper) enum(named *types.Named, values []string) *islandShape {
	key := types.TypeString(named, nil)
	if s, ok := m.enums[key]; ok {
		return s
	}
	s := &islandShape{kind: islandEnum, typ: named, values: values}
	s.alias = m.tsName(tsTypeName(named), named.Obj().Pkg(), key)
	m.enums[key] = s
	m.enumOrder = append(m.enumOrder, s)
	return s
}

// enumValues returns the values of the constants that the package of a
// named string type declares with that type, in order and with no value
// twice.
func enumValues(named *types.Named) []string {
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok || !types.Identical(c.Type(), named) {
			continue
		}
		s, err := strconv.Unquote(c.Val().ExactString())
		if err != nil || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

var notTSIdent = regexp.MustCompile(`[^A-Za-z0-9_$]+`)

// tsTypeName returns the TypeScript name of a named Go type. The type
// arguments of a generic type are part of the name.
func tsTypeName(named *types.Named) string {
	name := named.Obj().Name()
	if args := named.TypeArgs(); args != nil {
		for i := 0; i < args.Len(); i++ {
			arg := types.TypeString(args.At(i), func(*types.Package) string { return "" })
			name += upperFirst(notTSIdent.ReplaceAllString(arg, ""))
		}
	}
	return name
}

// tsName reserves a TypeScript name for the type key. Two Go types with one
// name get the package name in front, then a number.
func (m *islandMapper) tsName(name string, pkg *types.Package, key string) string {
	candidates := []string{name}
	if pkg != nil {
		candidates = append(candidates, upperFirst(pkg.Name())+upperFirst(name))
	}
	for _, c := range candidates {
		if _, used := m.tsNames[c]; !used {
			m.tsNames[c] = key
			return c
		}
	}
	for n := 2; ; n++ {
		c := name + strconv.Itoa(n)
		if _, used := m.tsNames[c]; !used {
			m.tsNames[c] = key
			return c
		}
	}
}

func hasMethod(named *types.Named, name string) bool {
	for _, t := range []types.Type{named, types.NewPointer(named)} {
		obj, _, _ := types.LookupFieldOrMethod(t, true, nil, name)
		if _, ok := obj.(*types.Func); ok {
			return true
		}
	}
	return false
}

// emptyCheck returns the Go condition that the value expr is not empty, as
// encoding/json reads omitempty. It returns "" for a value that is never
// empty, and "false" for a value that is always empty.
func emptyCheck(s *islandShape, expr string) string {
	switch s.kind {
	case islandString, islandEnum, islandSignal:
		return expr + ` != ""`
	case islandBool:
		return "bool(" + expr + ")"
	case islandInt, islandUint, islandFloat:
		return expr + " != 0"
	case islandList:
		if s.arrayLen == 0 {
			return "false"
		}
		if s.arrayLen > 0 {
			return ""
		}
		return "len(" + expr + ") != 0"
	case islandMap:
		return "len(" + expr + ") != 0"
	case islandPointer:
		return expr + " != nil"
	}
	return ""
}

// zeroCheck returns the Go condition that the value expr is not the zero
// value, as encoding/json reads omitzero. It returns "" for a type with no
// such condition.
func zeroCheck(s *islandShape, expr string) string {
	if named, ok := types.Unalias(s.typ).(*types.Named); ok {
		if obj, _, _ := types.LookupFieldOrMethod(named, false, nil, "IsZero"); obj != nil {
			if fn, ok := obj.(*types.Func); ok {
				sig := fn.Type().(*types.Signature)
				if sig.Params().Len() == 0 && sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) {
					return "!" + expr + ".IsZero()"
				}
			}
		}
	}
	switch s.kind {
	case islandString, islandEnum:
		return expr + ` != ""`
	case islandBool:
		return "bool(" + expr + ")"
	case islandInt, islandUint, islandFloat:
		return expr + " != 0"
	case islandPointer, islandMap:
		return expr + " != nil"
	case islandList:
		if s.arrayLen < 0 {
			return expr + " != nil"
		}
	}
	if types.Comparable(s.typ) {
		return "!gx.IsZero(" + expr + ")"
	}
	return ""
}

// goSource returns the generated Go file of the island: the component
// function and the encoders of its props. The encoders use no reflection
// (SDD 2.2).
func (m *islandMapper) goSource(pkgName, id string, root *islandObjectType) []byte {
	var body bytes.Buffer
	w := &islandWriter{b: &body, m: m}
	name := m.island.Name
	w.line("// %s renders the island of %s (REQ-ISL-01). The browser mounts it", name, filepath.Base(m.island.File))
	w.line("// with the props as JSON. An option sets the time of the load.")
	w.line("func %s(p %sProps, opts ...gx.IslandOption) gx.Node {", name, name)
	w.ind++
	w.line("return gx.Island(%s, string(%s(make([]byte, 0, 256), p)), opts...)", strconv.Quote(id), root.fn)
	w.ind--
	w.line("}")
	for _, obj := range append([]*islandObjectType{root}, m.order...) {
		w.line("")
		w.line("func %s(b []byte, v %s) []byte {", obj.fn, types.TypeString(obj.typ, m.qualify))
		w.ind++
		w.object(obj, "v")
		w.line("return b")
		w.ind--
		w.line("}")
	}

	var out bytes.Buffer
	out.WriteString("// Code generated by gx. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\n", pkgName)
	out.WriteString("import (\n\tgx \"github.com/alternayte/gx\"\n")
	paths := make([]string, 0, len(m.imports))
	for path := range m.imports {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return m.imports[paths[i]] < m.imports[paths[j]] })
	for _, path := range paths {
		fmt.Fprintf(&out, "\t%s %s\n", m.imports[path], strconv.Quote(path))
	}
	out.WriteString(")\n\n")
	out.Write(body.Bytes())
	if formatted, err := format.Source(out.Bytes()); err == nil {
		return formatted
	}
	return out.Bytes()
}

// islandWriter writes the Go encoder.
type islandWriter struct {
	b   *bytes.Buffer
	m   *islandMapper
	ind int
	// n numbers the local variables, so that a nested loop has its own.
	n int
}

func (w *islandWriter) line(format string, args ...any) {
	if format != "" {
		w.b.WriteString(strings.Repeat("\t", w.ind))
		fmt.Fprintf(w.b, format, args...)
	}
	w.b.WriteByte('\n')
}

// text writes the Go statement that appends fixed JSON text.
func (w *islandWriter) text(s string) {
	if len(s) == 1 {
		w.line("b = append(b, %s)", strconv.QuoteRune(rune(s[0])))
		return
	}
	w.line("b = append(b, %s...)", strconv.Quote(s))
}

func jsonKey(name string) string {
	data, _ := json.Marshal(name)
	return string(data) + ":"
}

// object writes the encoder of a struct value.
func (w *islandWriter) object(obj *islandObjectType, expr string) {
	conditional := false
	for _, f := range obj.fields {
		if f.absent != "" {
			conditional = true
		}
	}
	if !conditional {
		if len(obj.fields) == 0 {
			w.text("{}")
			return
		}
		for i, f := range obj.fields {
			sep := ","
			if i == 0 {
				sep = "{"
			}
			w.text(sep + jsonKey(f.jsonName))
			w.value(f.shape, expr+"."+f.goName)
		}
		w.text("}")
		return
	}
	// A key that can be absent decides the separator at run time.
	w.n++
	sep := "sep" + strconv.Itoa(w.n)
	w.line("%s := byte('{')", sep)
	for _, f := range obj.fields {
		access := expr + "." + f.goName
		cond := ""
		switch f.absent {
		case "nil":
			cond = access + " != nil"
			access = "(*" + access + ")"
		case "empty":
			cond = emptyCheck(f.shape, access)
		case "zero":
			cond = zeroCheck(f.shape, access)
		}
		if cond != "" {
			w.line("if %s {", cond)
			w.ind++
		}
		w.line("b = append(b, %s)", sep)
		w.line("%s = ','", sep)
		w.text(jsonKey(f.jsonName))
		w.value(f.shape, access)
		if cond != "" {
			w.ind--
			w.line("}")
		}
	}
	w.line("if %s == '{' {", sep)
	w.ind++
	w.text("{")
	w.ind--
	w.line("}")
	w.text("}")
}

// value writes the encoder of one value.
func (w *islandWriter) value(s *islandShape, expr string) {
	switch s.kind {
	case islandString, islandEnum:
		w.line("b = gx.AppendJSONString(b, string(%s))", expr)
	case islandBool:
		w.line("b = gx.AppendJSONBool(b, bool(%s))", expr)
	case islandInt:
		w.line("b = gx.AppendJSONInt(b, int64(%s))", expr)
	case islandUint:
		w.line("b = gx.AppendJSONUint(b, uint64(%s))", expr)
	case islandFloat:
		w.line("b = gx.AppendJSONFloat(b, float64(%s))", expr)
	case islandTime:
		w.line("b = gx.AppendJSONTime(b, %s)", expr)
	case islandSignal:
		w.line("b = gx.AppendJSONSignalRef(b, string(%s))", expr)
	case islandList:
		w.n++
		i := "i" + strconv.Itoa(w.n)
		w.text("[")
		w.line("for %s := range %s {", i, expr)
		w.ind++
		w.line("if %s > 0 {", i)
		w.ind++
		w.text(",")
		w.ind--
		w.line("}")
		w.value(s.elem, expr+"["+i+"]")
		w.ind--
		w.line("}")
		w.text("]")
	case islandMap:
		w.n++
		i, k := "i"+strconv.Itoa(w.n), "k"+strconv.Itoa(w.n)
		w.text("{")
		w.line("for %s, %s := range gx.SortedKeys(%s) {", i, k, expr)
		w.ind++
		w.line("if %s > 0 {", i)
		w.ind++
		w.text(",")
		w.ind--
		w.line("}")
		w.line("b = gx.AppendJSONString(b, string(%s))", k)
		w.text(":")
		w.value(s.elem, expr+"["+k+"]")
		w.ind--
		w.line("}")
		w.text("}")
	case islandPointer:
		w.line("if %s == nil {", expr)
		w.ind++
		w.text("null")
		w.ind--
		w.line("} else {")
		w.ind++
		w.value(s.elem, "(*"+expr+")")
		w.ind--
		w.line("}")
	case islandObject:
		if s.object.fn != "" {
			w.line("b = %s(b, %s)", s.object.fn, expr)
			return
		}
		w.object(s.object, expr)
	}
}

// tsSource returns the generated <Name>.props.ts file (REQ-ISL-02).
func (m *islandMapper) tsSource(root *islandObjectType) []byte {
	var b strings.Builder
	b.WriteString("// Code generated by gx. DO NOT EDIT.\n")
	for _, e := range m.enumOrder {
		parts := make([]string, len(e.values))
		for i, v := range e.values {
			data, _ := json.Marshal(v)
			parts[i] = string(data)
		}
		fmt.Fprintf(&b, "\nexport type %s = %s;\n", e.alias, strings.Join(parts, " | "))
	}
	for _, obj := range append(append([]*islandObjectType{}, m.order...), root) {
		fmt.Fprintf(&b, "\nexport interface %s {\n", obj.name)
		for _, f := range obj.fields {
			fmt.Fprintf(&b, "  %s;\n", tsField(f))
		}
		b.WriteString("}\n")
	}
	b.WriteString(islandContextTS)
	return []byte(b.String())
}

// islandReservedTS are the names that islandContextTS declares.
var islandReservedTS = []string{"SignalRef", "Signal", "Ctx", "Cleanup", "Mount", "Update"}

// islandContextTS is the part of a props file that is the same for every
// island: the types of the mount function and of its context
// (REQ-ISL-04). The file needs no package, so the type check of an island
// needs no install step.
const islandContextTS = `
/** A gx.SignalRef prop. Give it to ctx.signal to get the signal. */
export interface SignalRef<T> {
  readonly path: readonly string[];
  readonly value?: T;
}

/** A signal of the page. subscribe calls fn now and after each change. */
export interface Signal<T> {
  get(): T;
  set(value: T): void;
  subscribe(fn: (value: T) => void): () => void;
}

/** The context of one mounted island. abort ends when the island leaves the page. */
export interface Ctx {
  signal<T>(ref: SignalRef<T>): Signal<T>;
  readonly abort: AbortSignal;
}

type Cleanup = void | (() => void);

/** The default export of the island file. */
export type Mount = (el: HTMLElement, props: Props, ctx: Ctx) => Cleanup | Promise<Cleanup>;

/** The optional update export: new props for a mounted island. */
export type Update = (props: Props, el: HTMLElement, ctx: Ctx) => void;
`

var tsIdent = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func tsField(f islandField) string {
	name := f.jsonName
	if !tsIdent.MatchString(name) {
		data, _ := json.Marshal(name)
		name = string(data)
	}
	mark := ""
	if f.absent != "" {
		mark = "?"
	}
	return name + mark + ": " + tsType(f.shape)
}

// tsType returns the TypeScript type of a shape.
func tsType(s *islandShape) string {
	switch s.kind {
	case islandString, islandTime:
		return "string"
	case islandBool:
		return "boolean"
	case islandInt, islandUint, islandFloat:
		return "number"
	case islandEnum:
		return s.alias
	case islandSignal:
		return "SignalRef<" + tsType(s.elem) + ">"
	case islandList:
		elem := tsType(s.elem)
		if s.elem.kind == islandPointer {
			elem = "(" + elem + ")"
		}
		return elem + "[]"
	case islandMap:
		return "Record<string, " + tsType(s.elem) + ">"
	case islandPointer:
		return tsType(s.elem) + " | null"
	case islandObject:
		if s.object.name != "" {
			return s.object.name
		}
		if len(s.object.fields) == 0 {
			return "{}"
		}
		parts := make([]string, len(s.object.fields))
		for i, f := range s.object.fields {
			parts[i] = tsField(f)
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	}
	return "never"
}
