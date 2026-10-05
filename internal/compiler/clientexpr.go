package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Client expressions (REQ-ACT-07) are a typed subset of Go that the
// compiler transpiles to adapter syntax. This file owns the action
// invocation half; signal transpilation and type rules join it in M4.

// collectActions records every gx.Action registration by route type
// (REQ-ACT-02).
func (r *typesResult) collectActions(pkgs []*packages.Package) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isGxFunc(pkg, call.Fun, "Action") || len(call.Args) == 0 {
					return true
				}
				sig, ok := pkg.TypesInfo.TypeOf(call.Args[0]).Underlying().(*types.Signature)
				if !ok || sig.Params().Len() != 2 {
					return true
				}
				named, ok := sig.Params().At(1).Type().(*types.Named)
				if !ok || named.Obj().Pkg() == nil {
					return true
				}
				key := named.Obj().Pkg().Path() + "." + named.Obj().Name()
				r.actions[key] = append(r.actions[key], pkg.Fset.Position(call.Pos()))
				return true
			})
		}
	}
}

// checkActionInvocations reports an on: attribute that invokes a route type
// with no registration (GX4001), more than one registration (GX4002) or an
// unsupported method (GX4009). checkToastActions covers the invocations in
// Go files.
func (l *loader) checkActionInvocations(res *typesResult, dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkElements(f.Body, func(el *Element) {
				for i := range el.Attrs {
					a := &el.Attrs[i]
					if a.Kind != AttrExpr || !strings.HasPrefix(a.Name, "on:") {
						continue
					}
					key := namedTypeKey(res.types[a])
					if key == "" || !res.routeKeys[key] {
						continue
					}
					out = append(out, res.invocationDiags(key, f.File, a.At.Line, a.At.Col)...)
					if def := res.routeDefs[key]; def != nil && len(res.actions[key]) > 0 {
						out = append(out, res.checkSignalFields(f, a, def)...)
					}
				}
			})
		}
	}
	return out
}

// checkToastActions reports a gx.ToastAction call in a Go file that invokes
// a route type with no registration (GX4001), more than one registration
// (GX4002) or an unsupported method (GX4009). The position is the call.
func (r *typesResult) checkToastActions(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isGxFuncExpr(pkg, call.Fun, "ToastAction") || len(call.Args) != 2 {
					return true
				}
				key := namedTypeKey(pkg.TypesInfo.TypeOf(call.Args[1]))
				if key == "" || !r.routeKeys[key] {
					return true
				}
				pos := pkg.Fset.Position(call.Pos())
				out = append(out, r.invocationDiags(key, pos.Filename, pos.Line, pos.Column)...)
				return true
			})
		}
	}
	return out
}

// invocationDiags checks one invocation of the route type key against the
// module index of registered actions (REQ-ACT-02).
func (r *typesResult) invocationDiags(key, file string, line, col int) []Diagnostic {
	var out []Diagnostic
	regs := r.actions[key]
	switch {
	case len(regs) == 0:
		out = append(out, Diagnostic{
			Code: CodeActionMissing,
			File: file,
			Line: line,
			Col:  col,
			Msg:  "no action is registered for " + Quoted(typeName(key)),
			Fix:  "add gx.Action for this route type",
		})
	case len(regs) > 1:
		out = append(out, Diagnostic{
			Code: CodeActionTwice,
			File: file,
			Line: line,
			Col:  col,
			Msg:  Quoted(typeName(key)) + " is registered " + itoa(len(regs)) + " times: " + positionList(regs),
			Fix:  "delete all but one gx.Action registration",
		})
	}
	if !clientInvocable(r.routeMeth[key]) {
		out = append(out, Diagnostic{
			Code: CodeActionMethod,
			File: file,
			Line: line,
			Col:  col,
			Msg:  "method " + r.routeMeth[key] + " of " + Quoted(typeName(key)) + " cannot be invoked from the client",
		})
	}
	return out
}

// checkSignalFields checks every signal-bound field of the invoked action
// against the signals declared in the invoking component (REQ-ACT-03).
func (r *typesResult) checkSignalFields(f *File, a *Attr, def *routeDef) []Diagnostic {
	var out []Diagnostic
	for _, fld := range def.fields {
		if fld.signal == "" {
			continue
		}
		sigType := r.sigTypes[f][lowerFirst(fld.name)]
		if sigType == nil {
			out = append(out, Diagnostic{
				Code: CodeActionMissingSignal,
				File: f.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "field " + Quoted(fld.name) + " reads signal " + Quoted(fld.signal) + " but this component declares no signal " + Quoted(fld.signal),
				Fix:  "add the signal to the signals block of this component",
			})
			continue
		}
		if !types.AssignableTo(sigType, fld.typ) {
			out = append(out, Diagnostic{
				Code: CodeSignalTypeMismatch,
				File: f.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "field " + Quoted(fld.name) + " has type " + Quoted(fld.typeText) + " but signal " + Quoted(fld.signal) + " has type " + Quoted(sigType.String()),
			})
		}
	}
	return out
}

// componentScoped reports whether a component carries client signals
// itself or through a component it renders (REQ-ACT-06).
func (r *typesResult) componentScoped(l *loader, pkg *Package, file *File, comp *Component, seen map[*File]bool) bool {
	return componentScoped(l, pkg, file, comp, r.scopedMap, seen)
}

// componentScoped reports whether a component carries client signals itself
// or through a component it renders (REQ-ACT-06). The memo caches results.
// The memo and seen maps key on the component file, which stays stable
// across resolve calls, so a recursive component terminates.
func componentScoped(l *loader, pkg *Package, file *File, comp *Component, memo, seen map[*File]bool) bool {
	if comp == nil || comp.File == nil {
		return false
	}
	if v, ok := memo[comp.File]; ok {
		return v
	}
	if seen[comp.File] {
		return false
	}
	seen[comp.File] = true
	defer delete(seen, comp.File)
	if len(comp.File.Signals) > 0 {
		memo[comp.File] = true
		return true
	}
	found := false
	walkElements(comp.File.Body, func(el *Element) {
		if found {
			return
		}
		qual, name, ok := componentTag(el.Name)
		if !ok {
			return
		}
		child, childPkg, _ := resolveComponent(l, pkg, comp.File, qual, name)
		if child != nil && componentScoped(l, childPkg, comp.File, child, memo, seen) {
			found = true
		}
	})
	memo[comp.File] = found
	return found
}

// namedTypeKey returns the package path and name of a named type, or "".
func namedTypeKey(t types.Type) string {
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

// typeName shortens a route type key to its name.
func typeName(key string) string {
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		return key[i+1:]
	}
	return key
}

func positionList(ps []token.Position) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ", ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// clientInvocable reports whether the client can invoke an action with the
// HTTP method (REQ-ACT-02). The adapter owns the syntax of the call
// (REQ-PLG-04).
func clientInvocable(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// checkSecrets reports gx.Secret values that would cross to the client
// (SI-04).
func (l *loader) checkSecrets(res *typesResult, dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			for _, s := range f.Signals {
				if isSecretType(res.sigTypes[f][lowerFirst(s.Name)]) {
					out = append(out, Diagnostic{
						Code: CodeSecret,
						File: f.File,
						Line: s.At.Line,
						Col:  s.At.Col,
						Msg:  "signal " + Quoted(s.Name) + " has type gx.Secret; a secret cannot cross to the client",
					})
				}
			}
		}
	}
	for _, site := range res.clientSites {
		reported := false
		ast.Inspect(site.node, func(n ast.Node) bool {
			if reported {
				return false
			}
			expr, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if isSecretType(res.exprTypes[expr]) {
				reported = true
				out = append(out, Diagnostic{
					Code: CodeSecret,
					File: site.file.File,
					Line: site.attr.ValueAt.Line,
					Col:  site.attr.ValueAt.Col,
					Msg:  "a gx.Secret value cannot enter a client expression",
				})
			}
			return true
		})
	}
	return out
}

// isSecretType reports whether a type is gx.Secret or a pointer to it.
func isSecretType(t types.Type) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	return t.String() == "github.com/alternayte/gx.Secret"
}

// checkSignalRules reports an action with signal-bound fields and no
// Rules() method or gx.Unchecked marker (SI-13).
func checkSignalRules(routes []*routeDef) []Diagnostic {
	var out []Diagnostic
	for _, d := range routes {
		if !d.action || !d.hasSignals {
			continue
		}
		obj, _ := d.pkg.Types.Scope().Lookup(d.name).(*types.TypeName)
		if obj == nil {
			continue
		}
		if hasRulesMethod(obj.Type()) || embedsUnchecked(obj.Type()) {
			continue
		}
		out = append(out, Diagnostic{
			Code: CodeSignalRules,
			File: d.file,
			Line: d.pos.Line,
			Col:  d.pos.Column,
			Msg:  "action " + Quoted(d.name) + " has signal-bound fields; add Rules() or embed gx.Unchecked",
			Fix:  "add func (in *" + d.name + ") Rules() gx.Rules { ... }",
		})
	}
	return out
}

// hasRulesMethod reports whether the type has a Rules method.
func hasRulesMethod(t types.Type) bool {
	for _, typ := range []types.Type{t, types.NewPointer(t)} {
		sel := types.NewMethodSet(typ).Lookup(nil, "Rules")
		if sel == nil {
			continue
		}
		sig, ok := sel.Obj().Type().(*types.Signature)
		if ok && sig.Params().Len() == 0 && sig.Results().Len() == 1 {
			return true
		}
	}
	return false
}

// embedsUnchecked reports whether the struct embeds gx.Unchecked.
func embedsUnchecked(t types.Type) bool {
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		if f.Type().String() == "github.com/alternayte/gx.Unchecked" {
			return true
		}
	}
	return false
}
