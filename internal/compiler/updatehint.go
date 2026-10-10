package compiler

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// updateHintValues is the number of dynamic values above which a component
// with no fragment gets the hint GX4012 when c.Update takes it.
const updateHintValues = 8

// updateHints returns a hint for each c.Update call whose argument is a
// component with no fragment and with many dynamic values (REQ-ACT-17). The
// fragment is the unit of an update (DR-12): the server sends the whole root
// of such a component for each change.
func (l *loader) updateHints(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.PositionFor(file.Pos(), false).Filename, "_gx.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 || !isCtxUpdate(pkg, call.Fun) {
					return true
				}
				arg, ok := ast.Unparen(call.Args[0]).(*ast.CallExpr)
				if !ok {
					return true
				}
				var id *ast.Ident
				switch fun := ast.Unparen(arg.Fun).(type) {
				case *ast.Ident:
					id = fun
				case *ast.SelectorExpr:
					id = fun.Sel
				}
				if id == nil {
					return true
				}
				fn, ok := pkg.TypesInfo.Uses[id].(*types.Func)
				if !ok {
					return true
				}
				// The position with no //line directive is the
				// generated file, next to the .gx file.
				dir := filepath.Dir(pkg.Fset.PositionFor(fn.Pos(), false).Filename)
				p := l.pkgs[filepath.Clean(dir)]
				if p == nil {
					return true
				}
				f, ok := p.Files[fn.Name()]
				if !ok || len(fragmentElements(f.Body)) > 0 {
					return true
				}
				values := dynamicValues(f.Body)
				if values <= updateHintValues {
					return true
				}
				pos := pkg.Fset.Position(call.Pos())
				out = append(out, Diagnostic{
					Code: CodeUpdateNoFragment,
					File: pos.Filename,
					Line: pos.Line,
					Col:  pos.Column,
					Msg: "c.Update takes the component " + Quoted(fn.Name()) + ", which has no fragment and " + strconv.Itoa(values) +
						" dynamic values: the server sends the whole component for each change. Mark the parts that change with #name",
					Hint: true,
				})
				return true
			})
		}
	}
	return out
}

// isCtxUpdate reports whether fun is the Update method of *gx.Ctx.
func isCtxUpdate(pkg *packages.Package, fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Update" {
		return false
	}
	fn, ok := pkg.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/alternayte/gx" {
		return false
	}
	recv := fn.Type().(*types.Signature).Recv()
	return recv != nil && strings.HasSuffix(recv.Type().String(), "gx.Ctx")
}

// dynamicValues counts the expressions of a component body: each {expr} and
// each attribute with an expression.
func dynamicValues(ns []Node) int {
	n := 0
	for _, node := range ns {
		switch t := node.(type) {
		case *Expr:
			n++
		case *Element:
			for i := range t.Attrs {
				if t.Attrs[i].Kind == AttrExpr || t.Attrs[i].Kind == AttrSpread {
					n++
				}
			}
			n += dynamicValues(t.Children)
		case *Control:
			n += dynamicValues(t.Body) + dynamicValues(t.Else)
			for _, c := range t.Cases {
				n += dynamicValues(c.Body)
			}
		}
	}
	return n
}
