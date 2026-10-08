package compiler

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/alternayte/gx/internal/elementname"
)

// widgetDecl is one gx.Widget call of the module.
type widgetDecl struct {
	pkg *packages.Package
	// at is the position of the gx.Widget call.
	at token.Position
	// varKey names the package variable that holds the widget, or "".
	varKey string
	tag    string
	// tagAt is the position of the tag text, when the call has a Tag.
	tagAt  token.Position
	hasTag bool
	// input is the route type of the widget.
	input *types.Named
	// view is the component function of the widget.
	view types.Object
}

// label names the widget in a message.
func (w *widgetDecl) label() string {
	if w.tag != "" {
		return w.tag
	}
	return "at " + filepath.Base(w.at.Filename)
}

// routeMount is one place where a Group call mounts a handler variable.
type routeMount struct {
	// origins is true when gx.AllowOrigins or gx.AllowCredentials comes
	// before the handler in the Group call.
	origins bool
}

// checkWidgets reports the widget diagnostics of the module: a tag that is
// missing, not valid or used two times (GX6003), an input field that an
// attribute cannot hold (GX6006), and a widget or an action of its
// component in a group with no gx.AllowOrigins (GX6008).
func (l *loader) checkWidgets(res *typesResult, pkgs []*packages.Package, root string) []Diagnostic {
	widgets := collectWidgets(pkgs)
	if len(widgets) == 0 {
		return nil
	}
	var out []Diagnostic
	diag := func(code string, at token.Position, msg string) {
		out = append(out, Diagnostic{Code: code, File: at.Filename, Line: at.Line, Col: at.Column, Msg: msg})
	}

	byTag := map[string]*widgetDecl{}
	for _, w := range widgets {
		switch {
		case !w.hasTag:
			diag(CodeWidgetTag, w.at, "widget has no tag; add .Tag(\"acme-name\") to gx.Widget")
		case w.tag == "":
			// The tag is not a constant: gx.Widget checks it at startup.
		default:
			if problem := elementname.Problem(w.tag); problem != "" {
				diag(CodeWidgetTag, w.tagAt, "widget tag "+Quoted(w.tag)+" is not a custom element name: "+problem)
			} else if first := byTag[w.tag]; first != nil {
				diag(CodeWidgetTag, w.tagAt, "widget tag "+Quoted(w.tag)+" is the tag of a different widget ("+filepath.Base(first.at.Filename)+")")
			} else {
				byTag[w.tag] = w
			}
		}
		out = append(out, widgetAttrDiags(w)...)
	}

	mounts := collectMounts(pkgs)
	actionVars := collectActionVars(pkgs)
	module := findModule(root)
	for _, w := range widgets {
		if w.varKey != "" && lacksOrigins(mounts[w.varKey]) {
			diag(CodeWidgetOrigins, w.at, "widget "+w.label()+" is in a group with no gx.AllowOrigins; a page of a different origin cannot load it")
		}
		seen := map[string]bool{}
		for _, key := range l.invokedRoutes(res, module, w.view) {
			for _, av := range actionVars[key] {
				if seen[av.varKey] || !lacksOrigins(mounts[av.varKey]) {
					continue
				}
				seen[av.varKey] = true
				diag(CodeWidgetOrigins, av.at, "action "+Quoted(av.name)+" is in a group with no gx.AllowOrigins, and the widget "+w.label()+" invokes it from a different origin")
			}
		}
	}
	return out
}

// lacksOrigins reports whether one mount or more of a handler has no
// origins. A handler that no Group call names has no finding here: the
// analysis cannot see its group.
func lacksOrigins(ms []routeMount) bool {
	for _, m := range ms {
		if !m.origins {
			return true
		}
	}
	return false
}

// widgetAttrDiags reports each field of the widget input that an attribute
// cannot hold (GX6006). An attribute is text, so a field is a string, a
// number, a bool or a named type of one of these.
func widgetAttrDiags(w *widgetDecl) []Diagnostic {
	if w.input == nil {
		return nil
	}
	st, ok := w.input.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	var out []Diagnostic
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Embedded() || !f.Exported() {
			continue
		}
		if _, ok := bindKind(f.Type()); ok {
			continue
		}
		at := w.pkg.Fset.Position(f.Pos())
		out = append(out, Diagnostic{
			Code: CodeWidgetAttr, File: at.Filename, Line: at.Line, Col: at.Column,
			Msg: "field " + Quoted(f.Name()) + " of the widget input " + Quoted(w.input.Obj().Name()) + " has the type " + types.TypeString(f.Type(), typeQualifier(w.pkg)) +
				"; an attribute holds a string, a number or a bool",
		})
	}
	return out
}

// widgetCall returns the gx.Widget call at the root of a call chain such as
// gx.Widget(load, view).Tag("acme-cart"), and the Tag call of the chain.
func widgetCall(pkg *packages.Package, expr ast.Expr) (widget, tag *ast.CallExpr) {
	for {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok {
			return nil, nil
		}
		if isGxFuncExpr(pkg, call.Fun, "Widget") {
			return call, tag
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, nil
		}
		if sel.Sel.Name == "Tag" && tag == nil {
			tag = call
		}
		expr = sel.X
	}
}

// packageVarKey names a package-level variable, or returns "".
func packageVarKey(obj types.Object) string {
	v, ok := obj.(*types.Var)
	if !ok || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return ""
	}
	return v.Pkg().Path() + "." + v.Name()
}

// sourceFiles calls fn for each hand-written Go file of the packages.
func sourceFiles(pkgs []*packages.Package, fn func(pkg *packages.Package, file *ast.File)) {
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			fn(pkg, file)
		}
	}
}

// collectWidgets finds each gx.Widget call of the module, in a stable order.
func collectWidgets(pkgs []*packages.Package) []*widgetDecl {
	var out []*widgetDecl
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
		// The variable of a widget, by the outer call of its chain.
		names := map[*ast.CallExpr]string{}
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
					if i >= len(vs.Names) {
						break
					}
					if call, _ := widgetCall(pkg, val); call != nil {
						if obj := pkg.TypesInfo.Defs[vs.Names[i]]; obj != nil {
							names[call] = packageVarKey(obj)
						}
					}
				}
			}
		}
		tags := map[*ast.CallExpr]*ast.CallExpr{}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if wc, tag := widgetCall(pkg, call); wc != nil && tag != nil {
				tags[wc] = tag
			}
			return true
		})
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isGxFuncExpr(pkg, call.Fun, "Widget") || len(call.Args) != 2 {
				return true
			}
			w := &widgetDecl{pkg: pkg, at: pkg.Fset.Position(call.Pos()), varKey: names[call]}
			if tag := tags[call]; tag != nil && len(tag.Args) == 1 {
				w.hasTag = true
				w.tagAt = pkg.Fset.Position(tag.Args[0].Pos())
				if tv, ok := pkg.TypesInfo.Types[tag.Args[0]]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
					w.tag = constant.StringVal(tv.Value)
					if w.tag == "" {
						// An empty constant is a tag that is not valid.
						w.tag = " "
					}
				}
			}
			if t := pkg.TypesInfo.TypeOf(call.Args[0]); t != nil {
				if sig, ok := t.Underlying().(*types.Signature); ok && sig.Params().Len() == 2 {
					w.input, _ = sig.Params().At(1).Type().(*types.Named)
				}
			}
			switch v := ast.Unparen(call.Args[1]).(type) {
			case *ast.Ident:
				w.view = pkg.TypesInfo.Uses[v]
			case *ast.SelectorExpr:
				w.view = pkg.TypesInfo.Uses[v.Sel]
			}
			out = append(out, w)
			return true
		})
	})
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].at, out[j].at
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		return a.Offset < b.Offset
	})
	return out
}

// collectMounts reads each Group call of the module. It returns, for each
// handler variable that a Group call names directly or through a
// gx.Collect variable, the mounts of the handler.
func collectMounts(pkgs []*packages.Package) map[string][]routeMount {
	// collections maps a gx.Collect variable to the handler variables
	// that it holds.
	collections := map[string][]string{}
	var members func(pkg *packages.Package, expr ast.Expr) []string
	members = func(pkg *packages.Package, expr ast.Expr) []string {
		switch e := ast.Unparen(expr).(type) {
		case *ast.CallExpr:
			if !isGxFunc(pkg, e.Fun, "Collect") {
				return nil
			}
			var out []string
			for _, arg := range e.Args {
				out = append(out, members(pkg, arg)...)
			}
			return out
		case *ast.Ident:
			if key := packageVarKey(pkg.TypesInfo.Uses[e]); key != "" {
				return []string{key}
			}
		case *ast.SelectorExpr:
			if key := packageVarKey(pkg.TypesInfo.Uses[e.Sel]); key != "" {
				return []string{key}
			}
		}
		return nil
	}
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
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
					call, ok := ast.Unparen(val).(*ast.CallExpr)
					if !ok || i >= len(vs.Names) || !isGxFunc(pkg, call.Fun, "Collect") {
						continue
					}
					if key := packageVarKey(pkg.TypesInfo.Defs[vs.Names[i]]); key != "" {
						collections[key] = members(pkg, call)
					}
				}
			}
		}
	})
	// expand follows a variable to the handlers that it holds.
	var expand func(key string, depth int) []string
	expand = func(key string, depth int) []string {
		held, ok := collections[key]
		if !ok || depth > 8 {
			return []string{key}
		}
		var out []string
		for _, k := range held {
			out = append(out, expand(k, depth+1)...)
		}
		return out
	}

	mounts := map[string][]routeMount{}
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isGxFunc(pkg, call.Fun, "Group") || len(call.Args) == 0 {
				return true
			}
			origins := false
			for _, arg := range call.Args[1:] {
				if c, ok := ast.Unparen(arg).(*ast.CallExpr); ok && (isGxFunc(pkg, c.Fun, "AllowOrigins") || isGxFunc(pkg, c.Fun, "AllowCredentials")) {
					origins = true
					continue
				}
				for _, key := range members(pkg, arg) {
					for _, handler := range expand(key, 0) {
						mounts[handler] = append(mounts[handler], routeMount{origins: origins})
					}
				}
			}
			return true
		})
	})
	return mounts
}

// actionVar is one package variable that holds a gx.Action.
type actionVar struct {
	varKey string
	name   string
	at     token.Position
}

// collectActionVars maps the route type of each gx.Action that a package
// variable holds to that variable.
func collectActionVars(pkgs []*packages.Package) map[string][]actionVar {
	out := map[string][]actionVar{}
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
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
					if i >= len(vs.Names) {
						break
					}
					call := actionCall(pkg, val)
					if call == nil || len(call.Args) == 0 {
						continue
					}
					t := pkg.TypesInfo.TypeOf(call.Args[0])
					if t == nil {
						continue
					}
					sig, ok := t.Underlying().(*types.Signature)
					if !ok || sig.Params().Len() != 2 {
						continue
					}
					route := namedTypeKey(sig.Params().At(1).Type())
					key := packageVarKey(pkg.TypesInfo.Defs[vs.Names[i]])
					if route == "" || key == "" {
						continue
					}
					out[route] = append(out[route], actionVar{varKey: key, name: vs.Names[i].Name, at: pkg.Fset.Position(vs.Names[i].Pos())})
				}
			}
		}
	})
	return out
}

// actionCall returns the gx.Action call at the root of a call chain such as
// gx.Action(fn).Tool().
func actionCall(pkg *packages.Package, expr ast.Expr) *ast.CallExpr {
	for {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok {
			return nil
		}
		if isGxFuncExpr(pkg, call.Fun, "Action") {
			return call
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		expr = sel.X
	}
}

// treeFile is one .gx file of a component tree, with its package.
type treeFile struct {
	p *Package
	f *File
}

// componentTree returns the .gx file of a component function and the file
// of each component that it uses, in the module.
func (l *loader) componentTree(module *Module, view types.Object) []treeFile {
	if view == nil || view.Pkg() == nil || module == nil {
		return nil
	}
	dir, ok := moduleDir(module, view.Pkg().Path())
	if !ok {
		return nil
	}
	p := l.load(dir)
	f := p.Files[view.Name()]
	if f == nil {
		return nil
	}
	var out []treeFile
	seen := map[*File]bool{}
	var visit func(p *Package, f *File)
	visit = func(p *Package, f *File) {
		if seen[f] {
			return
		}
		seen[f] = true
		out = append(out, treeFile{p: p, f: f})
		walkElements(f.Body, func(el *Element) {
			qual, name, ok := componentTag(el.Name)
			if !ok {
				return
			}
			if qual == "" {
				if child := p.Files[name]; child != nil {
					visit(p, child)
				}
				return
			}
			if q, ok := l.importedPackage(p, f, qual); ok {
				if child := q.Files[name]; child != nil {
					visit(q, child)
				}
			}
		})
	}
	visit(p, f)
	return out
}

// invokedRoutes returns the route types that the component of a widget
// invokes with on:, in the component and in each component that it uses.
func (l *loader) invokedRoutes(res *typesResult, module *Module, view types.Object) []string {
	var out []string
	routes := map[string]bool{}
	for _, tf := range l.componentTree(module, view) {
		walkElements(tf.f.Body, func(el *Element) {
			for i := range el.Attrs {
				a := &el.Attrs[i]
				if a.Kind != AttrExpr || !strings.HasPrefix(a.Name, "on:") {
					continue
				}
				if key := namedTypeKey(res.types[a]); key != "" && res.routeKeys[key] && !routes[key] {
					routes[key] = true
					out = append(out, key)
				}
			}
		})
	}
	return out
}

// widgetClassesPath is the file with the class list of each widget. The
// stylesheet build makes one stylesheet for each list (REQ-ISL-11).
const widgetClassesPath = ".gx/widget-classes.json"

// widgetClasses returns, for the tag of each widget, the classes that the
// widget uses: the classes of its component tree and the string literals of
// the Go files in the packages of the tree. The rule is the rule of the app
// class list (REQ-STY-02), for the packages of one widget.
func (l *loader) widgetClasses(res *typesResult, root string) map[string][]string {
	out := map[string][]string{}
	if res == nil {
		return out
	}
	module := findModule(root)
	for _, w := range collectWidgets(res.pkgs) {
		if w.tag == "" || elementname.Problem(w.tag) != "" {
			continue
		}
		seen := map[string]bool{}
		add := func(text string) {
			for _, class := range strings.Fields(text) {
				seen[class] = true
			}
		}
		dirs := map[string]bool{}
		for _, tf := range l.componentTree(module, w.view) {
			collectFileClasses(tf.f.Body, add)
			dirs[filepath.Clean(tf.p.Dir)] = true
		}
		for _, pkg := range res.pkgs {
			for _, file := range pkg.Syntax {
				path := pkg.Fset.Position(file.Pos()).Filename
				if !dirs[filepath.Clean(filepath.Dir(path))] || strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
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
		sort.Strings(classes)
		out[w.tag] = classes
	}
	return out
}

// widgetClassesBytes renders the class lists of the widgets as JSON, with
// the tags in order. An app with no widget gets an empty file, as it gets an
// empty class list: the file of an earlier widget must not stay.
func widgetClassesBytes(lists map[string][]string) []byte {
	if len(lists) == 0 {
		return []byte{}
	}
	b, err := json.MarshalIndent(lists, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(b, '\n')
}

// moduleDir returns the directory of a package of the module.
func moduleDir(m *Module, pkgPath string) (string, bool) {
	switch {
	case pkgPath == m.Path:
		return m.Dir, true
	case strings.HasPrefix(pkgPath, m.Path+"/"):
		return filepath.Join(m.Dir, filepath.FromSlash(strings.TrimPrefix(pkgPath, m.Path+"/"))), true
	}
	return "", false
}

// checkEventSecrets reports a gx.Event whose detail type holds a gx.Secret
// (GX7002, SI-04). The detail of a domain event goes to the browser as
// JSON (REQ-ISL-17).
func checkEventSecrets(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isGxFuncExpr(pkg, call.Fun, "Event") {
				return true
			}
			index, ok := call.Fun.(*ast.IndexExpr)
			if !ok {
				return true
			}
			detail := pkg.TypesInfo.TypeOf(index.Index)
			path := secretPath(detail, map[types.Type]bool{})
			if path == nil {
				return true
			}
			name := "event"
			if len(call.Args) == 1 {
				if tv, ok := pkg.TypesInfo.Types[call.Args[0]]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
					name = "event " + constant.StringVal(tv.Value)
				}
			}
			at := pkg.Fset.Position(call.Pos())
			where := "its detail"
			if len(path) > 0 {
				where = "the detail field " + Quoted(strings.Join(path, "."))
			}
			out = append(out, Diagnostic{
				Code: CodeSecret, File: at.Filename, Line: at.Line, Col: at.Column,
				Msg: name + ": " + where + " has type gx.Secret; a secret cannot cross to the client",
			})
			return true
		})
	})
	return out
}

// secretPath returns the field path to a gx.Secret inside a type, or nil
// when the type holds none. A type that is a secret itself has an empty
// path.
func secretPath(t types.Type, seen map[types.Type]bool) []string {
	if t == nil || seen[t] {
		return nil
	}
	seen[t] = true
	if isSecretType(t) {
		return []string{}
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		return secretPath(u.Elem(), seen)
	case *types.Slice:
		return secretPath(u.Elem(), seen)
	case *types.Array:
		return secretPath(u.Elem(), seen)
	case *types.Map:
		return secretPath(u.Elem(), seen)
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if path := secretPath(u.Field(i).Type(), seen); path != nil {
				return append([]string{u.Field(i).Name()}, path...)
			}
		}
	}
	return nil
}
