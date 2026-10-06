package compiler

import (
	_ "embed"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// AppModel is the machine-readable model of an app (REQ-AI-01). `gx
// describe --json` prints it and the dev MCP server answers with it. The
// CLI adds the icon sets and the registry items from gx.lock.
type AppModel struct {
	Module      string            `json:"module"`
	Components  []ComponentModel  `json:"components"`
	Routes      []RouteModel      `json:"routes"`
	Actions     []ActionModel     `json:"actions"`
	Forms       []FormModel       `json:"forms"`
	Islands     []IslandModel     `json:"islands"`
	Elements    []ElementModel    `json:"elements"`
	Transitions []TransitionModel `json:"transitions"`
	Icons       []IconSetModel    `json:"icons"`
	Registry    []RegistryModel   `json:"registry"`
}

// ComponentModel is one .gx component.
type ComponentModel struct {
	Name      string          `json:"name"`
	Package   string          `json:"package"`
	File      string          `json:"file"`
	Props     []PropModel     `json:"props"`
	Signals   []SignalModel   `json:"signals"`
	Fragments []FragmentModel `json:"fragments"`
	// Fixtures holds the fixture names of the component, in order.
	Fixtures []string `json:"fixtures"`
}

// PropModel is one prop of a component.
type PropModel struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
	Doc      string `json:"doc,omitempty"`
}

// SignalModel is one client signal of a component.
type SignalModel struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Doc     string `json:"doc,omitempty"`
}

// FragmentModel is one #fragment of a component and its generated function.
type FragmentModel struct {
	Name string `json:"name"`
	// Func is the generated function, for example "CartTotal".
	Func string `json:"func"`
	// Params is the parameter list of the generated function.
	Params []ParamModel `json:"params"`
	// Keyed is true when the first parameter is the instance or loop key.
	Keyed bool `json:"keyed"`
}

// ParamModel is one parameter of a generated function.
type ParamModel struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// RouteModel is one route type.
type RouteModel struct {
	// Type is the import path and the type name.
	Type    string       `json:"type"`
	Method  string       `json:"method"`
	Pattern string       `json:"pattern"`
	Fields  []FieldModel `json:"fields"`
	// Kind is "page", "action", "form" or "none" when nothing handles the
	// route.
	Kind string `json:"kind"`
	// Handler is the package-level value that handles the route.
	Handler    string   `json:"handler,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
	Layouts    []string `json:"layouts,omitempty"`
	Middleware []string `json:"middleware,omitempty"`
}

// FieldModel is one input field of a route.
type FieldModel struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Source is "path", "query", "signal" or "form".
	Source string `json:"source"`
	// Key is the name of the field in its source.
	Key     string `json:"key"`
	Default string `json:"default,omitempty"`
}

// ActionModel is one gx.Action.
type ActionModel struct {
	Handler string `json:"handler"`
	Route   string `json:"route"`
	Method  string `json:"method"`
	Pattern string `json:"pattern"`
	// Signals lists the signal names the action reads.
	Signals []string `json:"signals"`
	// External is true when another server answers the action.
	External bool `json:"external"`
}

// FormModel is one gx.Form with the rules of its input.
type FormModel struct {
	Handler string           `json:"handler"`
	Route   string           `json:"route"`
	Method  string           `json:"method"`
	Pattern string           `json:"pattern"`
	Fields  []FormFieldModel `json:"fields"`
}

// FormFieldModel is one form field. Name is the form control name; a nested
// field has a dotted name.
type FormFieldModel struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Rules []string `json:"rules"`
}

// IslandModel is one TypeScript island (REQ-ISL-01). Props are the
// exported fields of its Go props struct.
type IslandModel struct {
	Name    string      `json:"name"`
	Package string      `json:"package"`
	File    string      `json:"file"`
	Props   []PropModel `json:"props"`
}

// ElementModel is one imported web component (REQ-ISL-09): the typed tag
// <package name>.<Name> renders the custom element Tag.
type ElementModel struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	Tag     string `json:"tag"`
	// Module is the import specifier of the module that defines the
	// element.
	Module     string        `json:"module"`
	Attributes []ElementAttr `json:"attributes"`
	Events     []string      `json:"events"`
	Slots      []string      `json:"slots"`
}

// TransitionModel is one gx.Transition value.
type TransitionModel struct {
	// Name is the package-level value.
	Name string `json:"name"`
	// Base is the view-transition base name.
	Base string `json:"base"`
	// Key is the key type.
	Key string `json:"key"`
}

// IconSetModel is one pinned icon set.
type IconSetModel struct {
	Set     string `json:"set"`
	Version string `json:"version"`
}

// RegistryModel is one installed registry item.
type RegistryModel struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

const gxPkgPath = "github.com/alternayte/gx"

// DescribeSchema is the JSON Schema of AppModel. `gx describe --schema`
// prints it.
//
//go:embed describe.schema.json
var DescribeSchema []byte

// Describe returns the app model of the module under root (REQ-AI-01).
func Describe(root string) (*AppModel, []Diagnostic) {
	root = absoluteRoot(root)
	l := newLoader()
	dirs := collectDirs(root)
	// The model of a module with an error would be wrong in silence, so
	// every check of `gx check` runs first.
	var diags []Diagnostic
	for _, dir := range dirs {
		diags = append(diags, l.checkDir(dir)...)
	}
	res, tdiags := l.analyze(root, dirs)
	diags = append(diags, tdiags...)
	diags = append(diags, checkContent(root, res.collections, nil)...)
	if len(diags) > 0 {
		sortDiags(diags)
		return nil, diags
	}
	m := &AppModel{
		Components:  []ComponentModel{},
		Routes:      []RouteModel{},
		Actions:     []ActionModel{},
		Forms:       []FormModel{},
		Islands:     []IslandModel{},
		Elements:    []ElementModel{},
		Transitions: []TransitionModel{},
		Icons:       []IconSetModel{},
		Registry:    []RegistryModel{},
	}
	if mod := findModule(root); mod != nil {
		m.Module = mod.Path
	}
	rel := func(path string) string {
		if r, err := filepath.Rel(root, path); err == nil {
			return filepath.ToSlash(r)
		}
		return filepath.ToSlash(path)
	}

	fixtures := collectFixtures(res.pkgs)
	for _, dir := range dirs {
		p := l.load(dir)
		if p.Module == nil {
			continue
		}
		pkgPath := modulePathOf(p.Module, dir)
		for _, f := range p.Files {
			name := componentName(f)
			if name == "" {
				continue
			}
			comp, ok := p.component(name)
			if !ok {
				continue
			}
			scoped := res.componentScoped(l, p, f, comp, map[*File]bool{})
			m.Components = append(m.Components, describeComponent(name, pkgPath, rel(f.File), f, scoped,
				fixtures[filepath.Clean(f.File)].keys))
		}
	}
	sort.Slice(m.Components, func(i, j int) bool {
		if m.Components[i].Package != m.Components[j].Package {
			return m.Components[i].Package < m.Components[j].Package
		}
		return m.Components[i].Name < m.Components[j].Name
	})
	// The directories are in order and so are the names in each one.
	for _, dir := range dirs {
		p := l.load(dir)
		if p.Module == nil {
			continue
		}
		for _, name := range islandNames(p) {
			isl := p.Islands[name]
			im := IslandModel{Name: isl.Name, Package: modulePathOf(p.Module, dir), File: rel(isl.File), Props: []PropModel{}}
			for _, prop := range isl.Props {
				im.Props = append(im.Props, PropModel{Name: prop.Name, Type: prop.Type, Doc: prop.Doc})
			}
			m.Islands = append(m.Islands, im)
		}
	}

	// The imported web components of each package that a .gx file imports.
	for dir, p := range l.pkgs {
		if p.Module == nil {
			continue
		}
		for _, def := range p.Elements {
			em := ElementModel{Name: def.Name, Package: modulePathOf(p.Module, dir), Tag: def.Tag, Module: def.Module,
				Attributes: def.Attributes, Events: def.Events, Slots: def.Slots}
			if em.Attributes == nil {
				em.Attributes = []ElementAttr{}
			}
			if em.Events == nil {
				em.Events = []string{}
			}
			if em.Slots == nil {
				em.Slots = []string{}
			}
			m.Elements = append(m.Elements, em)
		}
	}
	sort.Slice(m.Elements, func(i, j int) bool {
		if m.Elements[i].Package != m.Elements[j].Package {
			return m.Elements[i].Package < m.Elements[j].Package
		}
		return m.Elements[i].Name < m.Elements[j].Name
	})

	handlers := describeHandlers(res)
	mounts := mountInfo(res)
	for _, d := range res.routes {
		if d.pkg == nil {
			continue
		}
		key := d.pkg.PkgPath + "." + d.name
		mount := mounts[key]
		route := RouteModel{
			Type:       key,
			Method:     methodOf(d.pattern),
			Pattern:    d.pattern,
			Fields:     []FieldModel{},
			Kind:       "none",
			Prefix:     mount.prefix,
			Layouts:    mount.layouts,
			Middleware: mount.middleware,
		}
		for _, f := range d.fields {
			route.Fields = append(route.Fields, describeField(f))
		}
		h, handled := handlers[key]
		if handled {
			route.Kind, route.Handler = h.kind, h.name
		}
		m.Routes = append(m.Routes, route)
		switch h.kind {
		case "action":
			action := ActionModel{Handler: h.name, Route: key, Method: route.Method, Pattern: d.pattern, Signals: []string{}, External: h.external}
			for _, f := range d.fields {
				if f.signal != "" {
					action.Signals = append(action.Signals, f.signal)
				}
			}
			m.Actions = append(m.Actions, action)
		case "form":
			form := FormModel{Handler: h.name, Route: key, Method: route.Method, Pattern: d.pattern, Fields: []FormFieldModel{}}
			rules := describeRules(d)
			describeFormFields(&form.Fields, d.fields, "", "", rules)
			m.Forms = append(m.Forms, form)
		}
	}
	sort.SliceStable(m.Routes, func(i, j int) bool { return m.Routes[i].Type < m.Routes[j].Type })
	sort.SliceStable(m.Actions, func(i, j int) bool { return m.Actions[i].Route < m.Actions[j].Route })
	sort.SliceStable(m.Forms, func(i, j int) bool { return m.Forms[i].Route < m.Forms[j].Route })

	m.Transitions = append(m.Transitions, describeTransitions(res)...)
	sort.SliceStable(m.Transitions, func(i, j int) bool { return m.Transitions[i].Name < m.Transitions[j].Name })
	return m, nil
}

// describeComponent builds the model of one component file.
func describeComponent(name, pkgPath, file string, f *File, scoped bool, fixtureKeys []string) ComponentModel {
	c := ComponentModel{
		Name:      name,
		Package:   pkgPath,
		File:      file,
		Props:     []PropModel{},
		Signals:   []SignalModel{},
		Fragments: []FragmentModel{},
		Fixtures:  append([]string{}, fixtureKeys...),
	}
	for _, p := range f.Props {
		c.Props = append(c.Props, PropModel{Name: p.Name, Type: p.Type, Required: !p.HasDefault, Default: p.Default, Doc: p.Doc})
	}
	for _, s := range f.Signals {
		c.Signals = append(c.Signals, SignalModel{Name: s.Name, Type: s.Type, Default: s.Default, Doc: s.Doc})
	}
	for _, el := range fragmentElements(f.Body) {
		var frag *Attr
		for i := range el.Attrs {
			if el.Attrs[i].Kind == AttrFragment {
				frag = &el.Attrs[i]
				break
			}
		}
		if frag == nil {
			continue
		}
		// The parameter order is the order fragmentFunc writes.
		declared := fragmentParams(el)
		model := FragmentModel{Name: frag.Name, Func: name + upperFirst(frag.Name), Params: []ParamModel{}}
		if scoped && !declaresKey(declared) {
			model.Params = append(model.Params, ParamModel{Name: "key", Type: "gx.Key"})
			model.Keyed = true
		} else if declaresKey(declared) {
			model.Keyed = true
		}
		if usesP(el) {
			model.Params = append(model.Params, ParamModel{Name: "p", Type: name + "Props"})
		}
		for _, param := range splitParams(declared) {
			pname, ptype, _ := strings.Cut(param, " ")
			model.Params = append(model.Params, ParamModel{Name: pname, Type: strings.TrimSpace(ptype)})
		}
		c.Fragments = append(c.Fragments, model)
	}
	return c
}

// describeField builds the model of one route input field.
func describeField(f routeField) FieldModel {
	out := FieldModel{Name: f.name, Type: f.typeText, Default: f.def}
	switch {
	case f.path != "":
		out.Source, out.Key = "path", f.path
	case f.query != "":
		out.Source, out.Key = "query", f.query
	case f.signal != "":
		out.Source, out.Key = "signal", f.signal
	default:
		out.Source, out.Key = "form", f.bind
	}
	return out
}

// describeFormFields flattens the fields of a form input. A nested struct
// gives dotted names and a slice gives "name[]" (REQ-FRM-08).
func describeFormFields(out *[]FormFieldModel, fields []routeField, namePrefix, goPrefix string, rules map[string][]string) {
	for _, f := range fields {
		if f.path != "" || f.query != "" || f.signal != "" {
			continue
		}
		name := namePrefix + f.bind
		goPath := joinPath(goPrefix, f.name)
		if f.slice && len(f.sub) > 0 && len(f.sub[0].sub) > 0 {
			describeFormFields(out, f.sub[0].sub, name+"[].", goPath, rules)
			continue
		}
		if !f.slice && len(f.sub) > 0 {
			describeFormFields(out, f.sub, name+".", goPath, rules)
			continue
		}
		if f.slice {
			name += "[]"
		}
		fieldRules := rules[goPath]
		if fieldRules == nil {
			fieldRules = []string{}
		}
		*out = append(*out, FormFieldModel{Name: name, Type: f.typeText, Rules: fieldRules})
	}
}

// describeRules reads the Rules method of a form input: the dotted Go field
// path of each gx.Field call and the source text of its rules.
func describeRules(d *routeDef) map[string][]string {
	out := map[string][]string{}
	for _, file := range d.pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "Rules" || fn.Body == nil || len(fn.Recv.List) != 1 {
				continue
			}
			recv := fn.Recv.List[0].Type
			if star, ok := recv.(*ast.StarExpr); ok {
				recv = star.X
			}
			if id, ok := recv.(*ast.Ident); !ok || id.Name != d.name {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isGxFunc(d.pkg, call.Fun, "Field") || len(call.Args) < 2 {
					return true
				}
				name := fieldArgName(call.Args[0])
				if name == "" {
					return true
				}
				for _, arg := range call.Args[1:] {
					out[name] = append(out[name], types.ExprString(arg))
				}
				return true
			})
		}
	}
	return out
}

// describedHandler is the package-level value that handles one route.
type describedHandler struct {
	kind     string
	name     string
	external bool
}

// describeHandlers maps each route type to its gx.Page, gx.Action or
// gx.Form value. It reads the type of the value, so a chained call such as
// .Static or .External does not hide it.
func describeHandlers(res *typesResult) map[string]describedHandler {
	out := map[string]describedHandler{}
	eachPackageVar(res, func(pkg *packages.Package, name *ast.Ident, value ast.Expr) {
		obj := pkg.TypesInfo.Defs[name]
		if obj == nil {
			return
		}
		kind, key, ok := handlerRoute(obj)
		if !ok {
			return
		}
		if prev, ok := out[key]; ok && prev.kind != "page" {
			// A form route has a page that shows it; the form wins.
			return
		}
		out[key] = describedHandler{kind: kind, name: pkg.Name + "." + name.Name, external: callsMethod(value, "External")}
	})
	return out
}

// handlerRoute reports the kind ("page", "action" or "form") and the route
// type of a gx.Page, gx.Action or gx.Form value.
func handlerRoute(obj types.Object) (kind, key string, ok bool) {
	ptr, isPtr := obj.Type().(*types.Pointer)
	if !isPtr {
		return "", "", false
	}
	named, isNamed := ptr.Elem().(*types.Named)
	if !isNamed || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != gxPkgPath || named.TypeArgs().Len() == 0 {
		return "", "", false
	}
	kind = named.Obj().Name()
	if kind != "page" && kind != "action" && kind != "form" {
		return "", "", false
	}
	in := named.TypeArgs().At(0)
	if p, isPtr := in.(*types.Pointer); isPtr {
		in = p.Elem()
	}
	route, isNamed := in.(*types.Named)
	if !isNamed || route.Obj().Pkg() == nil {
		return "", "", false
	}
	return kind, route.Obj().Pkg().Path() + "." + route.Obj().Name(), true
}

// callsMethod reports whether a call chain calls the method name.
func callsMethod(expr ast.Expr, name string) bool {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if sel.Sel.Name == name {
			return true
		}
		expr = sel.X
	}
}

// describeTransitions finds every package-level gx.Transition value.
func describeTransitions(res *typesResult) []TransitionModel {
	var out []TransitionModel
	eachPackageVar(res, func(pkg *packages.Package, name *ast.Ident, value ast.Expr) {
		call, ok := value.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return
		}
		fun := call.Fun
		switch x := fun.(type) {
		case *ast.IndexExpr:
			fun = x.X
		case *ast.IndexListExpr:
			fun = x.X
		}
		if !isGxFunc(pkg, fun, "Transition") {
			return
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		base, err := strconv.Unquote(lit.Value)
		if err != nil {
			return
		}
		key := ""
		if obj := pkg.TypesInfo.Defs[name]; obj != nil {
			if sig, ok := obj.Type().(*types.Signature); ok && sig.Params().Len() == 1 {
				key = types.TypeString(sig.Params().At(0).Type(), func(p *types.Package) string { return p.Name() })
			}
		}
		out = append(out, TransitionModel{Name: pkg.Name + "." + name.Name, Base: base, Key: key})
	})
	return out
}

// eachPackageVar calls fn for every package-level variable with a value in
// the hand-written Go files of the module.
func eachPackageVar(res *typesResult, fn func(pkg *packages.Package, name *ast.Ident, value ast.Expr)) {
	for _, pkg := range res.pkgs {
		for _, file := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_gx.go") {
				continue
			}
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
					for i, value := range vs.Values {
						if i < len(vs.Names) {
							fn(pkg, vs.Names[i], value)
						}
					}
				}
			}
		}
	}
}
