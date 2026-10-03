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
	collectVars := map[types.Object][]types.Object{}
	for _, pkg := range res.pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, val := range vs.Values {
						call, ok := val.(*ast.CallExpr)
						if !ok || !isGxFunc(pkg, call.Fun, "Collect") || i >= len(vs.Names) {
							continue
						}
						obj := pkg.TypesInfo.Defs[vs.Names[i]]
						if obj == nil {
							continue
						}
						for _, arg := range call.Args {
							if member := identObject(pkg, arg); member != nil {
								collectVars[obj] = append(collectVars[obj], member)
							}
						}
					}
				}
			}
		}
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
					if members, ok := collectVars[obj]; ok {
						for _, m := range members {
							if key, ok := res.pageRoutes[m]; ok {
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
