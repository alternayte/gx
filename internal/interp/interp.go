package interp

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
)

// File is one compiled source file: its functions by name.
type File struct {
	funcs map[string]*Func
}

// Func returns the function with the name, or nil.
func (f *File) Func(name string) *Func { return f.funcs[name] }

// Names returns the names of the functions of the file.
func (f *File) Names() []string {
	out := make([]string, 0, len(f.funcs))
	for name := range f.funcs {
		out = append(out, name)
	}
	return out
}

// Func is one interpreted function.
type Func struct {
	name string
	typ  reflect.Type
	body *closure
}

// Type returns the type of the function.
func (f *Func) Type() reflect.Type { return f.typ }

// Call runs the function. The arguments must have the parameter types. A
// panic of the code is a panic of Call, as it is in compiled code.
func (f *Func) Call(args []reflect.Value) []reflect.Value {
	return f.body.call(nil, args)
}

// CallAny runs the function with plain values and returns its first
// result. A nil argument is the zero value of its parameter type, so a nil
// interface value passes.
func (f *Func) CallAny(args ...any) any {
	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		v := reflect.New(f.typ.In(i)).Elem()
		if arg != nil {
			v.Set(reflect.ValueOf(arg))
		}
		in[i] = v
	}
	out := f.Call(in)
	if len(out) == 0 {
		return nil
	}
	return out[0].Interface()
}

// Compile parses the Go source of one file of the package with the import
// path pkgPath and prepares its functions. It resolves each name against
// the table and returns an *Unsupported error for code that the interpreter
// does not cover.
func Compile(table *Table, pkgPath, filename string, src []byte) (file *File, err error) {
	fset := token.NewFileSet()
	parsed, perr := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if perr != nil {
		return nil, &Unsupported{Msg: perr.Error()}
	}
	own := table.Package(pkgPath)
	if own == nil {
		return nil, &Unsupported{Msg: "the symbol table has no package " + pkgPath}
	}
	c := &compiler{table: table, fset: fset, own: own, imports: map[string]*Package{}}
	defer func() {
		if r := recover(); r != nil {
			u, ok := r.(*Unsupported)
			if !ok {
				panic(r)
			}
			file, err = nil, u
		}
	}()
	for _, im := range parsed.Imports {
		path, _ := strconv.Unquote(im.Path.Value)
		pkg := table.Package(path)
		if pkg == nil {
			c.fail(im, "the symbol table has no package %s", path)
		}
		name := pkg.Name
		if im.Name != nil {
			name = im.Name.Name
		}
		c.imports[name] = pkg
	}
	file = &File{funcs: map[string]*Func{}}
	for _, decl := range parsed.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Body == nil {
			// The types of the file are part of the compiled app; a change
			// of one takes the rebuild path before the interpreter runs.
			continue
		}
		if fd.Type.TypeParams != nil {
			c.fail(fd, "a generic function is not interpreted")
		}
		// The compiled function gives the exact type, with its named
		// parameter types.
		compiled, ok := own.Values[fd.Name.Name]
		if !ok || compiled.Kind() != reflect.Func {
			c.fail(fd, "the symbol table has no function %s", fd.Name.Name)
		}
		fn := &Func{name: fd.Name.Name, typ: compiled.Type()}
		fn.body = c.funcBody(nil, fd.Type, fd.Body, compiled.Type())
		file.funcs[fn.name] = fn
	}
	return file, nil
}

// compiler holds the state of one Compile call.
type compiler struct {
	table   *Table
	fset    *token.FileSet
	own     *Package
	imports map[string]*Package
	fn      *funcScope
}

// funcScope is the compile-time view of one function or function literal:
// its variables by block, and the count of slots of its frame.
type funcScope struct {
	parent *funcScope
	blocks []map[string]variable
	slots  int
	out    []reflect.Type
}

type variable struct {
	slot int
	typ  reflect.Type
}

// frame is the run-time state of one call: one addressable value for each
// slot, and the frame of the function that made the closure.
type frame struct {
	vars  []reflect.Value
	outer *frame
}

// closure is a compiled function body.
type closure struct {
	slots  int
	params []int // the slots of the parameters, -1 for _
	in     []reflect.Type
	out    []reflect.Type
	body   stmt
	// variadic is true when the last parameter takes the rest of the
	// arguments as a slice; reflect hands that slice over.
}

func (cl *closure) call(outer *frame, args []reflect.Value) []reflect.Value {
	fr := &frame{vars: make([]reflect.Value, cl.slots), outer: outer}
	for i, slot := range cl.params {
		if slot < 0 {
			continue
		}
		v := reflect.New(cl.in[i]).Elem()
		v.Set(args[i])
		fr.vars[slot] = v
	}
	var res result
	cl.body(fr, &res)
	if len(cl.out) == 0 {
		return nil
	}
	out := make([]reflect.Value, len(cl.out))
	for i, t := range cl.out {
		v := reflect.New(t).Elem()
		if i < len(res.values) && res.values[i].IsValid() {
			v.Set(res.values[i])
		}
		out[i] = v
	}
	return out
}

func (c *compiler) fail(node ast.Node, format string, args ...any) {
	u := &Unsupported{Msg: fmt.Sprintf(format, args...)}
	if node != nil {
		u.Pos = c.fset.Position(node.Pos())
	}
	panic(u)
}

// funcBody compiles the body of a function declaration or literal. typ is
// the type of the function.
func (c *compiler) funcBody(parent *funcScope, ft *ast.FuncType, body *ast.BlockStmt, typ reflect.Type) *closure {
	scope := &funcScope{parent: parent, blocks: []map[string]variable{{}}}
	for i := 0; i < typ.NumOut(); i++ {
		scope.out = append(scope.out, typ.Out(i))
	}
	saved := c.fn
	c.fn = scope
	defer func() { c.fn = saved }()
	cl := &closure{out: scope.out}
	i := 0
	for _, field := range ft.Params.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{{Name: "_"}}
		}
		for _, name := range names {
			if i >= typ.NumIn() {
				c.fail(field, "the function has more parameters than its compiled type")
			}
			cl.in = append(cl.in, typ.In(i))
			if name.Name == "_" {
				cl.params = append(cl.params, -1)
			} else {
				cl.params = append(cl.params, c.declare(name.Name, typ.In(i)))
			}
			i++
		}
	}
	if i != typ.NumIn() {
		c.fail(ft, "the function has %d parameters and its compiled type has %d", i, typ.NumIn())
	}
	if ft.Results != nil {
		for _, field := range ft.Results.List {
			if len(field.Names) > 0 {
				c.fail(field, "a named result is not interpreted")
			}
		}
	}
	cl.body = c.block(body.List, false)
	cl.slots = scope.slots
	return cl
}

// declare adds a variable to the current block and returns its slot.
func (c *compiler) declare(name string, typ reflect.Type) int {
	slot := c.fn.slots
	c.fn.slots++
	if name != "_" {
		c.fn.blocks[len(c.fn.blocks)-1][name] = variable{slot: slot, typ: typ}
	}
	return slot
}

func (c *compiler) push() { c.fn.blocks = append(c.fn.blocks, map[string]variable{}) }
func (c *compiler) pop()  { c.fn.blocks = c.fn.blocks[:len(c.fn.blocks)-1] }

// lookup finds a variable: its slot, its type and how many function levels
// up its frame is.
func (c *compiler) lookup(name string) (v variable, depth int, ok bool) {
	for fn := c.fn; fn != nil; fn = fn.parent {
		for i := len(fn.blocks) - 1; i >= 0; i-- {
			if v, ok := fn.blocks[i][name]; ok {
				return v, depth, true
			}
		}
		depth++
	}
	return variable{}, 0, false
}

// value is one compiled expression. Exactly one form holds: a constant
// (cst, with typ nil while it is untyped), the untyped nil, a type, a
// list of results of a call (multi), or a run-time value (eval).
type value struct {
	typ reflect.Type
	cst constant.Value
	// isRune marks an untyped rune constant: its default type is rune.
	isRune bool
	isNil  bool
	// isType is set when the expression names a type.
	isType reflect.Type
	eval   func(fr *frame) reflect.Value
	// addr returns the variable itself, for an assignment, for & and for
	// a method with a pointer receiver. It is nil for a value with no
	// address.
	addr func(fr *frame) reflect.Value
	// multi and evalN hold a call with more or fewer than one result.
	multi []reflect.Type
	evalN func(fr *frame) []reflect.Value
	// method is set for a table method that waits for its call: the
	// function and its receiver.
	bound *boundMethod
}

type boundMethod struct {
	fn   reflect.Value
	recv func(fr *frame) reflect.Value
}

var (
	typeBool    = reflect.TypeOf(false)
	typeInt     = reflect.TypeOf(0)
	typeFloat64 = reflect.TypeOf(0.0)
	typeString  = reflect.TypeOf("")
	typeRune    = reflect.TypeOf(rune(0))
	typeAny     = reflect.TypeOf((*any)(nil)).Elem()
	typeError   = reflect.TypeOf((*error)(nil)).Elem()
)

var basicTypes = map[string]reflect.Type{
	"bool": typeBool, "string": typeString, "int": typeInt, "int8": reflect.TypeOf(int8(0)),
	"int16": reflect.TypeOf(int16(0)), "int32": reflect.TypeOf(int32(0)), "int64": reflect.TypeOf(int64(0)),
	"uint": reflect.TypeOf(uint(0)), "uint8": reflect.TypeOf(uint8(0)), "uint16": reflect.TypeOf(uint16(0)),
	"uint32": reflect.TypeOf(uint32(0)), "uint64": reflect.TypeOf(uint64(0)), "uintptr": reflect.TypeOf(uintptr(0)),
	"float32": reflect.TypeOf(float32(0)), "float64": typeFloat64, "byte": reflect.TypeOf(byte(0)),
	"rune": typeRune, "any": typeAny, "error": typeError,
}

// defaultType is the type that an untyped constant takes with no context.
func defaultType(cst constant.Value) reflect.Type {
	switch cst.Kind() {
	case constant.Bool:
		return typeBool
	case constant.String:
		return typeString
	case constant.Int:
		return typeInt
	case constant.Float:
		return typeFloat64
	}
	return nil
}

// constValue makes a run-time value of a type from a constant.
func (c *compiler) constValue(node ast.Node, cst constant.Value, typ reflect.Type) reflect.Value {
	v := reflect.New(typ).Elem()
	switch typ.Kind() {
	case reflect.Bool:
		if cst.Kind() == constant.Bool {
			v.SetBool(constant.BoolVal(cst))
			return v
		}
	case reflect.String:
		if cst.Kind() == constant.String {
			v.SetString(constant.StringVal(cst))
			return v
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if i, ok := constant.Int64Val(constant.ToInt(cst)); ok && !v.OverflowInt(i) {
			v.SetInt(i)
			return v
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u, ok := constant.Uint64Val(constant.ToInt(cst)); ok && !v.OverflowUint(u) {
			v.SetUint(u)
			return v
		}
	case reflect.Float32, reflect.Float64:
		if cst.Kind() == constant.Int || cst.Kind() == constant.Float {
			f, _ := constant.Float64Val(constant.ToFloat(cst))
			v.SetFloat(f)
			return v
		}
	case reflect.Interface:
		if dt := defaultType(cst); dt != nil && dt.AssignableTo(typ) {
			v.Set(c.constValue(node, cst, dt))
			return v
		}
	}
	c.fail(node, "the constant %s is not a value of type %s", cst.String(), typ)
	return v
}

// typed makes a value with a run-time form: a constant takes typ, or its
// default type when typ is nil.
func (c *compiler) typed(node ast.Node, v value, typ reflect.Type) value {
	switch {
	case v.isType != nil:
		c.fail(node, "a type is not a value")
	case v.multi != nil:
		c.fail(node, "a call with %d results is not one value", len(v.multi))
	case v.bound != nil:
		c.fail(node, "an unexported method is only called, not used as a value")
	case v.isNil:
		if typ == nil {
			c.fail(node, "nil needs a type")
		}
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface, reflect.Chan:
		default:
			c.fail(node, "nil is not a value of type %s", typ)
		}
		zero := reflect.Zero(typ)
		return value{typ: typ, eval: func(*frame) reflect.Value { return zero }}
	case v.cst != nil:
		target := typ
		if v.typ != nil {
			target = v.typ
			if typ != nil && typ.Kind() != reflect.Interface && typ != v.typ {
				c.fail(node, "a constant of type %s is not a value of type %s", v.typ, typ)
			}
		}
		if target == nil || (target.Kind() == reflect.Interface && v.isRune) {
			if v.isRune {
				rv := c.constValue(node, v.cst, typeRune)
				return c.typed(node, value{typ: typeRune, eval: func(*frame) reflect.Value { return rv }}, typ)
			}
			target = defaultType(v.cst)
		}
		if target == nil {
			c.fail(node, "the constant has no type")
		}
		rv := c.constValue(node, v.cst, target)
		v = value{typ: target, eval: func(*frame) reflect.Value { return rv }}
	}
	if typ == nil || v.typ == typ {
		return v
	}
	return c.convertAssign(node, v, typ)
}

// convertAssign makes a typed value assignable to typ: the value of an
// assignment, an argument or a field.
func (c *compiler) convertAssign(node ast.Node, v value, typ reflect.Type) value {
	if !v.typ.AssignableTo(typ) {
		c.fail(node, "a value of type %s is not assignable to %s", v.typ, typ)
	}
	eval := v.eval
	if typ.Kind() == reflect.Interface {
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			if in := eval(fr); in.IsValid() {
				out.Set(in)
			}
			return out
		}}
	}
	return value{typ: typ, eval: func(fr *frame) reflect.Value { return eval(fr).Convert(typ) }}
}

// typeOf resolves a type expression.
func (c *compiler) typeOf(expr ast.Expr) reflect.Type {
	switch t := expr.(type) {
	case *ast.Ident:
		if _, _, shadowed := c.lookup(t.Name); !shadowed {
			if typ, ok := c.own.Types[t.Name]; ok {
				return typ
			}
			if typ, ok := basicTypes[t.Name]; ok {
				return typ
			}
		}
		c.fail(expr, "the symbol table has no type %s", t.Name)
	case *ast.SelectorExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			if pkg, ok := c.imports[id.Name]; ok {
				if typ, ok := pkg.Types[t.Sel.Name]; ok {
					return typ
				}
				c.fail(expr, "the symbol table has no type %s.%s", id.Name, t.Sel.Name)
			}
		}
	case *ast.StarExpr:
		return reflect.PointerTo(c.typeOf(t.X))
	case *ast.ParenExpr:
		return c.typeOf(t.X)
	case *ast.ArrayType:
		elem := c.typeOf(t.Elt)
		if t.Len == nil {
			return reflect.SliceOf(elem)
		}
		n := c.expr(t.Len, nil)
		if n.cst == nil {
			c.fail(t.Len, "the length of an array is a constant")
		}
		length, _ := constant.Int64Val(constant.ToInt(n.cst))
		return reflect.ArrayOf(int(length), elem)
	case *ast.MapType:
		return reflect.MapOf(c.typeOf(t.Key), c.typeOf(t.Value))
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return typeAny
		}
	case *ast.StructType:
		// A struct type with no name. Reflection makes one with exported
		// fields only.
		var fields []reflect.StructField
		for _, field := range t.Fields.List {
			typ := c.typeOf(field.Type)
			if len(field.Names) == 0 {
				c.fail(field, "an embedded field in a struct type with no name is not interpreted")
			}
			for _, name := range field.Names {
				if !name.IsExported() {
					c.fail(field, "an unexported field in a struct type with no name is not interpreted")
				}
				sf := reflect.StructField{Name: name.Name, Type: typ}
				if field.Tag != nil {
					tag, _ := strconv.Unquote(field.Tag.Value)
					sf.Tag = reflect.StructTag(tag)
				}
				fields = append(fields, sf)
			}
		}
		return reflect.StructOf(fields)
	case *ast.FuncType:
		if t.TypeParams != nil {
			break
		}
		var in, out []reflect.Type
		variadic := false
		for _, field := range t.Params.List {
			var typ reflect.Type
			if ell, ok := field.Type.(*ast.Ellipsis); ok {
				variadic = true
				typ = reflect.SliceOf(c.typeOf(ell.Elt))
			} else {
				typ = c.typeOf(field.Type)
			}
			for n := max(len(field.Names), 1); n > 0; n-- {
				in = append(in, typ)
			}
		}
		if t.Results != nil {
			for _, field := range t.Results.List {
				typ := c.typeOf(field.Type)
				for n := max(len(field.Names), 1); n > 0; n-- {
					out = append(out, typ)
				}
			}
		}
		return reflect.FuncOf(in, out, variadic)
	}
	c.fail(expr, "the type %s is not interpreted", exprString(expr))
	return nil
}

// exprString names an expression for a message.
func exprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.IndexExpr:
		return exprString(t.X) + "[...]"
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	}
	return fmt.Sprintf("%T", expr)
}

// run evaluates a value that has a run-time form.
func run(v value, fr *frame) reflect.Value { return v.eval(fr) }

// fromFrame returns the frame that is depth function levels up.
func fromFrame(fr *frame, depth int) *frame {
	for ; depth > 0; depth-- {
		fr = fr.outer
	}
	return fr
}

// expr compiles one expression. hint is the type that the context wants,
// or nil; a composite literal with no type and a function literal use it.
func (c *compiler) expr(e ast.Expr, hint reflect.Type) value {
	switch t := e.(type) {
	case *ast.ParenExpr:
		return c.expr(t.X, hint)
	case *ast.BasicLit:
		cst := constant.MakeFromLiteral(t.Value, t.Kind, 0)
		if cst.Kind() == constant.Unknown {
			c.fail(e, "the literal %s is not interpreted", t.Value)
		}
		// A rune literal has the default type rune.
		return value{cst: cst, isRune: t.Kind == token.CHAR}
	case *ast.Ident:
		return c.ident(t)
	case *ast.SelectorExpr:
		return c.selector(t)
	case *ast.CallExpr:
		return c.call(t, hint)
	case *ast.CompositeLit:
		return c.composite(t, hint)
	case *ast.FuncLit:
		return c.funcLit(t, hint)
	case *ast.UnaryExpr:
		return c.unary(t, hint)
	case *ast.BinaryExpr:
		return c.binary(t, hint)
	case *ast.IndexExpr:
		return c.index(t)
	case *ast.SliceExpr:
		return c.slice(t)
	case *ast.StarExpr:
		x := c.typed(t.X, c.expr(t.X, nil), nil)
		if x.typ.Kind() != reflect.Pointer {
			c.fail(e, "* needs a pointer, not %s", x.typ)
		}
		eval := x.eval
		deref := func(fr *frame) reflect.Value { return eval(fr).Elem() }
		return value{typ: x.typ.Elem(), eval: deref, addr: deref}
	case *ast.TypeAssertExpr:
		if t.Type == nil {
			c.fail(e, "a type switch is not interpreted")
		}
		x := c.typed(t.X, c.expr(t.X, nil), nil)
		if x.typ.Kind() != reflect.Interface {
			c.fail(e, "a type assertion needs an interface value")
		}
		typ := c.typeOf(t.Type)
		eval := x.eval
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out, ok := assertType(eval(fr), typ)
			if !ok {
				panic(fmt.Sprintf("interface conversion: the value is not %s", typ))
			}
			return out
		}}
	case *ast.ArrayType, *ast.MapType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
		return value{isType: c.typeOf(e)}
	}
	c.fail(e, "the expression %T is not interpreted", e)
	return value{}
}

// assertType is x.(typ) on a value of an interface type.
func assertType(x reflect.Value, typ reflect.Type) (reflect.Value, bool) {
	if x.Kind() == reflect.Interface {
		if x.IsNil() {
			return reflect.Zero(typ), false
		}
		x = x.Elem()
	}
	if typ.Kind() == reflect.Interface {
		if !x.Type().Implements(typ) {
			return reflect.Zero(typ), false
		}
		out := reflect.New(typ).Elem()
		out.Set(x)
		return out, true
	}
	if x.Type() != typ {
		return reflect.Zero(typ), false
	}
	return x, true
}

func (c *compiler) ident(id *ast.Ident) value {
	if v, depth, ok := c.lookup(id.Name); ok {
		slot := v.slot
		get := func(fr *frame) reflect.Value { return fromFrame(fr, depth).vars[slot] }
		return value{typ: v.typ, eval: get, addr: get}
	}
	if v, ok := c.own.Values[id.Name]; ok {
		return symbolValue(v)
	}
	if cst, ok := c.own.Consts[id.Name]; ok {
		return value{cst: cst}
	}
	if typ, ok := c.own.Types[id.Name]; ok {
		return value{isType: typ}
	}
	switch id.Name {
	case "true":
		return value{cst: constant.MakeBool(true)}
	case "false":
		return value{cst: constant.MakeBool(false)}
	case "nil":
		return value{isNil: true}
	}
	if typ, ok := basicTypes[id.Name]; ok {
		return value{isType: typ}
	}
	c.fail(id, "the symbol table has no name %s", id.Name)
	return value{}
}

// symbolValue is a function, a variable or a typed constant of the table.
// A variable is addressable, so each read gives its current value.
func symbolValue(v reflect.Value) value {
	get := func(*frame) reflect.Value { return v }
	out := value{typ: v.Type(), eval: get}
	if v.CanAddr() {
		out.addr = get
	}
	return out
}

func (c *compiler) selector(sel *ast.SelectorExpr) value {
	if id, ok := sel.X.(*ast.Ident); ok {
		if _, _, local := c.lookup(id.Name); !local {
			if pkg, ok := c.imports[id.Name]; ok {
				name := sel.Sel.Name
				if v, ok := pkg.Values[name]; ok {
					return symbolValue(v)
				}
				if cst, ok := pkg.Consts[name]; ok {
					return value{cst: cst}
				}
				if typ, ok := pkg.Types[name]; ok {
					return value{isType: typ}
				}
				c.fail(sel, "the symbol table has no name %s.%s", id.Name, name)
			}
		}
	}
	x := c.expr(sel.X, nil)
	if x.isType != nil {
		c.fail(sel, "a method expression is not interpreted")
	}
	x = c.typed(sel.X, x, nil)
	return c.member(sel, x, sel.Sel.Name)
}

// member compiles x.name: a field or a method.
func (c *compiler) member(node ast.Node, x value, name string) value {
	typ := x.typ
	base := typ
	if base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	// A field, also through embedded structs.
	if base.Kind() == reflect.Struct {
		if field, ok := base.FieldByName(name); ok {
			index := field.Index
			eval := x.eval
			pointer := typ.Kind() == reflect.Pointer
			get := func(fr *frame) reflect.Value {
				v := eval(fr)
				if pointer {
					v = v.Elem()
				}
				return fieldByIndex(v, index)
			}
			out := value{typ: field.Type, eval: get}
			if pointer {
				out.addr = get
			} else if x.addr != nil {
				addr := x.addr
				out.addr = func(fr *frame) reflect.Value { return fieldByIndex(addr(fr), index) }
			}
			return out
		}
	}
	// An exported method of the value, or of its address.
	if ast.IsExported(name) || typ.Kind() == reflect.Interface {
		if m, ok := typ.MethodByName(name); ok {
			eval := x.eval
			var mtyp reflect.Type
			if typ.Kind() == reflect.Interface {
				mtyp = m.Type
			} else {
				mtyp = methodType(m.Type)
			}
			return value{typ: mtyp, eval: func(fr *frame) reflect.Value { return eval(fr).MethodByName(name) }}
		}
		if typ.Kind() != reflect.Pointer && typ.Kind() != reflect.Interface {
			if m, ok := reflect.PointerTo(typ).MethodByName(name); ok {
				addr := c.addressOf(node, x)
				return value{typ: methodType(m.Type), eval: func(fr *frame) reflect.Value { return addr(fr).MethodByName(name) }}
			}
		}
	}
	// An unexported method: reflection cannot call it, so the table holds
	// a function for it.
	if fn, pointer, ok := c.table.method(typ, name); ok {
		recv := x.eval
		switch {
		case pointer && typ.Kind() != reflect.Pointer:
			recv = c.addressOf(node, x)
		case !pointer && typ.Kind() == reflect.Pointer:
			eval := x.eval
			recv = func(fr *frame) reflect.Value { return eval(fr).Elem() }
		}
		return value{bound: &boundMethod{fn: fn, recv: recv}}
	}
	c.fail(node, "the type %s has no field or method %s in the symbol table", typ, name)
	return value{}
}

// methodType drops the receiver from the type of a method of a type.
func methodType(t reflect.Type) reflect.Type {
	in := make([]reflect.Type, 0, t.NumIn()-1)
	for i := 1; i < t.NumIn(); i++ {
		in = append(in, t.In(i))
	}
	out := make([]reflect.Type, 0, t.NumOut())
	for i := 0; i < t.NumOut(); i++ {
		out = append(out, t.Out(i))
	}
	return reflect.FuncOf(in, out, t.IsVariadic())
}

// addressOf returns the address of a value that has one: a variable, a
// field of one, an element of a slice or the target of a pointer.
func (c *compiler) addressOf(node ast.Node, x value) func(fr *frame) reflect.Value {
	if x.addr != nil {
		addr := x.addr
		return func(fr *frame) reflect.Value { return addr(fr).Addr() }
	}
	c.fail(node, "the value has no address")
	return nil
}
