package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	goprinter "go/printer"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// clientSite is one attributed parsed client expression (REQ-ACT-07).
// The AST is the rewritten probe form: $Qty is _gxSig_Qty and a
// gx.SignalRef prop read p.Open is _gxRef_Open.
type clientSite struct {
	file *File
	attr *Attr
	node ast.Node
	// block is true when the attribute holds statements, as in an on:
	// handler.
	block bool
}

// rewriteClient replaces signal references and signal-ref props so the
// expression is valid Go (REQ-ACT-07).
func rewriteClient(src string, refProps map[string]bool) (string, error) {
	var b strings.Builder
	for i := 0; i < len(src); {
		c := src[i]
		switch c {
		case '"', '\'', '`':
			j := endOfGoLiteral(src, i)
			if j <= i {
				return "", fmt.Errorf("unterminated literal")
			}
			b.WriteString(src[i:j])
			i = j
		case '$':
			name := identAfter(src, i+1)
			if name == "" {
				b.WriteByte(c)
				i++
				continue
			}
			b.WriteString("_gxSig_")
			b.WriteString(name)
			i += 1 + len(name)
		case 'p':
			if i > 0 && isIdentByte(src[i-1]) || len(refProps) == 0 || i+1 >= len(src) || src[i+1] != '.' {
				b.WriteByte(c)
				i++
				continue
			}
			name := identAfter(src, i+2)
			if name == "" || !refProps[name] {
				b.WriteByte(c)
				i++
				continue
			}
			b.WriteString("_gxRef_")
			b.WriteString(name)
			i += 2 + len(name)
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

// endOfGoLiteral returns the index after a Go string or rune literal that
// starts at i.
func endOfGoLiteral(src string, i int) int {
	quote := src[i]
	if quote == '`' {
		for j := i + 1; j < len(src); j++ {
			if src[j] == '`' {
				return j + 1
			}
		}
		return -1
	}
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case quote:
			return j + 1
		}
	}
	return -1
}

// identAfter returns the identifier that starts at i, or "".
func identAfter(src string, i int) string {
	j := i
	for j < len(src) && (isIdentByte(src[j])) {
		j++
	}
	return src[i:j]
}

// parseClient parses a rewritten client expression. It returns an
// expression node, or a statement block for an on: handler.
func parseClient(rewritten string) (node ast.Node, block bool, err error) {
	if expr, err := goparser.ParseExpr(rewritten); err == nil {
		return expr, false, nil
	}
	src := "package p\nfunc _gxClient() {\n" + rewritten + "\n}\n"
	file, err := goparser.ParseFile(token.NewFileSet(), "client.go", src, 0)
	if err != nil {
		return nil, false, err
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) == 0 {
		return nil, false, fmt.Errorf("not a client expression")
	}
	return fn.Body, true, nil
}

// needsClient reports whether an attribute holds a client expression
// (REQ-ACT-07).
func needsClient(name, value string) bool {
	if strings.Contains(value, "$") {
		return true
	}
	if name == "show" || name == "text" {
		return true
	}
	if name == "key" {
		return false
	}
	for _, prefix := range []string{"bind:", "attr:", "on:"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// signalNames returns the declared signals of a file by lower-first name.
func signalNames(f *File) map[string]bool {
	out := map[string]bool{}
	for _, s := range f.Signals {
		out[lowerFirst(s.Name)] = true
	}
	return out
}

// signalRefProps returns the props that carry a signal reference, keyed by
// prop name with the element type as value (REQ-ACT-07).
func signalRefProps(f *File) map[string]string {
	out := map[string]string{}
	for _, p := range f.Props {
		if elem, ok := signalRefArg(p.Type); ok {
			out[p.Name] = elem
		}
	}
	return out
}

// signalRefArg returns T of a gx.SignalRef[T] prop type.
func signalRefArg(typ string) (string, bool) {
	for _, prefix := range []string{"gx.SignalRef[", "SignalRef["} {
		if strings.HasPrefix(typ, prefix) && strings.HasSuffix(typ, "]") {
			return strings.TrimSpace(typ[len(prefix) : len(typ)-1]), true
		}
	}
	return "", false
}

// adapterAttrName maps a client directive name to the adapter attribute
// name, including event modifiers and special events (REQ-ACT-08).
func adapterAttrName(name string) (string, error) {
	if strings.HasPrefix(name, "bind:") {
		// Datastar v1 binds by signal path in the value and reads the
		// property from the element type.
		return "data-bind", nil
	}
	if !strings.HasPrefix(name, "on:") {
		return "data-" + name, nil
	}
	event, mods := splitOnSpec(strings.TrimPrefix(name, "on:"))
	out := "data-on:" + event
	switch {
	case event == "load":
		out = "data-on-init"
	case event == "visible":
		out = "data-on-intersect"
	case event == "interval" || strings.HasPrefix(event, "interval("):
		out = "data-on-interval"
		if dur, ok := parenValue(event); ok {
			out += "__duration." + dur
		}
	}
	for _, mod := range mods {
		label, value := splitMod(mod)
		switch label {
		case "prevent", "stop", "once", "outside", "window", "capture", "passive":
			out += "__" + label
		case "debounce", "throttle", "delay":
			if value == "" {
				return "", fmt.Errorf("%s() needs a duration", label)
			}
			out += "__" + label + "." + value
		default:
			return "", fmt.Errorf("unknown modifier .%s", label)
		}
	}
	return out, nil
}

// splitOnSpec splits an on: spec into the event and its modifiers.
func splitOnSpec(spec string) (string, []string) {
	depth := 0
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '.':
			if depth == 0 {
				return spec[:i], splitTopDots(spec[i+1:])
			}
		}
	}
	return spec, nil
}

// splitTopDots splits s at dots outside parentheses.
func splitTopDots(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '.':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// splitMod splits a modifier into its label and value.
func splitMod(mod string) (string, string) {
	if label, value, ok := strings.Cut(mod, "("); ok {
		return label, strings.TrimSuffix(value, ")")
	}
	return mod, ""
}

// parenValue returns the text inside the parentheses of s.
func parenValue(s string) (string, bool) {
	i := strings.IndexByte(s, '(')
	if i < 0 || !strings.HasSuffix(s, ")") {
		return "", false
	}
	return s[i+1 : len(s)-1], true
}

// checkClientSite reports calls and types the transpiler cannot represent
// (REQ-ACT-05, REQ-ACT-13).
func (r *typesResult) checkClientSite(site *clientSite) []Diagnostic {
	var out []Diagnostic
	report := func(n ast.Node, code, msg string) {
		at := site.attr.ValueAt
		out = append(out, Diagnostic{Code: code, File: site.file.File, Line: at.Line, Col: at.Col, Msg: msg})
	}
	if _, err := adapterAttrName(site.attr.Name); err != nil {
		report(site.node, CodeEventMod, err.Error())
	}
	if strings.HasPrefix(site.attr.Name, "bind:") {
		id, ok := site.node.(*ast.Ident)
		if site.block || !ok || !strings.HasPrefix(id.Name, "_gxSig_") {
			report(site.node, CodeClientType, "bind: targets one signal")
		}
	}
	ast.Inspect(site.node, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.CallExpr:
			if name, ok := gxcHelper(t.Fun); !ok {
				report(t, CodeClientCall, "only gxc helpers are allowed in a client expression")
			} else if !knownHelper(name) {
				report(t, CodeClientCall, "gxc."+name+" has no client equivalent")
			}
		case *ast.BinaryExpr:
			if !clientBinaryOps[t.Op] {
				report(t, CodeClientType, "operator "+t.Op.String()+" has no equal result in JavaScript")
			}
			for _, operand := range []ast.Expr{t.X, t.Y} {
				if name := unsafeNumber(r.exprTypes[operand]); name != "" {
					report(t, CodeClientType, "a client expression allows int and float64 numbers, not "+name)
				}
			}
			switch t.Op {
			case token.EQL, token.NEQ:
				l, rt := r.exprTypes[t.X], r.exprTypes[t.Y]
				if !clientComparable(l) || !clientComparable(rt) {
					report(t, CodeClientType, "== and != allow bool, string and number operands only")
				}
			}
		case *ast.UnaryExpr:
			if t.Op != token.NOT && t.Op != token.SUB && t.Op != token.ADD {
				report(t, CodeClientType, "operator "+t.Op.String()+" has no equal result in JavaScript")
			}
			if name := unsafeNumber(r.exprTypes[t.X]); name != "" {
				report(t, CodeClientType, "a client expression allows int and float64 numbers, not "+name)
			}
		}
		return true
	})
	return out
}

// clientBinaryOps are the operators that give the same result in Go and in
// JavaScript (DR-05, REQ-ACT-13).
var clientBinaryOps = map[token.Token]bool{
	token.ADD: true, token.SUB: true, token.MUL: true, token.QUO: true, token.REM: true,
	token.LSS: true, token.LEQ: true, token.GTR: true, token.GEQ: true,
	token.EQL: true, token.NEQ: true, token.LAND: true, token.LOR: true,
}

// unsafeNumber returns the name of a number type that JavaScript does not
// compute as Go does: an unsigned or sized integer wraps, and a float32
// rounds. It returns "" for int, float64 and every type that is not a
// number (REQ-ACT-13).
func unsafeNumber(t types.Type) string {
	if t == nil {
		return ""
	}
	b, ok := t.Underlying().(*types.Basic)
	if !ok || !isNumeric(b.Info()) {
		return ""
	}
	switch b.Kind() {
	case types.Int, types.Float64, types.UntypedInt, types.UntypedFloat, types.UntypedRune:
		return ""
	}
	return b.Name()
}

// clientComparable reports whether a type has the same equality semantics in
// Go and in JavaScript (REQ-ACT-13).
func clientComparable(t types.Type) bool {
	if t == nil {
		return true
	}
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	info := b.Info()
	return info&(types.IsBoolean|types.IsString) != 0 || isNumeric(info)
}

func isNumeric(info types.BasicInfo) bool {
	return info&(types.IsFloat|types.IsInteger) != 0
}

// gxcHelper returns the name of a gxc helper call, when the expression is
// one. The probe imports the package only when the .gx file imports it, so
// the qualifier is enough for the grammar check.
func gxcHelper(fun ast.Expr) (string, bool) {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "gxc" {
		return "", false
	}
	return sel.Sel.Name, true
}

// knownHelper reports whether a gxc helper has a JavaScript equivalent
// (REQ-ACT-13).
func knownHelper(name string) bool {
	switch name {
	case "Len", "At", "Contains", "Index":
		return true
	}
	return false
}

// transpiler turns a client expression into a Go expression of type string
// that builds the adapter syntax at render time (REQ-ACT-07).
type transpiler struct {
	res       *typesResult
	file      *File
	scopeBase string
	keyExpr   string
	// scoped sends the instance scope with every action call, so the
	// handler can patch and set the signals of this instance
	// (REQ-ACT-03, REQ-ACT-05).
	scoped bool
}

// transpileValue returns a Go expression that builds the adapter value.
func (t *transpiler) transpileValue(expr ast.Expr) (string, error) {
	if !t.hasSignal(expr) {
		return "gx.JSON(" + printNode(expr) + ")", nil
	}
	switch e := expr.(type) {
	case *ast.ParenExpr:
		inner, err := t.transpileValue(e.X)
		if err != nil {
			return "", err
		}
		return "(\"(\" + " + inner + " + \")\")", nil
	case *ast.BasicLit:
		return jsLiteral(e)
	case *ast.Ident:
		if term, ok := t.identTerm(e); ok {
			return term, nil
		}
		return "", fmt.Errorf("identifier %s has no client value", e.Name)
	case *ast.UnaryExpr:
		inner, err := t.transpileValue(e.X)
		if err != nil {
			return "", err
		}
		op := e.Op.String()
		return strconv.Quote(op) + " + " + inner, nil
	case *ast.BinaryExpr:
		return t.binary(e)
	case *ast.CallExpr:
		return t.call(e)
	default:
		return "", fmt.Errorf("expression is not allowed in a client value")
	}
}

// transpileStatements returns a Go expression that builds the adapter
// statement string of an on: handler.
func (t *transpiler) transpileStatements(block *ast.BlockStmt) (string, error) {
	var parts []string
	for _, stmt := range block.List {
		part, err := t.transpileStmt(stmt)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return strconv.Quote(""), nil
	}
	return strings.Join(parts, ` + "; " + `), nil
}

func (t *transpiler) transpileStmt(stmt ast.Stmt) (string, error) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Tok != token.ASSIGN || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return "", fmt.Errorf("only = is allowed in a signal statement")
		}
		lhs, err := t.assignTarget(s.Lhs[0])
		if err != nil {
			return "", err
		}
		rhs, err := t.transpileValue(s.Rhs[0])
		if err != nil {
			return "", err
		}
		return lhs + ` + " = " + ` + rhs, nil
	case *ast.IncDecStmt:
		lhs, err := t.assignTarget(s.X)
		if err != nil {
			return "", err
		}
		return lhs + " + " + strconv.Quote(s.Tok.String()), nil
	case *ast.ExprStmt:
		if key := namedTypeKey(t.res.exprTypes[s.X]); key != "" && t.res.routeKeys[key] {
			return t.actionValue(s.X, key)
		}
		return "", fmt.Errorf("only signal statements and action calls are allowed in an on: handler")
	default:
		return "", fmt.Errorf("statement is not allowed in an on: handler")
	}
}

// assignTarget returns the signal reference of an assignment target.
func (t *transpiler) assignTarget(expr ast.Expr) (string, error) {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf("only a signal can be assigned")
	}
	name, ok := strings.CutPrefix(id.Name, "_gxSig_")
	if !ok {
		return "", fmt.Errorf("only a signal can be assigned")
	}
	return t.signalTerm(name), nil
}

// actionValue returns the Go expression of the action call for a route
// literal (REQ-ACT-02, REQ-ACT-03). The adapter writes the call when the
// node renders (REQ-PLG-04).
func (t *transpiler) actionValue(call ast.Expr, key string) (string, error) {
	method := t.res.routeMeth[key]
	if !clientInvocable(method) {
		return "", fmt.Errorf("method %s cannot be invoked from the client", method)
	}
	scope := strconv.Quote("")
	if t.scoped {
		scope = "gx.ScopeString(" + strconv.Quote(t.scopeBase) + ", " + t.keyExpr + ")"
	}
	return "gx.Invoke(" + strconv.Quote(method) + ", (" + printNode(call) + ").URL(), " + scope + ").Value", nil
}

// binary transpiles an operator expression (REQ-ACT-13).
func (t *transpiler) binary(e *ast.BinaryExpr) (string, error) {
	l, err := t.transpileValue(e.X)
	if err != nil {
		return "", err
	}
	r, err := t.transpileValue(e.Y)
	if err != nil {
		return "", err
	}
	op := jsOp(e.Op)
	if e.Op == token.QUO && intExpr(t.res.exprTypes[e.X]) && intExpr(t.res.exprTypes[e.Y]) {
		return `"Math.trunc((" + ` + l + ` + ") / (" + ` + r + ` + "))"`, nil
	}
	return strconv.Quote("(") + " + " + l + " + " + strconv.Quote(" "+op+" ") + " + " + r + " + " + strconv.Quote(")"), nil
}

// call transpiles a gxc helper call (REQ-ACT-13).
func (t *transpiler) call(e *ast.CallExpr) (string, error) {
	sel, ok := e.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", fmt.Errorf("only gxc helpers are allowed in a client expression")
	}
	name := sel.Sel.Name
	var js string
	switch name {
	case "Len":
		js = "__gx.len"
	case "At":
		js = "__gx.at"
	case "Contains":
		js = "__gx.contains"
	case "Index":
		js = "__gx.index"
	default:
		return "", fmt.Errorf("gxc.%s has no client equivalent", name)
	}
	var args []string
	for _, a := range e.Args {
		v, err := t.transpileValue(a)
		if err != nil {
			return "", err
		}
		args = append(args, v)
	}
	parts := []string{strconv.Quote(js + "(")}
	for i, a := range args {
		if i > 0 {
			parts = append(parts, strconv.Quote(", "))
		}
		parts = append(parts, a)
	}
	parts = append(parts, strconv.Quote(")"))
	return strings.Join(parts, " + "), nil
}

// identTerm returns the Go expression of a signal or signal-ref identifier.
func (t *transpiler) identTerm(id *ast.Ident) (string, bool) {
	if name, ok := strings.CutPrefix(id.Name, "_gxSig_"); ok {
		return t.signalTerm(name), true
	}
	if name, ok := strings.CutPrefix(id.Name, "_gxRef_"); ok {
		return `"$" + gx.RefPath(p.` + name + `)`, true
	}
	return "", false
}

// signalTerm returns the Go expression of the adapter path of one signal.
func (t *transpiler) signalTerm(name string) string {
	return "gx.SignalPath(" + strconv.Quote(t.scopeBase) + ", " + t.keyExpr + ", " + strconv.Quote(lowerFirst(name)) + ")"
}

// hasSignal reports whether an expression reads a signal or signal ref.
func (t *transpiler) hasSignal(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "_gxSig_") || strings.HasPrefix(id.Name, "_gxRef_") {
				found = true
			}
		}
		return !found
	})
	return found
}

// jsLiteral returns the JavaScript literal of a Go literal.
func jsLiteral(lit *ast.BasicLit) (string, error) {
	switch lit.Kind {
	case token.STRING:
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return "", err
		}
		return strconv.Quote(s), nil
	case token.INT, token.FLOAT:
		return lit.Value, nil
	case token.CHAR:
		return "", fmt.Errorf("rune literals are not allowed in a client expression")
	default:
		return "", fmt.Errorf("literal is not allowed in a client expression")
	}
}

// jsOp maps a Go operator to its JavaScript form (REQ-ACT-13).
func jsOp(op token.Token) string {
	switch op {
	case token.EQL:
		return "==="
	case token.NEQ:
		return "!=="
	case token.LAND:
		return "&&"
	case token.LOR:
		return "||"
	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM,
		token.LSS, token.LEQ, token.GTR, token.GEQ:
		return op.String()
	}
	return op.String()
}

// intExpr reports whether an expression type is an integer type.
func intExpr(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

// printNode returns the Go source of an AST node.
func printNode(n ast.Node) string {
	var b bytes.Buffer
	_ = goprinter.Fprint(&b, token.NewFileSet(), n)
	return b.String()
}
