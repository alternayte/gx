package interp

import (
	"go/ast"
	"go/constant"
	"go/token"
	"reflect"
	"strings"
	"unsafe"
)

// fieldByIndex reads a struct field by its index path. Reflection marks an
// unexported field as read-only and refuses to pass it on; the generated
// code of the same package reads such a field, so the interpreter takes the
// field through its address.
func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		for v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		f := v.Field(i)
		if !f.CanInterface() {
			if !v.CanAddr() {
				addressable := reflect.New(v.Type()).Elem()
				addressable.Set(v)
				f = addressable.Field(i)
			}
			f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
		}
		v = f
	}
	return v
}

// args compiles the arguments of a call for the parameter types of ft.
func (c *compiler) args(call *ast.CallExpr, ft reflect.Type, skip int) (evals []func(*frame) reflect.Value, spread bool) {
	n := ft.NumIn() - skip
	variadic := ft.IsVariadic()
	spread = call.Ellipsis.IsValid()
	if spread && !variadic {
		c.fail(call, "... needs a variadic function")
	}
	if len(call.Args) == 1 && n != 1 {
		// f(g()) with a g that has many results.
		if inner, ok := call.Args[0].(*ast.CallExpr); ok {
			v := c.expr(inner, nil)
			if v.multi != nil {
				if len(v.multi) != n || variadic {
					c.fail(call, "the call has %d results for %d parameters", len(v.multi), n)
				}
				evalN := v.evalN
				for i := range v.multi {
					if !v.multi[i].AssignableTo(ft.In(skip + i)) {
						c.fail(call, "result %d of type %s is not assignable to %s", i, v.multi[i], ft.In(skip+i))
					}
				}
				var cached []reflect.Value
				for i := range v.multi {
					i := i
					evals = append(evals, func(fr *frame) reflect.Value {
						if i == 0 {
							cached = evalN(fr)
						}
						return cached[i]
					})
				}
				return evals, false
			}
		}
	}
	if (!variadic && len(call.Args) != n) || (variadic && !spread && len(call.Args) < n-1) || (spread && len(call.Args) != n) {
		c.fail(call, "the call has %d arguments for %d parameters", len(call.Args), n)
	}
	for i, arg := range call.Args {
		var typ reflect.Type
		switch {
		case variadic && i >= n-1 && !spread:
			typ = ft.In(ft.NumIn() - 1).Elem()
		default:
			typ = ft.In(skip + i)
		}
		v := c.typed(arg, c.expr(arg, typ), typ)
		evals = append(evals, v.eval)
	}
	return evals, spread
}

// results wraps a function call as a value with one result or many.
func results(ft reflect.Type, do func(fr *frame) []reflect.Value) value {
	if ft.NumOut() == 1 {
		return value{typ: ft.Out(0), eval: func(fr *frame) reflect.Value { return do(fr)[0] }}
	}
	multi := make([]reflect.Type, ft.NumOut())
	for i := range multi {
		multi[i] = ft.Out(i)
	}
	return value{multi: multi, evalN: do}
}

func (c *compiler) call(call *ast.CallExpr, hint reflect.Type) value {
	if id, ok := call.Fun.(*ast.Ident); ok {
		if _, _, local := c.lookup(id.Name); !local {
			if _, own := c.own.Values[id.Name]; !own {
				if v, ok := c.builtin(id.Name, call, hint); ok {
					return v
				}
			}
		}
	}
	if _, generic := call.Fun.(*ast.IndexListExpr); generic {
		c.fail(call, "a call of a generic function is not interpreted")
	}
	// gx.Ref[T](path) is a generic function, so the table has no value for
	// it. It only converts a string to gx.SignalRef[T], and the place that
	// takes the value gives that type: the generated code of a signal
	// passed to a child or to an island uses it.
	if ix, ok := call.Fun.(*ast.IndexExpr); ok && hint != nil && hint.Kind() == reflect.String && len(call.Args) == 1 {
		if sel, ok := ix.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "Ref" {
			if id, ok := sel.X.(*ast.Ident); ok {
				if pkg, ok := c.imports[id.Name]; ok && pkg.Path == "github.com/alternayte/gx" && hint.PkgPath() == pkg.Path && strings.HasPrefix(hint.Name(), "SignalRef[") {
					return c.conversion(call, hint)
				}
			}
		}
	}
	fn := c.expr(call.Fun, nil)
	if fn.isType != nil {
		return c.conversion(call, fn.isType)
	}
	if fn.bound != nil {
		// A table function for an unexported method: the receiver is the
		// first argument.
		ft := fn.bound.fn.Type()
		evals, spread := c.args(call, ft, 1)
		recv, f := fn.bound.recv, fn.bound.fn
		return results(ft, func(fr *frame) []reflect.Value {
			in := make([]reflect.Value, 0, len(evals)+1)
			in = append(in, recv(fr))
			for _, eval := range evals {
				in = append(in, eval(fr))
			}
			if spread {
				return f.CallSlice(in)
			}
			return f.Call(in)
		})
	}
	fn = c.typed(call.Fun, fn, nil)
	if fn.typ.Kind() != reflect.Func {
		c.fail(call, "a value of type %s is not a function", fn.typ)
	}
	ft := fn.typ
	evals, spread := c.args(call, ft, 0)
	eval := fn.eval
	return results(ft, func(fr *frame) []reflect.Value {
		f := eval(fr)
		in := make([]reflect.Value, len(evals))
		for i, e := range evals {
			in[i] = e(fr)
		}
		if spread {
			return f.CallSlice(in)
		}
		return f.Call(in)
	})
}

// conversion compiles T(x).
func (c *compiler) conversion(call *ast.CallExpr, typ reflect.Type) value {
	if len(call.Args) != 1 {
		c.fail(call, "a conversion takes one value")
	}
	x := c.expr(call.Args[0], typ)
	if x.cst != nil && x.typ == nil {
		// A constant takes the type, for example gx.Key("a"); an integer
		// constant to a string is the text of that rune.
		switch {
		case typ.Kind() == reflect.String && x.cst.Kind() == constant.Int:
			x = c.typed(call.Args[0], x, typeInt)
		case typ.Kind() == reflect.Slice || typ.Kind() == reflect.Interface:
			// []byte("text"), []rune("text") and any(1) convert the value
			// of the default type.
			x = c.typed(call.Args[0], x, nil)
		default:
			return value{typ: typ, cst: x.cst}
		}
	}
	if x.isNil {
		return c.typed(call, x, typ)
	}
	x = c.typed(call.Args[0], x, nil)
	if !x.typ.ConvertibleTo(typ) {
		c.fail(call, "a value of type %s is not convertible to %s", x.typ, typ)
	}
	eval := x.eval
	return value{typ: typ, eval: func(fr *frame) reflect.Value { return eval(fr).Convert(typ) }}
}

// builtin compiles a call of a built-in function.
func (c *compiler) builtin(name string, call *ast.CallExpr, hint reflect.Type) (value, bool) {
	arg := func(i int, typ reflect.Type) value {
		if i >= len(call.Args) {
			c.fail(call, "%s has too few arguments", name)
		}
		return c.typed(call.Args[i], c.expr(call.Args[i], typ), typ)
	}
	switch name {
	case "len", "cap":
		x := arg(0, nil)
		switch x.typ.Kind() {
		case reflect.String, reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
		default:
			c.fail(call, "%s needs a string, a slice, an array or a map", name)
		}
		eval := x.eval
		isCap := name == "cap"
		return value{typ: typeInt, eval: func(fr *frame) reflect.Value {
			v := eval(fr)
			if isCap {
				return reflect.ValueOf(v.Cap())
			}
			return reflect.ValueOf(v.Len())
		}}, true
	case "append":
		s := arg(0, hint)
		if s.typ.Kind() != reflect.Slice {
			c.fail(call, "append needs a slice")
		}
		eval := s.eval
		if call.Ellipsis.IsValid() {
			if len(call.Args) != 2 {
				c.fail(call, "append with ... takes two arguments")
			}
			rest := arg(1, nil)
			restEval := rest.eval
			if rest.typ.Kind() == reflect.String && s.typ.Elem().Kind() == reflect.Uint8 {
				return value{typ: s.typ, eval: func(fr *frame) reflect.Value {
					return reflect.AppendSlice(eval(fr), reflect.ValueOf([]byte(restEval(fr).String())))
				}}, true
			}
			if rest.typ.Kind() != reflect.Slice || !rest.typ.Elem().AssignableTo(s.typ.Elem()) {
				c.fail(call, "append cannot add %s to %s", rest.typ, s.typ)
			}
			return value{typ: s.typ, eval: func(fr *frame) reflect.Value { return reflect.AppendSlice(eval(fr), restEval(fr)) }}, true
		}
		var items []func(*frame) reflect.Value
		for i := 1; i < len(call.Args); i++ {
			items = append(items, arg(i, s.typ.Elem()).eval)
		}
		return value{typ: s.typ, eval: func(fr *frame) reflect.Value {
			out := eval(fr)
			for _, item := range items {
				out = reflect.Append(out, item(fr))
			}
			return out
		}}, true
	case "make":
		if len(call.Args) == 0 {
			c.fail(call, "make needs a type")
		}
		typ := c.typeOf(call.Args[0])
		var sizes []func(*frame) reflect.Value
		for i := 1; i < len(call.Args); i++ {
			sizes = append(sizes, arg(i, typeInt).eval)
		}
		size := func(fr *frame, i int) int {
			if i < len(sizes) {
				return int(sizes[i](fr).Int())
			}
			return 0
		}
		switch typ.Kind() {
		case reflect.Slice:
			if len(sizes) == 0 {
				c.fail(call, "make of a slice needs a length")
			}
			return value{typ: typ, eval: func(fr *frame) reflect.Value {
				n := size(fr, 0)
				return reflect.MakeSlice(typ, n, max(n, size(fr, 1)))
			}}, true
		case reflect.Map:
			return value{typ: typ, eval: func(fr *frame) reflect.Value { return reflect.MakeMapWithSize(typ, size(fr, 0)) }}, true
		}
		c.fail(call, "make of %s is not interpreted", typ)
	case "new":
		if len(call.Args) != 1 {
			c.fail(call, "new takes one type")
		}
		typ := c.typeOf(call.Args[0])
		return value{typ: reflect.PointerTo(typ), eval: func(*frame) reflect.Value { return reflect.New(typ) }}, true
	case "min", "max":
		if len(call.Args) == 0 {
			c.fail(call, "%s needs a value", name)
		}
		op := token.LSS
		if name == "max" {
			op = token.GTR
		}
		out := c.expr(call.Args[0], hint)
		for i := 1; i < len(call.Args); i++ {
			next := c.expr(call.Args[i], hint)
			out = c.pick(call, op, out, next)
		}
		return out, true
	case "delete":
		m, key := arg(0, nil), value{}
		if m.typ.Kind() != reflect.Map {
			c.fail(call, "delete needs a map")
		}
		key = arg(1, m.typ.Key())
		mEval, kEval := m.eval, key.eval
		return value{multi: []reflect.Type{}, evalN: func(fr *frame) []reflect.Value {
			mEval(fr).SetMapIndex(kEval(fr), reflect.Value{})
			return nil
		}}, true
	case "panic":
		x := arg(0, typeAny)
		eval := x.eval
		return value{multi: []reflect.Type{}, evalN: func(fr *frame) []reflect.Value {
			panic(eval(fr).Interface())
		}}, true
	}
	return value{}, false
}

// pick compiles min or max of two values.
func (c *compiler) pick(node ast.Node, op token.Token, a, b value) value {
	if a.cst != nil && b.cst != nil && a.typ == nil && b.typ == nil {
		if constant.Compare(b.cst, op, a.cst) {
			return b
		}
		return a
	}
	a, b = c.operands(node, a, b)
	less := c.compare(node, op, a.typ)
	ae, be := a.eval, b.eval
	float := a.typ.Kind() == reflect.Float32 || a.typ.Kind() == reflect.Float64
	return value{typ: a.typ, eval: func(fr *frame) reflect.Value {
		x, y := ae(fr), be(fr)
		if float {
			// min and max give NaN when one value is NaN.
			if f := x.Float(); f != f {
				return x
			}
			if f := y.Float(); f != f {
				return y
			}
		}
		if less(y, x) {
			return y
		}
		return x
	}}
}

// runeResult reports whether an operation on two untyped constants gives an
// untyped rune constant: one operand is a rune and no operand is a float.
// The default type of the result is then rune ('a' + 1).
func runeResult(a, b value, typ reflect.Type) bool {
	return typ == nil && (a.isRune || b.isRune) && a.cst.Kind() == constant.Int && b.cst.Kind() == constant.Int
}

// operands gives two operands one type: a constant takes the type of the
// other operand.
func (c *compiler) operands(node ast.Node, a, b value) (value, value) {
	switch {
	case a.isNil && b.isNil:
		c.fail(node, "nil is compared with a value")
	case a.isNil:
		b = c.typed(node, b, nil)
		a = c.typed(node, a, b.typ)
	case b.isNil:
		a = c.typed(node, a, nil)
		b = c.typed(node, b, a.typ)
	case a.eval == nil && b.eval != nil:
		b = c.typed(node, b, nil)
		a = c.typed(node, a, b.typ)
	case b.eval == nil && a.eval != nil:
		a = c.typed(node, a, nil)
		b = c.typed(node, b, a.typ)
	default:
		a = c.typed(node, a, nil)
		b = c.typed(node, b, nil)
	}
	if a.typ != b.typ {
		// One side can be an interface that the other side is assignable
		// to, for an equality test.
		switch {
		case a.typ.Kind() == reflect.Interface && b.typ.AssignableTo(a.typ):
			b = c.convertAssign(node, b, a.typ)
		case b.typ.Kind() == reflect.Interface && a.typ.AssignableTo(b.typ):
			a = c.convertAssign(node, a, b.typ)
		default:
			c.fail(node, "the operands have the types %s and %s", a.typ, b.typ)
		}
	}
	return a, b
}

// compare returns the function of an ordering operator for a type.
func (c *compiler) compare(node ast.Node, op token.Token, typ reflect.Type) func(a, b reflect.Value) bool {
	var cmp func(a, b reflect.Value) int
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		cmp = func(a, b reflect.Value) int { return sign(a.Int() < b.Int(), a.Int() > b.Int()) }
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		cmp = func(a, b reflect.Value) int { return sign(a.Uint() < b.Uint(), a.Uint() > b.Uint()) }
	case reflect.Float32, reflect.Float64:
		// Every order comparison with NaN is false, so <= is not "not >".
		switch op {
		case token.LSS:
			return func(a, b reflect.Value) bool { return a.Float() < b.Float() }
		case token.LEQ:
			return func(a, b reflect.Value) bool { return a.Float() <= b.Float() }
		case token.GTR:
			return func(a, b reflect.Value) bool { return a.Float() > b.Float() }
		default:
			return func(a, b reflect.Value) bool { return a.Float() >= b.Float() }
		}
	case reflect.String:
		cmp = func(a, b reflect.Value) int { return sign(a.String() < b.String(), a.String() > b.String()) }
	default:
		c.fail(node, "the type %s has no order", typ)
	}
	switch op {
	case token.LSS:
		return func(a, b reflect.Value) bool { return cmp(a, b) < 0 }
	case token.LEQ:
		return func(a, b reflect.Value) bool { return cmp(a, b) <= 0 }
	case token.GTR:
		return func(a, b reflect.Value) bool { return cmp(a, b) > 0 }
	default:
		return func(a, b reflect.Value) bool { return cmp(a, b) >= 0 }
	}
}

func sign(less, greater bool) int {
	switch {
	case less:
		return -1
	case greater:
		return 1
	}
	return 0
}

func boolValue(b bool) reflect.Value { return reflect.ValueOf(b) }

// equal is == on two values of one static type.
func equal(a, b reflect.Value) bool {
	if a.Kind() == reflect.Interface || a.Kind() == reflect.Pointer || a.Kind() == reflect.Map ||
		a.Kind() == reflect.Slice || a.Kind() == reflect.Func || a.Kind() == reflect.Chan {
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && b.IsNil()
		}
	}
	if a.Kind() == reflect.Interface {
		a, b = a.Elem(), b.Elem()
		if a.Type() != b.Type() {
			return false
		}
	}
	if !a.Comparable() {
		panic("runtime error: comparing uncomparable type " + a.Type().String())
	}
	return a.Equal(b)
}

func (c *compiler) binary(e *ast.BinaryExpr, hint reflect.Type) value {
	a, b := c.expr(e.X, nil), c.expr(e.Y, nil)
	// Two constants make a constant.
	if a.cst != nil && b.cst != nil && (a.typ == nil || b.typ == nil || a.typ == b.typ) {
		typ := a.typ
		if typ == nil {
			typ = b.typ
		}
		switch e.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return value{cst: constant.MakeBool(constant.Compare(a.cst, e.Op, b.cst))}
		case token.LAND, token.LOR:
			if a.cst.Kind() != constant.Bool || b.cst.Kind() != constant.Bool {
				c.fail(e, "%s needs two bool values", e.Op)
			}
			return value{cst: constant.BinaryOp(a.cst, e.Op, b.cst)}
		case token.SHL, token.SHR:
			n, _ := constant.Uint64Val(constant.ToInt(b.cst))
			return value{typ: typ, cst: constant.Shift(constant.ToInt(a.cst), e.Op, uint(n)), isRune: a.isRune && typ == nil}
		case token.QUO:
			if constant.Sign(b.cst) == 0 {
				c.fail(e, "division by zero")
			}
			op := e.Op
			// The type of a constant decides the division: float64(1) / 2
			// is 0.5. Two integer constants with no float type divide as
			// integers.
			floatType := typ != nil && (typ.Kind() == reflect.Float32 || typ.Kind() == reflect.Float64)
			if a.cst.Kind() == constant.Int && b.cst.Kind() == constant.Int && !floatType {
				op = token.QUO_ASSIGN
			}
			return value{typ: typ, cst: constant.BinaryOp(a.cst, op, b.cst), isRune: runeResult(a, b, typ)}
		case token.ADD, token.SUB, token.MUL, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
			if e.Op == token.REM && constant.Sign(b.cst) == 0 {
				c.fail(e, "division by zero")
			}
			return value{typ: typ, cst: constant.BinaryOp(a.cst, e.Op, b.cst), isRune: runeResult(a, b, typ)}
		}
	}
	switch e.Op {
	case token.LAND, token.LOR:
		a, b = c.typed(e.X, a, nil), c.typed(e.Y, b, nil)
		if a.typ.Kind() != reflect.Bool || b.typ.Kind() != reflect.Bool {
			c.fail(e, "%s needs two bool values", e.Op)
		}
		ae, be, and, typ := a.eval, b.eval, e.Op == token.LAND, a.typ
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			x := ae(fr)
			if x.Bool() != and {
				return x // false && ..., true || ...
			}
			return be(fr).Convert(typ)
		}}
	case token.SHL, token.SHR:
		a = c.typed(e.X, a, nil)
		b = c.typed(e.Y, b, nil)
		ae, be, typ, left := a.eval, b.eval, a.typ, e.Op == token.SHL
		amount := func(v reflect.Value) uint64 {
			if v.CanUint() {
				return v.Uint()
			}
			if v.Int() < 0 {
				panic("runtime error: negative shift amount")
			}
			return uint64(v.Int())
		}
		switch {
		case a.typ.Kind() >= reflect.Int && a.typ.Kind() <= reflect.Int64:
			return value{typ: typ, eval: func(fr *frame) reflect.Value {
				x, n := ae(fr).Int(), amount(be(fr))
				out := reflect.New(typ).Elem()
				if left {
					out.SetInt(x << n)
				} else {
					out.SetInt(x >> n)
				}
				return out
			}}
		case a.typ.Kind() >= reflect.Uint && a.typ.Kind() <= reflect.Uintptr:
			return value{typ: typ, eval: func(fr *frame) reflect.Value {
				x, n := ae(fr).Uint(), amount(be(fr))
				out := reflect.New(typ).Elem()
				if left {
					out.SetUint(x << n)
				} else {
					out.SetUint(x >> n)
				}
				return out
			}}
		}
		c.fail(e, "a shift needs an integer")
	}
	a, b = c.operands(e, a, b)
	ae, be, typ := a.eval, b.eval, a.typ
	switch e.Op {
	case token.EQL:
		return value{typ: typeBool, eval: func(fr *frame) reflect.Value { return boolValue(equal(ae(fr), be(fr))) }}
	case token.NEQ:
		return value{typ: typeBool, eval: func(fr *frame) reflect.Value { return boolValue(!equal(ae(fr), be(fr))) }}
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		cmp := c.compare(e, e.Op, typ)
		return value{typ: typeBool, eval: func(fr *frame) reflect.Value { return boolValue(cmp(ae(fr), be(fr))) }}
	}
	op := e.Op
	switch k := typ.Kind(); {
	case k == reflect.String:
		if op != token.ADD {
			break
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetString(ae(fr).String() + be(fr).String())
			return out
		}}
	case k >= reflect.Int && k <= reflect.Int64:
		fn := intOp(op)
		if fn == nil {
			break
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetInt(fn(ae(fr).Int(), be(fr).Int()))
			return out
		}}
	case k >= reflect.Uint && k <= reflect.Uintptr:
		fn := uintOp(op)
		if fn == nil {
			break
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetUint(fn(ae(fr).Uint(), be(fr).Uint()))
			return out
		}}
	case k == reflect.Float32 || k == reflect.Float64:
		fn := floatOp(op)
		if fn == nil {
			break
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetFloat(fn(ae(fr).Float(), be(fr).Float()))
			return out
		}}
	}
	c.fail(e, "the operator %s is not defined for %s", e.Op, typ)
	return value{}
}

func intOp(op token.Token) func(a, b int64) int64 {
	switch op {
	case token.ADD:
		return func(a, b int64) int64 { return a + b }
	case token.SUB:
		return func(a, b int64) int64 { return a - b }
	case token.MUL:
		return func(a, b int64) int64 { return a * b }
	case token.QUO:
		return func(a, b int64) int64 { return a / b }
	case token.REM:
		return func(a, b int64) int64 { return a % b }
	case token.AND:
		return func(a, b int64) int64 { return a & b }
	case token.OR:
		return func(a, b int64) int64 { return a | b }
	case token.XOR:
		return func(a, b int64) int64 { return a ^ b }
	case token.AND_NOT:
		return func(a, b int64) int64 { return a &^ b }
	}
	return nil
}

func uintOp(op token.Token) func(a, b uint64) uint64 {
	switch op {
	case token.ADD:
		return func(a, b uint64) uint64 { return a + b }
	case token.SUB:
		return func(a, b uint64) uint64 { return a - b }
	case token.MUL:
		return func(a, b uint64) uint64 { return a * b }
	case token.QUO:
		return func(a, b uint64) uint64 { return a / b }
	case token.REM:
		return func(a, b uint64) uint64 { return a % b }
	case token.AND:
		return func(a, b uint64) uint64 { return a & b }
	case token.OR:
		return func(a, b uint64) uint64 { return a | b }
	case token.XOR:
		return func(a, b uint64) uint64 { return a ^ b }
	case token.AND_NOT:
		return func(a, b uint64) uint64 { return a &^ b }
	}
	return nil
}

func floatOp(op token.Token) func(a, b float64) float64 {
	switch op {
	case token.ADD:
		return func(a, b float64) float64 { return a + b }
	case token.SUB:
		return func(a, b float64) float64 { return a - b }
	case token.MUL:
		return func(a, b float64) float64 { return a * b }
	case token.QUO:
		return func(a, b float64) float64 { return a / b }
	}
	return nil
}

func (c *compiler) unary(e *ast.UnaryExpr, hint reflect.Type) value {
	if e.Op == token.AND {
		// &T{...} makes a new value; &x is the address of a variable.
		if lit, ok := e.X.(*ast.CompositeLit); ok {
			var elemHint reflect.Type
			if hint != nil && hint.Kind() == reflect.Pointer {
				elemHint = hint.Elem()
			}
			x := c.composite(lit, elemHint)
			eval := x.eval
			return value{typ: reflect.PointerTo(x.typ), eval: func(fr *frame) reflect.Value {
				p := reflect.New(x.typ)
				p.Elem().Set(eval(fr))
				return p
			}}
		}
		x := c.typed(e.X, c.expr(e.X, nil), nil)
		addr := c.addressOf(e, x)
		return value{typ: reflect.PointerTo(x.typ), eval: addr}
	}
	x := c.expr(e.X, hint)
	if x.cst != nil {
		switch e.Op {
		case token.NOT:
			if x.cst.Kind() == constant.Bool {
				return value{typ: x.typ, cst: constant.MakeBool(!constant.BoolVal(x.cst))}
			}
		case token.SUB, token.ADD, token.XOR:
			return value{typ: x.typ, cst: constant.UnaryOp(e.Op, x.cst, 0), isRune: x.isRune}
		}
	}
	x = c.typed(e.X, x, nil)
	eval, typ := x.eval, x.typ
	k := typ.Kind()
	switch {
	case e.Op == token.NOT && k == reflect.Bool:
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetBool(!eval(fr).Bool())
			return out
		}}
	case e.Op == token.ADD && (k >= reflect.Int && k <= reflect.Float64):
		return x
	case (e.Op == token.SUB || e.Op == token.XOR) && k >= reflect.Int && k <= reflect.Int64:
		neg := e.Op == token.SUB
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			if neg {
				out.SetInt(-eval(fr).Int())
			} else {
				out.SetInt(^eval(fr).Int())
			}
			return out
		}}
	case (e.Op == token.SUB || e.Op == token.XOR) && k >= reflect.Uint && k <= reflect.Uintptr:
		neg := e.Op == token.SUB
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			if neg {
				out.SetUint(-eval(fr).Uint())
			} else {
				out.SetUint(^eval(fr).Uint())
			}
			return out
		}}
	case e.Op == token.SUB && (k == reflect.Float32 || k == reflect.Float64):
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			out.SetFloat(-eval(fr).Float())
			return out
		}}
	}
	c.fail(e, "the operator %s is not defined for %s", e.Op, typ)
	return value{}
}

func (c *compiler) index(e *ast.IndexExpr) value {
	x := c.expr(e.X, nil)
	if x.isType != nil || (x.eval == nil && x.cst == nil) {
		c.fail(e, "a generic type or function is not interpreted")
	}
	x = c.typed(e.X, x, nil)
	eval := x.eval
	typ := x.typ
	if typ.Kind() == reflect.Pointer && typ.Elem().Kind() == reflect.Array {
		inner := eval
		eval = func(fr *frame) reflect.Value { return inner(fr).Elem() }
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Map:
		key := c.typed(e.Index, c.expr(e.Index, typ.Key()), typ.Key())
		kEval, elem := key.eval, typ.Elem()
		return value{typ: elem, eval: func(fr *frame) reflect.Value {
			v := eval(fr).MapIndex(kEval(fr))
			if !v.IsValid() {
				return reflect.Zero(elem)
			}
			return v
		}}
	case reflect.Slice, reflect.Array, reflect.String:
		i := c.typed(e.Index, c.expr(e.Index, typeInt), nil)
		if k := i.typ.Kind(); k < reflect.Int || k > reflect.Uintptr {
			c.fail(e.Index, "an index is an integer")
		}
		iEval := i.eval
		at := func(fr *frame) int {
			v := iEval(fr)
			if v.CanInt() {
				return int(v.Int())
			}
			return int(v.Uint())
		}
		if typ.Kind() == reflect.String {
			return value{typ: reflect.TypeOf(byte(0)), eval: func(fr *frame) reflect.Value {
				return reflect.ValueOf(eval(fr).String()[at(fr)])
			}}
		}
		get := func(fr *frame) reflect.Value { return eval(fr).Index(at(fr)) }
		out := value{typ: typ.Elem(), eval: get}
		if typ.Kind() == reflect.Slice || x.addr != nil {
			out.addr = get
			if typ.Kind() == reflect.Array {
				addr := x.addr
				out.addr = func(fr *frame) reflect.Value { return addr(fr).Index(at(fr)) }
			}
		}
		return out
	}
	c.fail(e, "a value of type %s has no index", typ)
	return value{}
}

func (c *compiler) slice(e *ast.SliceExpr) value {
	x := c.typed(e.X, c.expr(e.X, nil), nil)
	if e.Slice3 {
		c.fail(e, "a slice with three indexes is not interpreted")
	}
	switch x.typ.Kind() {
	case reflect.Slice, reflect.String:
	default:
		c.fail(e, "a value of type %s is not sliced", x.typ)
	}
	bound := func(expr ast.Expr) func(*frame) int {
		if expr == nil {
			return nil
		}
		v := c.typed(expr, c.expr(expr, typeInt), typeInt)
		eval := v.eval
		return func(fr *frame) int { return int(eval(fr).Int()) }
	}
	lo, hi, eval := bound(e.Low), bound(e.High), x.eval
	return value{typ: x.typ, eval: func(fr *frame) reflect.Value {
		v := eval(fr)
		i, j := 0, v.Len()
		if lo != nil {
			i = lo(fr)
		}
		if hi != nil {
			j = hi(fr)
		}
		return v.Slice(i, j)
	}}
}

// composite compiles a composite literal: a struct, a slice, an array or a
// map. hint gives the type of a literal that writes none.
func (c *compiler) composite(lit *ast.CompositeLit, hint reflect.Type) value {
	typ := hint
	if lit.Type != nil {
		typ = c.typeOf(lit.Type)
	}
	if typ == nil {
		c.fail(lit, "the literal has no type")
	}
	// A literal for a pointer type with no type of its own is &T{...}.
	if lit.Type == nil && typ.Kind() == reflect.Pointer {
		inner := c.composite(lit, typ.Elem())
		eval := inner.eval
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			p := reflect.New(typ.Elem())
			p.Elem().Set(eval(fr))
			return p
		}}
	}
	type part struct {
		index []int
		key   func(*frame) reflect.Value
		at    int
		eval  func(*frame) reflect.Value
	}
	var parts []part
	switch typ.Kind() {
	case reflect.Struct:
		for i, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				name, ok := kv.Key.(*ast.Ident)
				if !ok {
					c.fail(kv, "a struct literal names its fields")
				}
				field, ok := typ.FieldByName(name.Name)
				if !ok {
					c.fail(kv, "the type %s has no field %s", typ, name.Name)
				}
				v := c.typed(kv.Value, c.expr(kv.Value, field.Type), field.Type)
				parts = append(parts, part{index: field.Index, eval: v.eval})
				continue
			}
			if i >= typ.NumField() {
				c.fail(elt, "the literal has too many values")
			}
			field := typ.Field(i)
			v := c.typed(elt, c.expr(elt, field.Type), field.Type)
			parts = append(parts, part{index: field.Index, eval: v.eval})
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.New(typ).Elem()
			for _, p := range parts {
				fieldByIndex(out, p.index).Set(p.eval(fr))
			}
			return out
		}}
	case reflect.Slice, reflect.Array:
		elem := typ.Elem()
		next, length := 0, 0
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				k := c.expr(kv.Key, typeInt)
				if k.cst == nil {
					c.fail(kv, "the index of a literal is a constant")
				}
				n, _ := constant.Int64Val(constant.ToInt(k.cst))
				next = int(n)
				elt = kv.Value
			}
			v := c.typed(elt, c.expr(elt, elem), elem)
			parts = append(parts, part{at: next, eval: v.eval})
			next++
			length = max(length, next)
		}
		isArray := typ.Kind() == reflect.Array
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			var out reflect.Value
			if isArray {
				out = reflect.New(typ).Elem()
			} else {
				out = reflect.MakeSlice(typ, length, length)
			}
			for _, p := range parts {
				out.Index(p.at).Set(p.eval(fr))
			}
			return out
		}}
	case reflect.Map:
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				c.fail(elt, "a map literal has keys")
			}
			k := c.typed(kv.Key, c.expr(kv.Key, typ.Key()), typ.Key())
			v := c.typed(kv.Value, c.expr(kv.Value, typ.Elem()), typ.Elem())
			parts = append(parts, part{key: k.eval, eval: v.eval})
		}
		return value{typ: typ, eval: func(fr *frame) reflect.Value {
			out := reflect.MakeMapWithSize(typ, len(parts))
			for _, p := range parts {
				out.SetMapIndex(p.key(fr), p.eval(fr))
			}
			return out
		}}
	}
	c.fail(lit, "a literal of type %s is not interpreted", typ)
	return value{}
}

// funcLit compiles a function literal. Its type is the written signature;
// a context that wants a named function type, such as gx.Slot[T], converts
// it.
func (c *compiler) funcLit(lit *ast.FuncLit, hint reflect.Type) value {
	typ := c.typeOf(lit.Type)
	body := c.funcBody(c.fn, lit.Type, lit.Body, typ)
	return value{typ: typ, eval: func(fr *frame) reflect.Value {
		// The closure keeps the variables of this moment: each pass of a
		// loop has its own.
		outer := &frame{vars: append([]reflect.Value(nil), fr.vars...), outer: fr.outer}
		return reflect.MakeFunc(typ, func(args []reflect.Value) []reflect.Value {
			return body.call(outer, args)
		})
	}}
}
