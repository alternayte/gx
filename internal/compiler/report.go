package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"net/http"
	"net/url"
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
	mounts, _ := mountsAndCollections(res)
	return mounts
}

// collectionOf returns the collection variable of a gx.ContentEntries or
// gx.ContentPages call.
func collectionOf(pkg *packages.Package, expr ast.Expr) types.Object {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	// A chained call such as .LLMS(...) wraps the ContentEntries call.
	for {
		if isGxFuncExpr(pkg, call.Fun, "ContentEntries") || isGxFuncExpr(pkg, call.Fun, "ContentPages") {
			break
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return nil
		}
		call = inner
	}
	if len(call.Args) == 0 {
		return nil
	}
	return identObject(pkg, call.Args[0])
}

// mountsAndCollections is mountInfo plus the mount prefix of each content
// collection, keyed by the collection variable (REQ-CNT-10).
func mountsAndCollections(res *typesResult) (map[string]mount, map[types.Object]string) {
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
			if coll := collectionOf(pkg, e); coll != nil {
				// The routes of a collection stand for the collection.
				return []types.Object{coll}
			}
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
	collections := map[types.Object]string{}
	isCollection := func(obj types.Object) bool {
		ptr, ok := obj.Type().(*types.Pointer)
		if !ok {
			return false
		}
		named, ok := ptr.Elem().(*types.Named)
		return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == gxPkgPath && named.Obj().Name() == "collection"
	}
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
							} else if isCollection(m) {
								collections[m] = cur.prefix
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
	return out, collections
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

// resolveContentMounts gives each collection its mount prefix and a test
// for the page routes of the app, so the link check knows the site paths
// (REQ-CNT-10).
func (res *typesResult) resolveContentMounts() {
	if len(res.collections) == 0 {
		return
	}
	mounts, prefixes := mountsAndCollections(res)
	// A mux with each GET page pattern answers "is this path a page".
	mux := http.NewServeMux()
	seen := map[string]bool{}
	for _, d := range res.routes {
		if d.pkg == nil || methodOf(d.pattern) != "GET" {
			continue
		}
		_, path, _ := strings.Cut(d.pattern, " ")
		prefix := strings.TrimSuffix(mounts[d.pkg.PkgPath+"."+d.name].prefix, "/")
		pattern := "GET " + prefix + "/" + strings.TrimPrefix(path, "/")
		if seen[pattern] {
			continue
		}
		seen[pattern] = true
		func() {
			// A pattern that the mux rejects is a diagnostic of its own.
			defer func() { _ = recover() }()
			mux.Handle(pattern, http.NotFoundHandler())
		}()
	}
	appPage := func(path string) bool {
		for _, candidate := range []string{path, strings.TrimSuffix(path, "/"), strings.TrimSuffix(path, "/") + "/"} {
			if candidate == "" {
				continue
			}
			u, err := url.Parse(candidate)
			if err != nil {
				continue
			}
			if _, pattern := mux.Handler(&http.Request{Method: http.MethodGet, URL: u}); pattern != "" {
				return true
			}
		}
		return false
	}
	byVar := map[string]string{}
	for obj, prefix := range prefixes {
		if obj.Pkg() != nil {
			byVar[obj.Pkg().Path()+"."+obj.Name()] = strings.TrimSuffix(prefix, "/")
		}
	}
	for i := range res.collections {
		coll := &res.collections[i]
		coll.prefix = byVar[coll.pkgPath+"."+coll.varName]
		coll.appPage = appPage
	}
}
