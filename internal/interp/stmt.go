package interp

import (
	"go/ast"
	"go/token"
	"reflect"
)

// ctl is how a statement ends.
type ctl uint8

const (
	ctlNext ctl = iota
	ctlBreak
	ctlContinue
	ctlReturn
)

// result holds the values of a return statement.
type result struct {
	values []reflect.Value
}

// stmt is one compiled statement.
type stmt func(fr *frame, res *result) ctl

// block compiles a list of statements. scoped opens a block scope for it.
func (c *compiler) block(list []ast.Stmt, scoped bool) stmt {
	if scoped {
		c.push()
		defer c.pop()
	}
	stmts := make([]stmt, 0, len(list))
	for _, s := range list {
		if compiled := c.stmt(s); compiled != nil {
			stmts = append(stmts, compiled)
		}
	}
	return func(fr *frame, res *result) ctl {
		for _, s := range stmts {
			if how := s(fr, res); how != ctlNext {
				return how
			}
		}
		return ctlNext
	}
}

// newVar makes the variable of a slot with a first value. A closure that
// was made before keeps the variable it saw.
func newVar(fr *frame, slot int, typ reflect.Type, v reflect.Value) {
	nv := reflect.New(typ).Elem()
	if v.IsValid() {
		nv.Set(v)
	}
	fr.vars[slot] = nv
}

func (c *compiler) stmt(s ast.Stmt) stmt {
	switch t := s.(type) {
	case *ast.EmptyStmt:
		return nil
	case *ast.BlockStmt:
		return c.block(t.List, true)
	case *ast.ExprStmt:
		v := c.expr(t.X, nil)
		if v.multi != nil {
			evalN := v.evalN
			return func(fr *frame, _ *result) ctl { evalN(fr); return ctlNext }
		}
		v = c.typed(t.X, v, nil)
		eval := v.eval
		return func(fr *frame, _ *result) ctl { eval(fr); return ctlNext }
	case *ast.DeclStmt:
		return c.decl(t)
	case *ast.AssignStmt:
		return c.assign(t)
	case *ast.IncDecStmt:
		op := token.ADD
		if t.Tok == token.DEC {
			op = token.SUB
		}
		return c.assign(&ast.AssignStmt{Lhs: []ast.Expr{t.X}, Tok: token.ASSIGN, TokPos: t.TokPos,
			Rhs: []ast.Expr{&ast.BinaryExpr{X: t.X, Op: op, OpPos: t.TokPos, Y: &ast.BasicLit{Kind: token.INT, Value: "1", ValuePos: t.TokPos}}}})
	case *ast.IfStmt:
		return c.ifStmt(t)
	case *ast.ForStmt:
		return c.forStmt(t)
	case *ast.RangeStmt:
		return c.rangeStmt(t)
	case *ast.SwitchStmt:
		return c.switchStmt(t)
	case *ast.ReturnStmt:
		return c.returnStmt(t)
	case *ast.BranchStmt:
		if t.Label != nil {
			c.fail(s, "a label is not interpreted")
		}
		switch t.Tok {
		case token.BREAK:
			return func(*frame, *result) ctl { return ctlBreak }
		case token.CONTINUE:
			return func(*frame, *result) ctl { return ctlContinue }
		}
	}
	c.fail(s, "the statement %T is not interpreted", s)
	return nil
}

func (c *compiler) decl(d *ast.DeclStmt) stmt {
	gen, ok := d.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		c.fail(d, "only a var declaration is interpreted in a function")
	}
	var stmts []stmt
	for _, spec := range gen.Specs {
		vs := spec.(*ast.ValueSpec)
		var typ reflect.Type
		if vs.Type != nil {
			typ = c.typeOf(vs.Type)
		}
		if len(vs.Values) != 0 && len(vs.Values) != len(vs.Names) {
			c.fail(vs, "a var declaration has one value for each name")
		}
		for i, name := range vs.Names {
			var init func(*frame) reflect.Value
			vtyp := typ
			if len(vs.Values) > 0 {
				v := c.typed(vs.Values[i], c.expr(vs.Values[i], typ), typ)
				init, vtyp = v.eval, v.typ
			}
			slot := c.declare(name.Name, vtyp)
			t := vtyp
			stmts = append(stmts, func(fr *frame, _ *result) ctl {
				var v reflect.Value
				if init != nil {
					v = init(fr)
				}
				newVar(fr, slot, t, v)
				return ctlNext
			})
		}
	}
	return func(fr *frame, res *result) ctl {
		for _, s := range stmts {
			s(fr, res)
		}
		return ctlNext
	}
}

// target is the left side of an assignment.
type target struct {
	// define declares a new variable in slot; otherwise set stores.
	define bool
	slot   int
	typ    reflect.Type
	set    func(fr *frame, v reflect.Value)
	// ref reads the operands of the left side (an index, a map, a
	// pointer) and returns the store. Go reads them before it writes any
	// value of the statement. nil means that set has no operand to read.
	ref func(fr *frame) func(v reflect.Value)
}

// detach returns a copy of a value that is a variable or a part of one, so
// a later write to the variable does not change the value that the
// statement has read.
func detach(v reflect.Value) reflect.Value {
	if !v.IsValid() || !v.CanAddr() {
		return v
	}
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	return out
}

// lhs compiles the left side of = for a value of a known type.
func (c *compiler) lhs(e ast.Expr) target {
	if id, ok := e.(*ast.Ident); ok && id.Name == "_" {
		return target{set: func(*frame, reflect.Value) {}}
	}
	// A map element has no address; it is set through the map.
	if ix, ok := e.(*ast.IndexExpr); ok {
		m := c.expr(ix.X, nil)
		if m.eval != nil && m.typ != nil && m.typ.Kind() == reflect.Map {
			key := c.typed(ix.Index, c.expr(ix.Index, m.typ.Key()), m.typ.Key())
			mEval, kEval := m.eval, key.eval
			ref := func(fr *frame) func(reflect.Value) {
				mv, kv := mEval(fr), detach(kEval(fr))
				return func(v reflect.Value) { mv.SetMapIndex(kv, v) }
			}
			return target{typ: m.typ.Elem(), ref: ref, set: func(fr *frame, v reflect.Value) { ref(fr)(v) }}
		}
	}
	x := c.expr(e, nil)
	if x.addr == nil {
		c.fail(e, "the left side of = has no address")
	}
	addr := x.addr
	ref := func(fr *frame) func(reflect.Value) {
		at := addr(fr)
		return func(v reflect.Value) { at.Set(v) }
	}
	return target{typ: x.typ, ref: ref, set: func(fr *frame, v reflect.Value) { addr(fr).Set(v) }}
}

func (c *compiler) assign(a *ast.AssignStmt) stmt {
	switch a.Tok {
	case token.DEFINE, token.ASSIGN:
	default:
		// x op= y is x = x op y.
		op, ok := assignOps[a.Tok]
		if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
			c.fail(a, "the assignment %s is not interpreted", a.Tok)
		}
		return c.assign(&ast.AssignStmt{Lhs: a.Lhs, Tok: token.ASSIGN, TokPos: a.TokPos,
			Rhs: []ast.Expr{&ast.BinaryExpr{X: a.Lhs[0], Op: op, OpPos: a.TokPos, Y: a.Rhs[0]}}})
	}
	define := a.Tok == token.DEFINE

	// Compile the right side first: x := f(x) reads the outer x.
	var types []reflect.Type
	var evalAll func(fr *frame) []reflect.Value
	switch {
	case len(a.Rhs) == 1 && len(a.Lhs) == 2:
		types, evalAll = c.twoValues(a, define)
	case len(a.Rhs) == 1 && len(a.Lhs) > 1:
		v := c.expr(a.Rhs[0], nil)
		if len(v.multi) != len(a.Lhs) {
			c.fail(a, "the right side has %d values for %d names", len(v.multi), len(a.Lhs))
		}
		types, evalAll = v.multi, v.evalN
	case len(a.Rhs) == len(a.Lhs):
		evals := make([]func(*frame) reflect.Value, len(a.Rhs))
		for i, rhs := range a.Rhs {
			var hint reflect.Type
			if !define {
				hint = c.lhsType(a.Lhs[i])
			}
			v := c.typed(rhs, c.expr(rhs, hint), hint)
			evals[i] = v.eval
			types = append(types, v.typ)
		}
		evalAll = func(fr *frame) []reflect.Value {
			// Each value is a copy: a, b = b, a reads both names before
			// it writes one.
			out := make([]reflect.Value, len(evals))
			for i, e := range evals {
				out[i] = detach(e(fr))
			}
			return out
		}
	default:
		c.fail(a, "the assignment has %d values for %d names", len(a.Rhs), len(a.Lhs))
	}

	targets := make([]target, len(a.Lhs))
	for i, lhs := range a.Lhs {
		id, isIdent := lhs.(*ast.Ident)
		if define {
			if !isIdent {
				c.fail(lhs, ":= declares names")
			}
			// A name of this block that exists is assigned, as Go does
			// when := has one new name at least.
			if v, ok := c.fn.blocks[len(c.fn.blocks)-1][id.Name]; ok && id.Name != "_" {
				slot := v.slot
				targets[i] = target{typ: v.typ, set: func(fr *frame, val reflect.Value) { fr.vars[slot].Set(val) }}
				continue
			}
			targets[i] = target{define: true, slot: c.declare(id.Name, types[i]), typ: types[i]}
			continue
		}
		targets[i] = c.lhs(lhs)
		if targets[i].typ != nil && !types[i].AssignableTo(targets[i].typ) {
			c.fail(lhs, "a value of type %s is not assignable to %s", types[i], targets[i].typ)
		}
	}
	return func(fr *frame, _ *result) ctl {
		// The order of Go: the operands of the left side, then the right
		// side, then the stores from left to right.
		stores := make([]func(reflect.Value), len(targets))
		for i, t := range targets {
			if !t.define && t.ref != nil {
				stores[i] = t.ref(fr)
			}
		}
		values := evalAll(fr)
		for i, t := range targets {
			if t.define {
				newVar(fr, t.slot, t.typ, values[i])
				continue
			}
			v := values[i]
			if t.typ != nil && v.Type() != t.typ && t.typ.Kind() != reflect.Interface {
				v = v.Convert(t.typ)
			}
			if stores[i] != nil {
				stores[i](v)
				continue
			}
			t.set(fr, v)
		}
		return ctlNext
	}
}

// lhsType returns the type of the left side of =, as the hint for the
// value. It is nil for _ and for a map element.
func (c *compiler) lhsType(e ast.Expr) reflect.Type {
	if id, ok := e.(*ast.Ident); ok {
		if id.Name == "_" {
			return nil
		}
		if v, _, ok := c.lookup(id.Name); ok {
			return v.typ
		}
	}
	v := c.expr(e, nil)
	if v.eval == nil {
		return nil
	}
	return v.typ
}

var assignOps = map[token.Token]token.Token{
	token.ADD_ASSIGN: token.ADD, token.SUB_ASSIGN: token.SUB, token.MUL_ASSIGN: token.MUL,
	token.QUO_ASSIGN: token.QUO, token.REM_ASSIGN: token.REM, token.AND_ASSIGN: token.AND,
	token.OR_ASSIGN: token.OR, token.XOR_ASSIGN: token.XOR, token.SHL_ASSIGN: token.SHL,
	token.SHR_ASSIGN: token.SHR, token.AND_NOT_ASSIGN: token.AND_NOT,
}

// twoValues compiles the right side of v, ok := x: a map index, a type
// assertion, or a call with two results.
func (c *compiler) twoValues(a *ast.AssignStmt, define bool) ([]reflect.Type, func(fr *frame) []reflect.Value) {
	rhs := a.Rhs[0]
	for {
		p, ok := rhs.(*ast.ParenExpr)
		if !ok {
			break
		}
		rhs = p.X
	}
	switch t := rhs.(type) {
	case *ast.IndexExpr:
		m := c.expr(t.X, nil)
		if m.eval != nil && m.typ != nil && m.typ.Kind() == reflect.Map {
			key := c.typed(t.Index, c.expr(t.Index, m.typ.Key()), m.typ.Key())
			mEval, kEval, elem := m.eval, key.eval, m.typ.Elem()
			return []reflect.Type{elem, typeBool}, func(fr *frame) []reflect.Value {
				v := mEval(fr).MapIndex(kEval(fr))
				if !v.IsValid() {
					return []reflect.Value{reflect.Zero(elem), boolValue(false)}
				}
				return []reflect.Value{v, boolValue(true)}
			}
		}
	case *ast.TypeAssertExpr:
		if t.Type == nil {
			c.fail(rhs, "a type switch is not interpreted")
		}
		x := c.typed(t.X, c.expr(t.X, nil), nil)
		if x.typ.Kind() != reflect.Interface {
			c.fail(rhs, "a type assertion needs an interface value")
		}
		typ := c.typeOf(t.Type)
		eval := x.eval
		return []reflect.Type{typ, typeBool}, func(fr *frame) []reflect.Value {
			v, ok := assertType(eval(fr), typ)
			return []reflect.Value{v, boolValue(ok)}
		}
	}
	v := c.expr(rhs, nil)
	if len(v.multi) != 2 {
		c.fail(a, "the right side does not have two values")
	}
	return v.multi, v.evalN
}

// cond compiles a condition.
func (c *compiler) cond(e ast.Expr) func(fr *frame) bool {
	v := c.typed(e, c.expr(e, nil), nil)
	if v.typ.Kind() != reflect.Bool {
		c.fail(e, "a condition is a bool, not %s", v.typ)
	}
	eval := v.eval
	return func(fr *frame) bool { return eval(fr).Bool() }
}

func (c *compiler) ifStmt(s *ast.IfStmt) stmt {
	c.push()
	defer c.pop()
	var init stmt
	if s.Init != nil {
		init = c.stmt(s.Init)
	}
	cond := c.cond(s.Cond)
	then := c.block(s.Body.List, true)
	var otherwise stmt
	if s.Else != nil {
		otherwise = c.stmt(s.Else)
	}
	return func(fr *frame, res *result) ctl {
		if init != nil {
			init(fr, res)
		}
		if cond(fr) {
			return then(fr, res)
		}
		if otherwise != nil {
			return otherwise(fr, res)
		}
		return ctlNext
	}
}

func (c *compiler) forStmt(s *ast.ForStmt) stmt {
	c.push()
	defer c.pop()
	var init, post stmt
	var loopVars []variable
	if s.Init != nil {
		before := c.fn.slots
		init = c.stmt(s.Init)
		for _, v := range c.fn.blocks[len(c.fn.blocks)-1] {
			if v.slot >= before {
				loopVars = append(loopVars, v)
			}
		}
	}
	var cond func(*frame) bool
	if s.Cond != nil {
		cond = c.cond(s.Cond)
	}
	if s.Post != nil {
		post = c.stmt(s.Post)
	}
	body := c.block(s.Body.List, true)
	return func(fr *frame, res *result) ctl {
		if init != nil {
			init(fr, res)
		}
		for cond == nil || cond(fr) {
			how := body(fr, res)
			if how == ctlBreak {
				break
			}
			if how == ctlReturn {
				return how
			}
			// Each pass has its own copy of the loop variables, so a
			// closure of one pass keeps the values of that pass.
			for _, v := range loopVars {
				newVar(fr, v.slot, v.typ, fr.vars[v.slot])
			}
			if post != nil {
				post(fr, res)
			}
		}
		return ctlNext
	}
}

func (c *compiler) rangeStmt(s *ast.RangeStmt) stmt {
	c.push()
	defer c.pop()
	x := c.expr(s.X, nil)

	x = c.typed(s.X, x, nil)
	var keyType, valType reflect.Type
	switch x.typ.Kind() {
	case reflect.Slice, reflect.Array:
		keyType, valType = typeInt, x.typ.Elem()
	case reflect.String:
		keyType, valType = typeInt, typeRune
	case reflect.Map:
		keyType, valType = x.typ.Key(), x.typ.Elem()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		keyType = x.typ
	default:
		// A range over a function (an iterator) or a channel.
		c.fail(s, "a range over %s is not interpreted", x.typ)
	}
	bind := func(e ast.Expr, typ reflect.Type) *target {
		if e == nil {
			return nil
		}
		if typ == nil {
			c.fail(e, "the range has no second value")
		}
		if id, ok := e.(*ast.Ident); ok && id.Name == "_" {
			return nil
		}
		if s.Tok == token.DEFINE {
			id, ok := e.(*ast.Ident)
			if !ok {
				c.fail(e, ":= declares names")
			}
			return &target{define: true, slot: c.declare(id.Name, typ), typ: typ}
		}
		t := c.lhs(e)
		return &t
	}
	key, val := bind(s.Key, keyType), bind(s.Value, valType)
	body := c.block(s.Body.List, true)
	eval := x.eval
	store := func(fr *frame, t *target, v reflect.Value) {
		if t == nil {
			return
		}
		if t.define {
			newVar(fr, t.slot, t.typ, v)
			return
		}
		t.set(fr, v)
	}
	kind := x.typ.Kind()
	return func(fr *frame, res *result) ctl {
		// Go reads the operand one time: a range over an array reads a
		// copy, and a range over a slice keeps the slice of the start.
		v := detach(eval(fr))
		step := func(k, e reflect.Value) (stop bool, how ctl) {
			store(fr, key, k)
			store(fr, val, e)
			how = body(fr, res)
			return how == ctlBreak || how == ctlReturn, how
		}
		switch kind {
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				if stop, how := step(reflect.ValueOf(i), v.Index(i)); stop {
					if how == ctlReturn {
						return how
					}
					break
				}
			}
		case reflect.String:
			for i, r := range v.String() {
				if stop, how := step(reflect.ValueOf(i), reflect.ValueOf(r)); stop {
					if how == ctlReturn {
						return how
					}
					break
				}
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				if stop, how := step(iter.Key(), iter.Value()); stop {
					if how == ctlReturn {
						return how
					}
					break
				}
			}
		default:
			n := v.Int()
			for i := int64(0); i < n; i++ {
				k := reflect.New(keyType).Elem()
				k.SetInt(i)
				if stop, how := step(k, reflect.Value{}); stop {
					if how == ctlReturn {
						return how
					}
					break
				}
			}
		}
		return ctlNext
	}
}

func (c *compiler) switchStmt(s *ast.SwitchStmt) stmt {
	c.push()
	defer c.pop()
	var init stmt
	if s.Init != nil {
		init = c.stmt(s.Init)
	}
	var tag *value
	if s.Tag != nil {
		v := c.typed(s.Tag, c.expr(s.Tag, nil), nil)
		tag = &v
	}
	type clause struct {
		matches []func(fr *frame, tag reflect.Value) bool
		body    stmt
	}
	var clauses []clause
	fallback := -1
	for _, raw := range s.Body.List {
		cc := raw.(*ast.CaseClause)
		var cl clause
		if cc.List == nil {
			fallback = len(clauses)
		}
		for _, e := range cc.List {
			if tag == nil {
				cond := c.cond(e)
				cl.matches = append(cl.matches, func(fr *frame, _ reflect.Value) bool { return cond(fr) })
				continue
			}
			a, b := c.operands(e, *tag, c.expr(e, tag.typ))
			be := b.eval
			convert := a.typ != tag.typ
			to := a.typ
			cl.matches = append(cl.matches, func(fr *frame, t reflect.Value) bool {
				if convert {
					nv := reflect.New(to).Elem()
					nv.Set(t)
					t = nv
				}
				return equal(t, be(fr))
			})
		}
		for _, st := range cc.Body {
			if br, ok := st.(*ast.BranchStmt); ok && br.Tok == token.FALLTHROUGH {
				c.fail(st, "fallthrough is not interpreted")
			}
		}
		cl.body = c.block(cc.Body, true)
		clauses = append(clauses, cl)
	}
	return func(fr *frame, res *result) ctl {
		if init != nil {
			init(fr, res)
		}
		var t reflect.Value
		if tag != nil {
			t = tag.eval(fr)
		}
		chosen := fallback
	find:
		for i, cl := range clauses {
			for _, match := range cl.matches {
				if match(fr, t) {
					chosen = i
					break find
				}
			}
		}
		if chosen < 0 {
			return ctlNext
		}
		how := clauses[chosen].body(fr, res)
		if how == ctlBreak {
			return ctlNext
		}
		return how
	}
}

func (c *compiler) returnStmt(s *ast.ReturnStmt) stmt {
	out := c.fn.out
	if len(s.Results) == 1 && len(out) != 1 {
		v := c.expr(s.Results[0], nil)
		if len(v.multi) != len(out) {
			c.fail(s, "the return has %d values for %d results", len(v.multi), len(out))
		}
		evalN := v.evalN
		return func(fr *frame, res *result) ctl {
			res.values = evalN(fr)
			return ctlReturn
		}
	}
	if len(s.Results) != len(out) {
		c.fail(s, "the return has %d values for %d results", len(s.Results), len(out))
	}
	evals := make([]func(*frame) reflect.Value, len(out))
	for i, e := range s.Results {
		evals[i] = c.typed(e, c.expr(e, out[i]), out[i]).eval
	}
	return func(fr *frame, res *result) ctl {
		values := make([]reflect.Value, len(evals))
		for i, e := range evals {
			values[i] = e(fr)
		}
		res.values = values
		return ctlReturn
	}
}
