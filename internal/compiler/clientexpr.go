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
// unsupported method (GX4009).
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
					regs := res.actions[key]
					switch {
					case len(regs) == 0:
						out = append(out, Diagnostic{
							Code: CodeActionMissing,
							File: f.File,
							Line: a.At.Line,
							Col:  a.At.Col,
							Msg:  "no action is registered for " + Quoted(typeName(key)),
							Fix:  "add gx.Action for this route type",
						})
					case len(regs) > 1:
						out = append(out, Diagnostic{
							Code: CodeActionTwice,
							File: f.File,
							Line: a.At.Line,
							Col:  a.At.Col,
							Msg:  Quoted(typeName(key)) + " is registered " + itoa(len(regs)) + " times: " + positionList(regs),
							Fix:  "delete all but one gx.Action registration",
						})
					}
					if clientActionFunc(res.routeMeth[key]) == "" {
						out = append(out, Diagnostic{
							Code: CodeActionMethod,
							File: f.File,
							Line: a.At.Line,
							Col:  a.At.Col,
							Msg:  "method " + res.routeMeth[key] + " of " + Quoted(typeName(key)) + " cannot be invoked from the client",
						})
					}
					if def := res.routeDefs[key]; def != nil && len(regs) > 0 {
						out = append(out, res.checkSignalFields(f, a, def)...)
					}
				}
			})
		}
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

// clientActionFunc maps an HTTP method to the adapter client function
// (REQ-ACT-02). It returns "" for a method the client cannot invoke.
func clientActionFunc(method string) string {
	switch method {
	case "GET":
		return "get"
	case "POST":
		return "post"
	case "PUT":
		return "put"
	case "PATCH":
		return "patch"
	case "DELETE":
		return "delete"
	}
	return ""
}
