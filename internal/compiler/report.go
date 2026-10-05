package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RouteReport is one entry of `gx routes` (REQ-RTE-14).
type RouteReport struct {
	Type       string   `json:"type"`
	Method     string   `json:"method"`
	Pattern    string   `json:"pattern"`
	Fields     []string `json:"fields"`
	Page       string   `json:"page,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
	Layouts    []string `json:"layouts,omitempty"`
	Middleware []string `json:"middleware,omitempty"`
}

// Routes reports every route of the module under root.
func Routes(root string) ([]RouteReport, []Diagnostic) {
	root = absoluteRoot(root)
	l := newLoader()
	dirs := collectDirs(root)
	res, diags := l.analyze(root, dirs)
	if len(diags) > 0 {
		sortDiags(diags)
		return nil, diags
	}
	mounts := mountInfo(res)
	var out []RouteReport
	for _, d := range res.routes {
		key := d.pkg.PkgPath + "." + d.name
		m := mounts[key]
		rep := RouteReport{
			Type:       d.pkg.Name + "." + d.name,
			Method:     methodOf(d.pattern),
			Pattern:    d.pattern,
			Page:       res.routePages[key],
			Prefix:     m.prefix,
			Layouts:    m.layouts,
			Middleware: m.middleware,
		}
		for _, f := range d.fields {
			rep.Fields = append(rep.Fields, f.name+" "+f.typeText)
		}
		out = append(out, rep)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pattern < out[j].Pattern })
	return out, nil
}

func methodOf(pattern string) string {
	method, _, _ := strings.Cut(pattern, " ")
	return method
}

type mount struct {
	prefix     string
	layouts    []string
	middleware []string
}

// mountInfo maps route types to the prefix, layouts and middleware of their
// Group call.
func mountInfo(res *typesResult) map[string]mount {
	// A route list is a gx.Collect call, an append of route lists, or the
	// name of another list. collectExprs holds the value of each
	// package-level variable; members resolves a list to its handlers.
	collectExprs := map[types.Object]struct {
		pkg  *packages.Package
		expr ast.Expr
	}{}
	eachPackageVar(res, func(pkg *packages.Package, name *ast.Ident, value ast.Expr) {
		if obj := pkg.TypesInfo.Defs[name]; obj != nil {
			collectExprs[obj] = struct {
				pkg  *packages.Package
				expr ast.Expr
			}{pkg, value}
		}
	})
	memo := map[types.Object][]types.Object{}
	visiting := map[types.Object]bool{}
	var members func(obj types.Object) []types.Object
	var exprMembers func(pkg *packages.Package, expr ast.Expr) []types.Object
	exprMembers = func(pkg *packages.Package, expr ast.Expr) []types.Object {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			return exprMembers(pkg, e.X)
		case *ast.CallExpr:
			var out []types.Object
			if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "append" || isGxFunc(pkg, e.Fun, "Collect") {
				for _, arg := range e.Args {
					out = append(out, exprMembers(pkg, arg)...)
				}
			}
			return out
		case *ast.Ident, *ast.SelectorExpr:
			obj := identObject(pkg, expr)
			if obj == nil {
				return nil
			}
			if _, _, ok := handlerRoute(obj); ok {
				return []types.Object{obj}
			}
			return members(obj)
		}
		return nil
	}
	members = func(obj types.Object) []types.Object {
		if got, ok := memo[obj]; ok {
			return got
		}
		src, ok := collectExprs[obj]
		if !ok || visiting[obj] {
			return nil
		}
		visiting[obj] = true
		got := exprMembers(src.pkg, src.expr)
		visiting[obj] = false
		memo[obj] = got
		return got
	}

	out := map[string]mount{}
	for _, pkg := range res.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Group" {
					return true
				}
				recv := pkg.TypesInfo.TypeOf(sel.X)
				if recv == nil || !strings.HasSuffix(recv.String(), "gx.App") {
					return true
				}
				var cur mount
				if len(call.Args) > 0 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if p, err := strconv.Unquote(lit.Value); err == nil {
							cur.prefix = p
						}
					}
				}
				for _, arg := range call.Args[1:] {
					obj := identObject(pkg, arg)
					if obj == nil {
						continue
					}
					if _, key, ok := handlerRoute(obj); ok {
						out[key] = cur
						continue
					}
					if list := members(obj); len(list) > 0 {
						for _, m := range list {
							if _, key, ok := handlerRoute(m); ok {
								out[key] = cur
							}
						}
						continue
					}
					typ := obj.Type().String()
					switch {
					case strings.Contains(typ, "gx.layout["):
						cur.layouts = append(cur.layouts, obj.Name())
					case strings.Contains(typ, "http.Handler) ") && strings.HasSuffix(typ, "http.Handler"):
						cur.middleware = append(cur.middleware, obj.Name())
					}
				}
				return true
			})
		}
	}
	return out
}

// identObject returns the object an identifier or a qualified identifier
// names.
func identObject(pkg *packages.Package, expr ast.Expr) types.Object {
	switch e := expr.(type) {
	case *ast.Ident:
		return pkg.TypesInfo.Uses[e]
	case *ast.SelectorExpr:
		return pkg.TypesInfo.Uses[e.Sel]
	}
	return nil
}
