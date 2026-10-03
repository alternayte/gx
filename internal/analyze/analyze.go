// Package analyze holds the Gx go/analysis passes: route packages (GX3005)
// and safe HTML conversions (GX7001). gx lint runs them over a module and
// the lintplugin package registers them with golangci-lint (REQ-TLS-03).
package analyze

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// gxPath is the import path of the runtime package.
const gxPath = "github.com/alternayte/gx"

// SafeHTML reports a gx.SafeHTML conversion of a non-constant value without
// //gx:trusted on the same line (GX7001, SI-01).
var SafeHTML = &analysis.Analyzer{
	Name: "gxsafehtml",
	Doc:  "report a gx.SafeHTML conversion of a non-constant value without //gx:trusted (GX7001)",
	Run:  runSafeHTML,
}

// RoutePackage reports code other than route types in a route package
// (GX3005, REQ-RTE-20).
var RoutePackage = &analysis.Analyzer{
	Name: "gxroutepkg",
	Doc:  "report code other than route types in a route package (GX3005)",
	Run:  runRoutePackage,
}

// Analyzers returns the Gx analyzers in a stable order (REQ-TLS-03).
func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{SafeHTML, RoutePackage}
}

func runSafeHTML(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		trusted := map[int]bool{}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if strings.Contains(c.Text, "gx:trusted") {
					trusted[pass.Fset.Position(c.Pos()).Line] = true
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			obj := called(pass.TypesInfo, call.Fun)
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != gxPath || obj.Name() != "SafeHTML" {
				return true
			}
			if tv, ok := pass.TypesInfo.Types[call.Args[0]]; ok && tv.Value != nil {
				return true
			}
			if trusted[pass.Fset.Position(call.Pos()).Line] {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos:      call.Pos(),
				End:      call.End(),
				Category: "GX7001",
				Message:  "conversion to gx.SafeHTML needs //gx:trusted <reason>",
			})
			return true
		})
	}
	return nil, nil
}

func runRoutePackage(pass *analysis.Pass) (any, error) {
	if !strings.HasSuffix(pass.Pkg.Path(), "/route") {
		return nil, nil
	}
	routeNames := map[string]bool{}
	type fileInfo struct {
		file *ast.File
		name string
	}
	var files []fileInfo
	for _, file := range pass.Files {
		name := pass.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(name, "_gx.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, fileInfo{file: file, name: name})
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if isRouteStruct(pass, st) {
					routeNames[ts.Name.Name] = true
				}
			}
		}
	}
	for _, fi := range files {
		for _, imp := range fi.file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil || path == gxPath || !strings.Contains(strings.Split(path, "/")[0], ".") {
				continue
			}
			pass.Report(analysis.Diagnostic{
				Pos:      imp.Pos(),
				End:      imp.End(),
				Category: "GX3005",
				Message:  "route package imports " + quote(path) + "; only gx and the standard library are allowed",
			})
		}
		for _, decl := range fi.file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok.String() == "import" {
					continue
				}
				if d.Tok.String() != "type" {
					pass.Report(analysis.Diagnostic{
						Pos:      d.Pos(),
						End:      d.End(),
						Category: "GX3005",
						Message:  "route package holds a " + d.Tok.String() + "; only route types are allowed",
					})
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); ok {
						continue
					}
					pass.Report(analysis.Diagnostic{
						Pos:      ts.Pos(),
						End:      ts.End(),
						Category: "GX3005",
						Message:  "route package declares " + quote(ts.Name.Name) + " which is not a route type",
					})
				}
			case *ast.FuncDecl:
				if d.Recv != nil && len(d.Recv.List) == 1 && routeNames[receiverName(d.Recv.List[0].Type)] {
					continue
				}
				pass.Report(analysis.Diagnostic{
					Pos:      d.Pos(),
					End:      d.End(),
					Category: "GX3005",
					Message:  "route package holds a func; only route types are allowed",
				})
			}
		}
	}
	return nil, nil
}

// isRouteStruct reports whether a struct embeds gx.Route with a method and
// pattern tag.
func isRouteStruct(pass *analysis.Pass, st *ast.StructType) bool {
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		t := pass.TypesInfo.TypeOf(field.Type)
		named, ok := t.(*types.Named)
		if !ok || named.Obj().Pkg() == nil {
			continue
		}
		if named.Obj().Pkg().Path() == gxPath && named.Obj().Name() == "Route" {
			return true
		}
	}
	return false
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr:
		return receiverName(t.X)
	}
	return ""
}

func called(info *types.Info, fun ast.Expr) types.Object {
	switch f := fun.(type) {
	case *ast.Ident:
		return info.Uses[f]
	case *ast.SelectorExpr:
		return info.Uses[f.Sel]
	}
	return nil
}

func quote(s string) string { return strconv.Quote(s) }
