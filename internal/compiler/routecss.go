package compiler

import (
	"encoding/json"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// routeClassesPath is the file with the class list of each page route that
// has a stylesheet of its own. The stylesheet build makes one stylesheet
// for each distinct list (REQ-STY-13).
const routeClassesPath = ".gx/route-classes.json"

// routeClasses returns, for the pattern of each page route, the classes
// that the page can use (REQ-STY-13). The rule is the rule of the app class
// list (REQ-STY-02), for a part of the packages of the app:
//
//   - the package that declares the page, the package of its view, and each
//     package of the module that one of them imports, with the imports of
//     those. An action of the page and each component that it patches are in
//     these packages;
//   - the packages of the component tree of each layout view, of each error
//     view and of the toast of the app, which each page can show.
//
// A page is in the result only when the compiler sees its view: a
// gx.Page(load, View) call whose view is a generated component. A page whose
// list is the list of the app is not in the result. Each other page links
// the stylesheet of the app.
func (l *loader) routeClasses(res *typesResult, root string) map[string][]string {
	out := map[string][]string{}
	if res == nil {
		return out
	}
	module := findModule(root)
	if module == nil {
		return out
	}
	// The packages of the module, by import path, with the imports of
	// each one.
	byPath := map[string]*packages.Package{}
	packages.Visit(res.pkgs, nil, func(pkg *packages.Package) {
		if _, ok := moduleDir(module, pkg.PkgPath); ok {
			byPath[pkg.PkgPath] = pkg
		}
	})
	// The classes of one package: its .gx files and the string literals
	// of its Go files.
	withSyntax := map[string]*packages.Package{}
	for _, pkg := range res.pkgs {
		withSyntax[pkg.PkgPath] = pkg
	}
	classCache := map[string][]string{}
	classesOf := func(pkgPath string) []string {
		if classes, ok := classCache[pkgPath]; ok {
			return classes
		}
		seen := map[string]bool{}
		add := func(text string) {
			for _, class := range strings.Fields(text) {
				seen[class] = true
			}
		}
		if dir, ok := moduleDir(module, pkgPath); ok {
			for _, f := range l.load(dir).Files {
				collectFileClasses(f.Body, add)
			}
		}
		if pkg := withSyntax[pkgPath]; pkg != nil {
			for _, file := range pkg.Syntax {
				path := pkg.Fset.PositionFor(file.Pos(), false).Filename
				if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
					continue
				}
				ast.Inspect(file, func(n ast.Node) bool {
					if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if s, ok := unquoteGo(lit.Value); ok {
							add(s)
						}
					}
					return true
				})
			}
		}
		classes := make([]string, 0, len(seen))
		for class := range seen {
			classes = append(classes, class)
		}
		classCache[pkgPath] = classes
		return classes
	}
	// closure adds a package and each package of the module that it
	// imports, with the imports of those.
	var closure func(into map[string]bool, pkgPath string)
	closure = func(into map[string]bool, pkgPath string) {
		if into[pkgPath] {
			return
		}
		pkg := byPath[pkgPath]
		if pkg == nil {
			return
		}
		into[pkgPath] = true
		for path := range pkg.Imports {
			closure(into, path)
		}
		// An action that a template of the package invokes can patch a
		// component of its own slice. The template imports only the
		// route package of that slice (DR-01), so the slice comes in by
		// the route type of the invocation.
		if dir, ok := moduleDir(module, pkgPath); ok {
			for _, f := range l.load(dir).Files {
				for _, routePkg := range invokedRoutePackages(f) {
					if slice, ok := strings.CutSuffix(routePkg, "/route"); ok {
						closure(into, slice)
					}
				}
			}
		}
	}
	// component returns the generated component that an expression names,
	// or nil.
	component := func(pkg *packages.Package, expr ast.Expr) *types.Func {
		var id *ast.Ident
		switch e := ast.Unparen(expr).(type) {
		case *ast.Ident:
			id = e
		case *ast.SelectorExpr:
			id = e.Sel
		}
		if id == nil {
			return nil
		}
		fn, ok := pkg.TypesInfo.Uses[id].(*types.Func)
		if !ok || fn.Pkg() == nil || !strings.HasSuffix(pkg.Fset.PositionFor(fn.Pos(), false).Filename, "_gx.go") {
			return nil
		}
		return fn
	}
	// tree adds the packages of the component tree of each component
	// that an expression names.
	tree := func(into map[string]bool, pkg *packages.Package, expr ast.Expr) {
		ast.Inspect(expr, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			fn := component(pkg, e)
			if fn == nil {
				return true
			}
			for _, tf := range l.componentTree(module, fn) {
				if rel, err := filepath.Rel(module.Dir, tf.p.Dir); err == nil {
					path := module.Path
					if rel != "." {
						path += "/" + filepath.ToSlash(rel)
					}
					into[path] = true
				}
			}
			return true
		})
	}

	// What each page can show: the layouts, the error views and the toast.
	common := map[string]bool{}
	type page struct {
		pattern string
		pkgs    map[string]bool
	}
	var pages []page
	sourceFiles(res.pkgs, func(pkg *packages.Package, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.CallExpr:
				switch {
				case isGxFuncExpr(pkg, t.Fun, "Layout") && len(t.Args) == 2:
					tree(common, pkg, t.Args[1])
				case isGxFuncExpr(pkg, t.Fun, "Page") && len(t.Args) == 2:
					view := component(pkg, t.Args[1])
					def := res.routeDefs[pageRouteKey(pkg, t)]
					if view == nil || def == nil {
						return true
					}
					reach := map[string]bool{}
					closure(reach, pkg.PkgPath)
					closure(reach, view.Pkg().Path())
					pages = append(pages, page{pattern: def.pattern, pkgs: reach})
				default:
					// app.Errors(notFound, forbidden, serverError).
					if sel, ok := t.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Errors" {
						for _, arg := range t.Args {
							tree(common, pkg, arg)
						}
					}
				}
			case *ast.KeyValueExpr:
				// gx.Config{Toast: toast.Render}: the package of the
				// function renders each toast.
				if key, ok := t.Key.(*ast.Ident); ok && key.Name == "Toast" {
					var id *ast.Ident
					switch e := ast.Unparen(t.Value).(type) {
					case *ast.Ident:
						id = e
					case *ast.SelectorExpr:
						id = e.Sel
					}
					if id != nil {
						if fn, ok := pkg.TypesInfo.Uses[id].(*types.Func); ok && fn.Pkg() != nil {
							closure(common, fn.Pkg().Path())
						}
					}
				}
			}
			return true
		})
	})

	all := map[string]bool{}
	for path := range withSyntax {
		for _, class := range classesOf(path) {
			all[class] = true
		}
	}
	for _, p := range pages {
		seen := map[string]bool{}
		for _, set := range []map[string]bool{p.pkgs, common} {
			for path := range set {
				for _, class := range classesOf(path) {
					seen[class] = true
				}
			}
		}
		if len(seen) >= len(all) {
			// The page can use each class of the app.
			continue
		}
		classes := make([]string, 0, len(seen))
		for class := range seen {
			classes = append(classes, class)
		}
		sort.Strings(classes)
		out[p.pattern] = classes
	}
	return out
}

// routeClassesBytes renders the class lists of the routes as JSON, with the
// patterns in order. An app with no such route gets an empty file.
func routeClassesBytes(lists map[string][]string) []byte {
	if len(lists) == 0 {
		return []byte{}
	}
	b, err := json.MarshalIndent(lists, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(b, '\n')
}

// invokedRoutePackages returns the import path of each route package whose
// type a template of the file gives to an on: directive or to the action of
// a form.
func invokedRoutePackages(f *File) []string {
	imports := map[string]string{}
	for _, imp := range f.Imports {
		fields := strings.Fields(imp.Raw)
		if len(fields) == 0 {
			continue
		}
		path, ok := unquoteGo(fields[len(fields)-1])
		if !ok {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if len(fields) > 1 {
			name = fields[0]
		}
		imports[name] = path
	}
	var out []string
	walkElements(f.Body, func(el *Element) {
		for i := range el.Attrs {
			a := &el.Attrs[i]
			if a.Kind != AttrExpr || !(strings.HasPrefix(a.Name, "on:") || a.Name == "action" || a.Name == "formaction") {
				continue
			}
			// The value starts with the route type: cartroute.Add{...}.
			value := strings.TrimSpace(a.Value)
			if dot := strings.IndexByte(value, '.'); dot > 0 {
				if path, ok := imports[value[:dot]]; ok {
					out = append(out, path)
				}
			}
		}
	})
	return out
}
