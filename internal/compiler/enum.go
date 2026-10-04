package compiler

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// checkEnums reports a gx.Enum[T] literal that misses a constant of T
// (GX5001, REQ-STY-05).
func checkEnums(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_gx.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				named, ok := pkg.TypesInfo.TypeOf(lit).(*types.Named)
				if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
					return true
				}
				if named.Obj().Pkg().Path() != "github.com/alternayte/gx" || named.Obj().Name() != "Enum" {
					return true
				}
				args := named.TypeArgs()
				if args == nil || args.Len() != 1 {
					return true
				}
				have := map[string]bool{}
				for _, elt := range lit.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if id, ok := kv.Key.(*ast.Ident); ok {
						have[id.Name] = true
					}
				}
				pos := pkg.Fset.Position(lit.Pos())
				for _, c := range enumConstants(args.At(0)) {
					if have[c.Name()] {
						continue
					}
					out = append(out, Diagnostic{
						Code: CodeEnum,
						File: filepath.Clean(pos.Filename),
						Line: pos.Line,
						Col:  pos.Column,
						Msg:  "gx.Enum misses " + Quoted(c.Name()) + "; every constant of the type needs an entry",
						Fix:  "add " + c.Name() + ": \"<classes>\"",
					})
				}
				return true
			})
		}
	}
	return out
}

// enumConstants returns the package-level constants of a named type.
func enumConstants(t types.Type) []*types.Const {
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return nil
	}
	scope := named.Obj().Pkg().Scope()
	var out []*types.Const
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok {
			continue
		}
		if types.Identical(c.Type(), named) {
			out = append(out, c)
		}
	}
	return out
}
