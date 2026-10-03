package compiler

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// routeDef is one route struct found in a Go file (REQ-RTE-01).
type routeDef struct {
	pkg        *packages.Package
	file       string
	name       string
	pattern    string
	fields     []routeField
	pos        token.Position
	action     bool
	hasSignals bool
	// form marks a route struct with Rules(), the input of a gx.Form
	// (REQ-FRM-01).
	form bool
	// formRules holds the native constraints of every field, parsed from
	// the Rules method body (REQ-FRM-04).
	formRules map[string][]formConstraint
}

// formConstraint is one native constraint a rule maps to (REQ-FRM-04).
type formConstraint struct {
	key    string
	value  string
	isBool bool
}

// routeField is one bindable field of a route struct. A form-bound field
// holds a tree: struct fields nest and slices hold one element.
type routeField struct {
	name     string // Go field name
	bind     string // form name segment
	typeText string
	typ      types.Type
	kind     types.BasicKind // basic leaves only
	path     string
	query    string
	signal   string
	form     string
	def      string
	pos      token.Position
	sub      []routeField // struct fields, or the one slice element
	slice    bool
	arrayLen int64 // 0 for a slice
}

var pathVarRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(\.\.\.)?\}`)

// collectRoutes finds every route struct in the loaded packages and checks
// its pattern against its fields (REQ-RTE-02). An action input type also
// binds untagged fields from form fields (REQ-ACT-03).
func collectRoutes(pkgs []*packages.Package, actions map[string][]token.Position) ([]*routeDef, []Diagnostic) {
	var defs []*routeDef
	var diags []Diagnostic
	for _, pkg := range pkgs {
		rulesMethods := map[string]*ast.FuncDecl{}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "Rules" || fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				if name := receiverTypeName(fn.Recv.List[0].Type); name != "" {
					rulesMethods[name] = fn
				}
			}
		}
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
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
					pattern, ok := routePattern(pkg, st)
					if !ok {
						continue
					}
					obj, _ := pkg.TypesInfo.Defs[ts.Name].(*types.TypeName)
					if obj == nil {
						continue
					}
					named, ok := obj.Type().(*types.Named)
					if !ok {
						continue
					}
					stype, ok := named.Underlying().(*types.Struct)
					if !ok {
						continue
					}
					def := &routeDef{
						pkg:     pkg,
						file:    path,
						name:    ts.Name.Name,
						pattern: pattern,
						pos:     pkg.Fset.Position(ts.Pos()),
					}
					def.action = len(actions[def.pkg.PkgPath+"."+def.name]) > 0
					def.form = hasFormRules(named)
					if def.form {
						def.formRules = parseFormRules(pkg, rulesMethods[def.name])
					}
					diags = append(diags, routeFields(def, stype)...)
					defs = append(defs, def)
				}
			}
		}
	}
	return defs, diags
}

// receiverTypeName returns the type name of a method receiver expression.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	}
	return ""
}

// hasFormRules reports whether the type has a Rules() gx.Rules method
// (REQ-FRM-01).
func hasFormRules(named *types.Named) bool {
	sel := types.NewMethodSet(types.NewPointer(named)).Lookup(nil, "Rules")
	if sel == nil {
		return false
	}
	sig, ok := sel.Obj().Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	return sig.Results().At(0).Type().String() == "github.com/alternayte/gx.Rules"
}

// parseFormRules reads the Rules method body and maps every Go field name to
// the native constraints of its rules (REQ-FRM-04).
func parseFormRules(pkg *packages.Package, fn *ast.FuncDecl) map[string][]formConstraint {
	out := map[string][]formConstraint{}
	if fn == nil || fn.Body == nil {
		return out
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isGxFunc(pkg, call.Fun, "Field") || len(call.Args) < 2 {
			return true
		}
		name := fieldArgName(call.Args[0])
		if name == "" {
			return true
		}
		for _, arg := range call.Args[1:] {
			out[name] = append(out[name], ruleConstraints(pkg, arg)...)
		}
		return true
	})
	return out
}

// fieldArgName returns the dotted Go field path of a gx.Field call argument,
// for example "Address.Street". A slice element gives "" (REQ-FRM-08).
func fieldArgName(expr ast.Expr) string {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return ""
	}
	var parts []string
	for {
		switch x := unary.X.(type) {
		case *ast.SelectorExpr:
			parts = append(parts, x.Sel.Name)
			unary = &ast.UnaryExpr{X: x.X}
		case *ast.Ident:
			// The receiver name is not part of the path.
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return strings.Join(parts, ".")
		default:
			return ""
		}
	}
}

// ruleConstraints maps one rule value to its native constraints. Rules
// without a native form give none (REQ-FRM-04).
func ruleConstraints(pkg *packages.Package, expr ast.Expr) []formConstraint {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		switch ruleName(pkg, t) {
		case "Required":
			return []formConstraint{{key: "required", isBool: true}}
		case "Email":
			return []formConstraint{{key: "type", value: "email"}}
		case "IsURL":
			return []formConstraint{{key: "type", value: "url"}}
		}
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		switch ruleName(pkg, sel) {
		case "MinLen":
			return intConstraint(pkg, t, "minlength")
		case "MaxLen":
			return intConstraint(pkg, t, "maxlength")
		case "Min":
			return intConstraint(pkg, t, "min")
		case "Max":
			return intConstraint(pkg, t, "max")
		case "True":
			return []formConstraint{{key: "required", isBool: true}}
		case "Pattern":
			if re := patternLiteral(pkg, t); re != "" {
				return []formConstraint{{key: "pattern", value: re}}
			}
		}
	}
	return nil
}

// ruleName returns the gx name of a rule expression, or "".
func ruleName(pkg *packages.Package, sel *ast.SelectorExpr) string {
	obj := pkg.TypesInfo.Uses[sel.Sel]
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "github.com/alternayte/gx" {
		return ""
	}
	return obj.Name()
}

// intConstraint returns the numeric native constraint of a rule call.
func intConstraint(pkg *packages.Package, call *ast.CallExpr, key string) []formConstraint {
	if len(call.Args) != 1 {
		return nil
	}
	tv, ok := pkg.TypesInfo.Types[call.Args[0]]
	if !ok || tv.Value == nil {
		return nil
	}
	return []formConstraint{{key: key, value: tv.Value.ExactString()}}
}

// patternLiteral returns the source of a constant regexp rule value, or "".
func patternLiteral(pkg *packages.Package, call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return ""
	}
	return regexpSource(pkg, call.Args[0])
}

// regexpSource returns the pattern of a regexp expression.
func regexpSource(pkg *packages.Package, expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.BasicLit:
		if t.Kind != token.STRING {
			return ""
		}
		s, err := strconv.Unquote(t.Value)
		if err != nil {
			return ""
		}
		return s
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "MustCompile" {
			return ""
		}
		if len(t.Args) != 1 {
			return ""
		}
		return regexpSource(pkg, t.Args[0])
	case *ast.Ident:
		// A package-level var holds the compiled regexp.
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Values) != len(vs.Names) {
						continue
					}
					for i, id := range vs.Names {
						if id.Name == t.Name {
							return regexpSource(pkg, vs.Values[i])
						}
					}
				}
			}
		}
	}
	return ""
}

// routePattern returns the pattern of an embedded gx.Route field.
func routePattern(pkg *packages.Package, st *ast.StructType) (string, bool) {
	for _, f := range st.Fields.List {
		if len(f.Names) != 0 || f.Tag == nil {
			continue
		}
		sel, ok := f.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		obj := pkg.TypesInfo.Uses[sel.Sel]
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "github.com/alternayte/gx" || obj.Name() != "Route" {
			continue
		}
		raw, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		pattern := strings.TrimSpace(raw)
		method, _, ok := strings.Cut(pattern, " ")
		if !ok {
			continue
		}
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			return pattern, true
		}
		continue
	}
	return "", false
}

// routeFields fills def.fields and reports pattern mismatches.
func routeFields(def *routeDef, stype *types.Struct) []Diagnostic {
	var diags []Diagnostic
	vars := map[string]string{} // lowercase name -> pattern spelling
	for _, m := range pathVarRe.FindAllStringSubmatch(def.pattern, -1) {
		vars[strings.ToLower(m[1])] = m[1]
	}
	bound := map[string]bool{}
	for i := 0; i < stype.NumFields(); i++ {
		f := stype.Field(i)
		if f.Embedded() || !f.Exported() {
			continue
		}
		tag := reflect.StructTag(stype.Tag(i))
		if tag.Get("bind") == "-" {
			continue
		}
		pos := def.pkg.Fset.Position(f.Pos())
		pathName := tag.Get("path")
		if pathName == "" {
			pathName = vars[strings.ToLower(f.Name())]
		}
		queryName := tag.Get("query")
		signalName := tag.Get("signal")
		formName := tag.Get("form")
		isPath := pathName != "" && vars[strings.ToLower(pathName)] != ""
		if tag.Get("path") != "" && !isPath {
			diags = append(diags, Diagnostic{
				Code: CodePathField,
				File: def.file,
				Line: pos.Line,
				Col:  pos.Column,
				Msg:  "field " + Quoted(f.Name()) + " binds path " + Quoted(tag.Get("path")) + " which the pattern has no variable for",
			})
		}
		if !isPath && queryName == "" && signalName == "" && !def.action && !def.form {
			continue // not a request-bound field
		}
		if isPath || queryName != "" || signalName != "" {
			kind, ok := bindKind(f.Type())
			if !ok {
				diags = append(diags, Diagnostic{
					Code: CodeRouteField,
					File: def.file,
					Line: pos.Line,
					Col:  pos.Column,
					Msg:  "field " + Quoted(f.Name()) + " has type " + Quoted(types.TypeString(f.Type(), nil)) + " which cannot bind",
				})
				continue
			}
			field := routeField{
				name:     f.Name(),
				typeText: types.TypeString(f.Type(), typeQualifier(def.pkg)),
				typ:      f.Type(),
				kind:     kind,
				query:    queryName,
				signal:   signalName,
				def:      tag.Get("default"),
				pos:      pos,
			}
			if isPath {
				field.path = pathName
				bound[strings.ToLower(pathName)] = true
			}
			if signalName != "" {
				def.hasSignals = true
			}
			def.fields = append(def.fields, field)
			continue
		}
		bind := formName
		if bind == "" {
			bind = lowerFirst(f.Name())
		}
		field, ok := bindFormField(def, f.Name(), f.Type(), bind, pos)
		if !ok {
			diags = append(diags, Diagnostic{
				Code: CodeRouteField,
				File: def.file,
				Line: pos.Line,
				Col:  pos.Column,
				Msg:  "field " + Quoted(f.Name()) + " has type " + Quoted(types.TypeString(f.Type(), nil)) + " which cannot bind",
			})
			continue
		}
		def.fields = append(def.fields, field)
	}
	for name := range vars {
		if !bound[name] {
			diags = append(diags, Diagnostic{
				Code: CodePathVar,
				File: def.file,
				Line: def.pos.Line,
				Col:  def.pos.Column,
				Msg:  "pattern variable " + Quoted(name) + " has no field in " + Quoted(def.name),
			})
		}
	}
	sort.SliceStable(diags, func(i, j int) bool { return diags[i].Msg < diags[j].Msg })
	return diags
}

// bindFormField builds the binding tree of one form-bound field. Leaves are
// basic values; structs nest and slices hold one element (REQ-FRM-08).
func bindFormField(def *routeDef, goName string, t types.Type, bind string, pos token.Position) (routeField, bool) {
	field := routeField{
		name:     goName,
		bind:     bind,
		typ:      t,
		typeText: types.TypeString(t, typeQualifier(def.pkg)),
		pos:      pos,
	}
	if kind, ok := bindKind(t); ok {
		field.kind = kind
		return field, true
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			sub := u.Field(i)
			if sub.Embedded() || !sub.Exported() {
				continue
			}
			tag := reflect.StructTag(u.Tag(i))
			if tag.Get("bind") == "-" {
				continue
			}
			name := tag.Get("form")
			if name == "" {
				name = lowerFirst(sub.Name())
			}
			child, ok := bindFormField(def, sub.Name(), sub.Type(), name, pos)
			if ok {
				field.sub = append(field.sub, child)
			}
		}
		return field, len(field.sub) > 0
	case *types.Slice:
		switch u.Elem().Underlying().(type) {
		case *types.Slice, *types.Array:
			return field, false // a slice of slices has no HTML form shape
		}
		child, ok := bindFormField(def, goName, u.Elem(), bind, pos)
		if !ok {
			return field, false
		}
		field.sub = []routeField{child}
		field.slice = true
		return field, true
	case *types.Array:
		child, ok := bindFormField(def, goName, u.Elem(), bind, pos)
		if !ok {
			return field, false
		}
		field.sub = []routeField{child}
		field.slice = true
		field.arrayLen = u.Len()
		return field, true
	}
	return field, false
}

// bindKind returns the basic kind of a bindable field type.
// typeQualifier prints same-package type names without a qualifier.
func typeQualifier(pkg *packages.Package) types.Qualifier {
	return func(p *types.Package) string {
		if p == nil || (pkg != nil && p.Path() == pkg.PkgPath) {
			return ""
		}
		return p.Name()
	}
}

func bindKind(t types.Type) (types.BasicKind, bool) {
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return 0, false
	}
	if b.Info()&(types.IsInteger|types.IsFloat|types.IsString|types.IsBoolean) == 0 {
		return 0, false
	}
	return b.Kind(), true
}

// checkMounted reports a gx.Page value that no gx.Collect holds (REQ-RTE-06).
func (r *typesResult) checkMounted(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		collected := map[types.Object]bool{}
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isGxFunc(pkg, call.Fun, "Collect") {
					return true
				}
				for _, arg := range call.Args {
					if id, ok := arg.(*ast.Ident); ok {
						if obj := pkg.TypesInfo.Uses[id]; obj != nil {
							collected[obj] = true
						}
					}
				}
				return true
			})
		}
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
					for i, val := range vs.Values {
						call, ok := val.(*ast.CallExpr)
						if !ok || !isGxFunc(pkg, call.Fun, "Page") || i >= len(vs.Names) {
							continue
						}
						obj := pkg.TypesInfo.Defs[vs.Names[i]]
						if obj == nil {
							continue
						}
						if key := pageRouteKey(pkg, call); key != "" {
							r.routePages[key] = pkg.Name + "." + vs.Names[i].Name
							r.pageRoutes[obj] = key
						}
						if collected[obj] {
							continue
						}
						pos := pkg.Fset.Position(vs.Names[i].Pos())
						out = append(out, Diagnostic{
							Code: CodeUnmounted,
							File: pos.Filename,
							Line: pos.Line,
							Col:  pos.Column,
							Msg:  "route value " + Quoted(vs.Names[i].Name) + " is not held by any gx.Collect",
						})
					}
				}
			}
		}
	}
	return out
}

// pageRouteKey returns the route type a gx.Page call binds, from the loader's
// input parameter.
func pageRouteKey(pkg *packages.Package, call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	sig, ok := pkg.TypesInfo.TypeOf(call.Args[0]).Underlying().(*types.Signature)
	if !ok || sig.Params().Len() < 2 {
		return ""
	}
	named, ok := sig.Params().At(1).Type().(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	return named.Obj().Pkg().Path() + "." + named.Obj().Name()
}

// checkRoutePackages reports a route package that holds more than route types
// or imports more than gx and the standard library (REQ-RTE-20).
func checkRoutePackages(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		if !strings.HasSuffix(pkg.PkgPath, "/route") {
			continue
		}
		routeNames := map[string]bool{}
		for _, file := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_gx.go") {
				continue
			}
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
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
					if _, isRoute := routePattern(pkg, st); isRoute {
						routeNames[ts.Name.Name] = true
					}
				}
			}
		}
		for _, file := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_gx.go") {
				continue
			}
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				if err != nil || path == "github.com/alternayte/gx" {
					continue
				}
				if dep, ok := pkg.Imports[path]; ok && dep.Module == nil {
					continue // standard library
				}
				pos := pkg.Fset.Position(imp.Pos())
				out = append(out, Diagnostic{
					Code: CodeRoutePkg,
					File: pos.Filename,
					Line: pos.Line,
					Col:  pos.Column,
					Msg:  "route package imports " + Quoted(path) + "; only gx and the standard library are allowed",
				})
			}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					if d.Tok == token.IMPORT {
						continue
					}
					if d.Tok != token.TYPE {
						pos := pkg.Fset.Position(d.Pos())
						out = append(out, Diagnostic{
							Code: CodeRoutePkg,
							File: pos.Filename,
							Line: pos.Line,
							Col:  pos.Column,
							Msg:  "route package holds a " + strings.ToLower(d.Tok.String()) + "; only route types are allowed",
						})
						continue
					}
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						// A struct type is either a route or the nested
						// data of one (REQ-FRM-08).
						if _, ok := ts.Type.(*ast.StructType); ok {
							continue
						}
						pos := pkg.Fset.Position(ts.Pos())
						out = append(out, Diagnostic{
							Code: CodeRoutePkg,
							File: pos.Filename,
							Line: pos.Line,
							Col:  pos.Column,
							Msg:  "route package declares " + Quoted(ts.Name.Name) + " which is not a route type",
						})
					}
				case *ast.FuncDecl:
					// A method of a route type belongs to the route
					// type (REQ-FRM-01 gives the input type Rules).
					if d.Recv != nil && len(d.Recv.List) == 1 && routeNames[receiverTypeName(d.Recv.List[0].Type)] {
						continue
					}
					pos := pkg.Fset.Position(d.Pos())
					out = append(out, Diagnostic{
						Code: CodeRoutePkg,
						File: pos.Filename,
						Line: pos.Line,
						Col:  pos.Column,
						Msg:  "route package holds a func; only route types are allowed",
					})
				}
			}
		}
	}
	return out
}

// checkDuplicatePatterns reports two route structs with one pattern
// (REQ-RTE-07).
func checkDuplicatePatterns(defs []*routeDef) []Diagnostic {
	sorted := append([]*routeDef{}, defs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].file != sorted[j].file {
			return sorted[i].file < sorted[j].file
		}
		return sorted[i].pos.Line < sorted[j].pos.Line
	})
	seen := map[string]*routeDef{}
	var out []Diagnostic
	for _, d := range sorted {
		if prev, ok := seen[d.pattern]; ok {
			out = append(out, Diagnostic{
				Code: CodeDuplicate,
				File: d.file,
				Line: d.pos.Line,
				Col:  d.pos.Column,
				Msg:  "pattern " + Quoted(d.pattern) + " is already used by " + Quoted(prev.name),
			})
			continue
		}
		seen[d.pattern] = d
	}
	return out
}

func isGxFunc(pkg *packages.Package, fun ast.Expr, name string) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	obj := pkg.TypesInfo.Uses[sel.Sel]
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "github.com/alternayte/gx" && obj.Name() == name
}

// renderRouteFiles groups route definitions by source file and renders one
// generated file each.
func renderRouteFiles(defs []*routeDef) map[string][]byte {
	byFile := map[string][]*routeDef{}
	for _, d := range defs {
		byFile[d.file] = append(byFile[d.file], d)
	}
	out := map[string][]byte{}
	for file, group := range byFile {
		base := strings.TrimSuffix(file, ".go")
		out[base+"_gx.go"] = renderRouteFile(group)
	}
	return out
}

func renderRouteFile(defs []*routeDef) []byte {
	var b bytes.Buffer
	b.WriteString("// Code generated by gx. DO NOT EDIT.\n\n")
	sort.SliceStable(defs, func(i, j int) bool { return defs[i].name < defs[j].name })
	b.WriteString("package " + defs[0].pkg.Name + "\n\n")
	needStrconv, needFmt, needURL, needGx := false, false, false, false
	for _, d := range defs {
		// Every route has a URL method: typed links use GET routes and
		// action invocations use every method (REQ-RTE-05, REQ-ACT-02).
		needGx = true
		if d.form {
			needFmt, needStrconv = true, true
		}
		for _, f := range d.fields {
			if f.path != "" || f.query != "" {
				needURL = true
			}
			if f.path != "" || f.query != "" {
				if f.kind != types.String {
					needStrconv, needFmt = true, true
				}
			}
		}
	}
	b.WriteString("import (\n")
	if needFmt {
		b.WriteString("\t\"fmt\"\n")
	}
	if needGx {
		b.WriteString("\tgx \"github.com/alternayte/gx\"\n")
	}
	b.WriteString("\t\"net/http\"\n")
	if needURL {
		b.WriteString("\t\"net/url\"\n")
	}
	if needStrconv {
		b.WriteString("\t\"strconv\"\n")
	}
	if len(defs) > 0 {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString(")\n\n")
	for _, d := range defs {
		b.WriteString("// Pattern returns the method and pattern of " + d.name + ".\n")
		b.WriteString("func (" + d.name + ") Pattern() string { return " + strconv.Quote(d.pattern) + " }\n\n")
		if d.form {
			renderFormBind(&b, d)
		} else {
			renderBindFunc(&b, d)
		}
		renderRouteURL(&b, d)
		if d.form {
			renderFormValue(&b, d)
		}
	}
	src, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes()
	}
	return src
}

// bindNameExpr appends one form name segment to a Go string expression.
func bindNameExpr(base, seg string) string {
	if base == "" {
		return strconv.Quote(seg)
	}
	return "(" + base + " + " + strconv.Quote("."+seg) + ")"
}

// bindIndexExpr appends one slice index to a Go form name expression.
func bindIndexExpr(base, iVar string) string {
	return "(" + base + " + \"[\" + strconv.Itoa(" + iVar + ") + \"]\")"
}

// renderBindFunc writes the Bind method of a non-form route.
func renderBindFunc(b *bytes.Buffer, d *routeDef) {
	b.WriteString("// Bind fills " + d.name + " from the request.\n")
	b.WriteString("func (in *" + d.name + ") Bind(r *http.Request) error {\n")
	if d.hasSignals {
		b.WriteString("\tsignals, err := gx.Signals(r)\n\tif err != nil {\n\t\treturn err\n\t}\n")
	}
	n := 0
	for _, f := range d.fields {
		switch {
		case f.signal != "":
			b.WriteString("\tif err := gx.BindSignal(signals, gx.Scope(r), " + strconv.Quote(f.signal) + ", &in." + f.name + "); err != nil {\n\t\treturn err\n\t}\n")
		case f.path != "" || f.query != "":
			source := "gx.PathValue(r, " + strconv.Quote(f.path) + ")"
			if f.path == "" {
				source = "r.URL.Query().Get(" + strconv.Quote(f.query) + ")"
			}
			b.WriteString("\tif v := " + source + "; v != \"\" {\n")
			renderBindField(b, f)
			b.WriteString("\t}\n")
			if f.query != "" && f.def != "" {
				b.WriteString("\tif in." + f.name + " == " + zeroLiteral(f) + " {\n")
				b.WriteString("\t\tin." + f.name + " = " + defaultLiteral(f) + "\n\t}\n")
			}
		default:
			renderBindTree(b, f, "in."+f.name, strconv.Quote(f.bind), false, 0, &n)
		}
	}
	b.WriteString("\treturn nil\n}\n\n")
}

// renderFormBind writes GxBindForm and the Bind delegator of a form input.
// A conversion error becomes a field error, not a bind failure
// (REQ-FRM-07).
func renderFormBind(b *bytes.Buffer, d *routeDef) {
	b.WriteString("// GxBindForm fills " + d.name + " and returns one message key per field\n")
	b.WriteString("// that did not convert (REQ-FRM-07).\n")
	b.WriteString("func (in *" + d.name + ") GxBindForm(r *http.Request) (map[string]string, error) {\n")
	b.WriteString("\terrs := map[string]string{}\n")
	if d.hasSignals {
		b.WriteString("\tsignals, err := gx.Signals(r)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n")
	}
	n := 0
	for _, f := range d.fields {
		switch {
		case f.signal != "":
			b.WriteString("\tif err := gx.BindSignal(signals, gx.Scope(r), " + strconv.Quote(f.signal) + ", &in." + f.name + "); err != nil {\n\t\treturn nil, err\n\t}\n")
		case f.path != "" || f.query != "":
			source := "r.FormValue(" + strconv.Quote(f.bind) + ")"
			if f.path != "" {
				source = "gx.PathValue(r, " + strconv.Quote(f.path) + ")"
			} else if f.query != "" {
				source = "r.URL.Query().Get(" + strconv.Quote(f.query) + ")"
			}
			b.WriteString("\tif v := " + source + "; v != \"\" {\n")
			renderBindFieldForm(b, f)
			b.WriteString("\t}\n")
			if f.query != "" && f.def != "" {
				b.WriteString("\tif in." + f.name + " == " + zeroLiteral(f) + " {\n")
				b.WriteString("\t\tin." + f.name + " = " + defaultLiteral(f) + "\n\t}\n")
			}
		default:
			renderBindTree(b, f, "in."+f.name, strconv.Quote(f.bind), true, 0, &n)
		}
	}
	b.WriteString("\treturn errs, nil\n}\n\n")
	b.WriteString("// Bind fills " + d.name + " from the request.\n")
	b.WriteString("func (in *" + d.name + ") Bind(r *http.Request) error {\n")
	b.WriteString("\terrs, err := in.GxBindForm(r)\n\tif err != nil {\n\t\treturn err\n\t}\n")
	b.WriteString("\tfor _, name := range gx.FieldNames(errs) {\n")
	b.WriteString("\t\treturn fmt.Errorf(\"gx: %s: %s\", name, errs[name])\n\t}\n")
	b.WriteString("\treturn nil\n}\n\n")
}

// renderBindTree writes binding code for one form-bound tree (REQ-FRM-08).
func renderBindTree(b *bytes.Buffer, f routeField, target, nameExpr string, form bool, depth int, n *int) {
	ind := strings.Repeat("\t", depth+1)
	if f.slice {
		idxVar := "gxIdx" + strconv.Itoa(*n)
		iVar := "i" + strconv.Itoa(*n)
		*n++
		b.WriteString(ind + idxVar + " := gx.FormIndexes(r, " + nameExpr + ")\n")
		if f.arrayLen == 0 {
			b.WriteString(ind + "if n := len(" + idxVar + "); n > 0 {\n")
			b.WriteString(ind + "\t" + target + " = make(" + f.typeText + ", " + idxVar + "[n-1]+1)\n")
			b.WriteString(ind + "}\n")
		}
		b.WriteString(ind + "for _, " + iVar + " := range " + idxVar + " {\n")
		child := f.sub[0]
		renderBindTree(b, child, target+"["+iVar+"]", bindIndexExpr(nameExpr, iVar), form, depth+1, n)
		b.WriteString(ind + "}\n")
		return
	}
	if len(f.sub) > 0 {
		for _, sub := range f.sub {
			renderBindTree(b, sub, target+"."+sub.name, bindNameExpr(nameExpr, sub.bind), form, depth, n)
		}
		return
	}
	b.WriteString(ind + "{\n")
	b.WriteString(ind + "\tif v := gx.FormText(r, " + nameExpr + "); v != \"\" {\n")
	renderParse(b, f, target, ind+"\t\t", form, nameExpr)
	b.WriteString(ind + "\t}\n")
	b.WriteString(ind + "}\n")
}

// renderParse writes the conversion and assignment of one basic leaf.
func renderParse(b *bytes.Buffer, f routeField, target, ind string, form bool, nameExpr string) {
	onErr := func() {
		if form {
			b.WriteString(ind + "errs[" + nameExpr + "] = \"invalid\"\n")
			return
		}
		b.WriteString(ind + "return fmt.Errorf(\"gx: " + f.name + ": %w\", err)\n")
	}
	switch {
	case f.kind == types.String:
		b.WriteString(ind + target + " = " + f.typeText + "(v)\n")
	case f.kind == types.Bool:
		b.WriteString(ind + "x, err := gx.ParseBool(v)\n")
		b.WriteString(ind + "if err != nil {\n")
		onErr()
		b.WriteString(ind + "} else {\n" + ind + "\t" + target + " = " + f.typeText + "(x)\n" + ind + "}\n")
	case floatBits(f.kind) > 0:
		b.WriteString(ind + "x, err := strconv.ParseFloat(v, " + strconv.Itoa(floatBits(f.kind)) + ")\n")
		b.WriteString(ind + "if err != nil {\n")
		onErr()
		b.WriteString(ind + "} else {\n" + ind + "\t" + target + " = " + f.typeText + "(x)\n" + ind + "}\n")
	case unsignedKind(f.kind):
		b.WriteString(ind + "x, err := strconv.ParseUint(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString(ind + "if err != nil {\n")
		onErr()
		b.WriteString(ind + "} else {\n" + ind + "\t" + target + " = " + f.typeText + "(x)\n" + ind + "}\n")
	default:
		b.WriteString(ind + "x, err := strconv.ParseInt(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString(ind + "if err != nil {\n")
		onErr()
		b.WriteString(ind + "} else {\n" + ind + "\t" + target + " = " + f.typeText + "(x)\n" + ind + "}\n")
	}
}

// renderBindFieldForm writes a binding that records a conversion error as a
// field error (REQ-FRM-07) for a top-level path or query field.
func renderBindFieldForm(b *bytes.Buffer, f routeField) {
	name := strconv.Quote(f.bind)
	switch {
	case f.kind == types.String:
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(v)\n")
	case f.kind == types.Bool:
		b.WriteString("\t\tx, err := gx.ParseBool(v)\n")
		b.WriteString("\t\tif err != nil {\n\t\t\terrs[" + name + "] = \"invalid\"\n\t\t} else {\n")
		b.WriteString("\t\t\tin." + f.name + " = " + f.typeText + "(x)\n\t\t}\n")
	case floatBits(f.kind) > 0:
		b.WriteString("\t\tx, err := strconv.ParseFloat(v, " + strconv.Itoa(floatBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\terrs[" + name + "] = \"invalid\"\n\t\t} else {\n")
		b.WriteString("\t\t\tin." + f.name + " = " + f.typeText + "(x)\n\t\t}\n")
	case unsignedKind(f.kind):
		b.WriteString("\t\tx, err := strconv.ParseUint(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\terrs[" + name + "] = \"invalid\"\n\t\t} else {\n")
		b.WriteString("\t\t\tin." + f.name + " = " + f.typeText + "(x)\n\t\t}\n")
	default:
		b.WriteString("\t\tx, err := strconv.ParseInt(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\terrs[" + name + "] = \"invalid\"\n\t\t} else {\n")
		b.WriteString("\t\t\tin." + f.name + " = " + f.typeText + "(x)\n\t\t}\n")
	}
}

// formShape is one struct type in a form's binding tree (REQ-FRM-08).
type formShape struct {
	typeText string
	goName   string
	path     string // Go field path from the root, for constraints
	fields   []routeField
}

// collectShapes appends every nested struct shape, deepest first.
func collectShapes(out *[]formShape, seen map[string]bool, fields []routeField, path string) {
	for _, f := range fields {
		if f.slice {
			child := f.sub[0]
			if len(child.sub) > 0 {
				collectShapes(out, seen, child.sub, path+"."+f.name)
				addShape(out, seen, child.typeText, path+"."+f.name, child.sub)
			}
			continue
		}
		if len(f.sub) > 0 {
			collectShapes(out, seen, f.sub, path+"."+f.name)
			addShape(out, seen, f.typeText, path+"."+f.name, f.sub)
		}
	}
}

func addShape(out *[]formShape, seen map[string]bool, typeText, path string, fields []routeField) {
	if seen[typeText] {
		return
	}
	seen[typeText] = true
	*out = append(*out, formShape{typeText: typeText, goName: formTypeName(typeText), path: strings.TrimPrefix(path, "."), fields: fields})
}

// formTypeName returns the Go name part of a type string.
func formTypeName(typeText string) string {
	if i := strings.LastIndexByte(typeText, '.'); i >= 0 {
		return typeText[i+1:]
	}
	return typeText
}

// fieldGoType returns the generated form field type of one tree node.
func fieldGoType(owner string, f routeField) string {
	if f.slice {
		return owner + f.name + "Field"
	}
	return elemFieldType(f)
}

// elemFieldType returns the generated type of one element field.
func elemFieldType(f routeField) string {
	if len(f.sub) > 0 {
		return formTypeName(f.typeText) + "Form"
	}
	return "gx.FormField[" + f.typeText + "]"
}

// renderFormValue writes the generated form types, the form value builders
// and the field namer (REQ-FRM-02, REQ-FRM-03, REQ-FRM-08).
func renderFormValue(b *bytes.Buffer, d *routeDef) {
	name := lowerFirst(d.name)
	method, _, _ := strings.Cut(d.pattern, " ")
	shapes := []formShape{}
	collectShapes(&shapes, map[string]bool{}, d.fields, "")
	// Struct declarations.
	for _, s := range shapes {
		b.WriteString("// " + s.goName + "Form is the generated form value of " + s.goName + " (REQ-FRM-08).\n")
		b.WriteString("type " + s.goName + "Form struct {\n")
		for _, f := range s.fields {
			b.WriteString("\t" + f.name + " " + fieldGoType(s.goName, f) + "\n")
		}
		b.WriteString("}\n\n")
	}
	b.WriteString("// " + d.name + "Form is the generated form value of " + d.name + " (REQ-FRM-03).\n")
	b.WriteString("type " + d.name + "Form struct {\n\tgx.FormMeta\n")
	for _, f := range d.fields {
		b.WriteString("\t" + f.name + " " + fieldGoType(d.name, f) + "\n")
	}
	b.WriteString("}\n\n")
	// Slice wrappers: one per slice field of each shape and of the root.
	for _, s := range shapes {
		renderSliceWrappers(b, d, s.goName, s.fields)
	}
	renderSliceWrappers(b, d, d.name, d.fields)
	// Root slice constructors: actions rebuild a repeated field (REQ-FRM-08).
	for _, f := range d.fields {
		if !f.slice {
			continue
		}
		ctor := d.name + f.name + "FieldValue"
		b.WriteString("// " + ctor + " builds the repeated field " + d.name + "." + f.name + " (REQ-FRM-08).\n")
		b.WriteString("func " + ctor + "(form string, v " + f.typeText + ", errs map[string]string) " + d.name + f.name + "Field {\n")
		b.WriteString("\treturn " + d.name + f.name + "Field{FormField: gx.FormField[" + f.typeText + "]{Name: " + strconv.Quote(f.bind) +
			", ID: gx.FieldID(form, " + strconv.Quote(f.bind) + "), Value: v}, form: form, errs: errs}\n}\n\n")
	}
	// Struct builders.
	for _, s := range shapes {
		renderShapeBuilder(b, d, s)
	}
	// The root form value builder.
	b.WriteString("// GxFormValue fills the form value of " + d.name + " (REQ-FRM-03).\n")
	b.WriteString("func (in *" + d.name + ") GxFormValue(errs map[string]string) gx.FormValue {\n")
	b.WriteString("\tf := " + d.name + "Form{FormMeta: gx.FormMeta{Name: " + strconv.Quote(name) +
		", ID: " + strconv.Quote(name+"-form") + ", Action: in.URL(), Method: " + strconv.Quote(method) + "}}\n")
	for _, f := range d.fields {
		b.WriteString("\tf." + f.name + " = " + formFieldExpr(d, d.name, strconv.Quote(name), "in.URL()", f, "in."+f.name, "errs", "", "") + "\n")
	}
	b.WriteString("\treturn f\n}\n\n")
	// Field namer.
	b.WriteString("// GxFieldName returns the form field name of a field pointer (REQ-FRM-02).\n")
	b.WriteString("func (in *" + d.name + ") GxFieldName(ptr any) string {\n\tswitch ptr {\n")
	renderFieldNameCases(b, d.fields, "in", "")
	b.WriteString("\t}\n\treturn \"\"\n}\n\n")
	b.WriteString("// GxNewForm returns a fresh " + d.name + " (REQ-FRM-02).\n")
	b.WriteString("func (in *" + d.name + ") GxNewForm() gx.FormInput { return &" + d.name + "{} }\n\n")
	b.WriteString("// GxRunForm calls the form handler with the concrete input type (REQ-FRM-02).\n")
	b.WriteString("func (in *" + d.name + ") GxRunForm(ctx *gx.Ctx, fn any) error {\n")
	b.WriteString("\treturn fn.(func(*gx.Ctx, *" + d.name + ") error)(ctx, in)\n}\n\n")
}

// renderSliceWrappers writes the wrapper of every direct slice field.
func renderSliceWrappers(b *bytes.Buffer, d *routeDef, owner string, fields []routeField) {
	for _, f := range fields {
		if f.slice {
			renderSliceWrapper(b, d, owner, f)
		}
	}
}

// renderSliceWrapper writes one slice field wrapper type (REQ-FRM-08).
func renderSliceWrapper(b *bytes.Buffer, d *routeDef, owner string, f routeField) {
	child := f.sub[0]
	name := owner + f.name + "Field"
	elem := elemFieldType(child)
	b.WriteString("// " + name + " is the generated slice field of " + owner + "." + f.name + " (REQ-FRM-08).\n")
	b.WriteString("type " + name + " struct {\n\tgx.FormField[" + f.typeText + "]\n\tform string\n\terrs map[string]string\n}\n\n")
	b.WriteString("// Rows fills the form value of every row.\n")
	b.WriteString("func (f " + name + ") Rows() []" + elem + " {\n")
	b.WriteString("\tout := make([]" + elem + ", len(f.Value))\n")
	b.WriteString("\tfor i, v := range f.Value {\n")
	b.WriteString("\t\tname := f.FormField.Name + \"[\" + strconv.Itoa(i) + \"]\"\n")
	if len(child.sub) > 0 {
		b.WriteString("\t\tout[i] = " + lowerFirst(formTypeName(child.typeText)) + "FormValue(f.form, name, v, f.errs)\n")
	} else {
		b.WriteString("\t\tout[i] = gx.FormField[" + child.typeText + "]{Name: name, ID: gx.FieldID(f.form, name), Value: v, ErrorKey: f.errs[name], Error: gx.Translate(f.errs[name], gx.DefaultMessage(f.errs[name]))}\n")
	}
	b.WriteString("\t}\n\treturn out\n}\n\n")
	b.WriteString("// Each renders one node per row (REQ-FRM-08).\n")
	b.WriteString("func (f " + name + ") Each(fn func(int, " + elem + ") gx.Node) gx.Node {\n")
	b.WriteString("\treturn gx.EachRow(f.Rows(), fn)\n}\n\n")
}

// renderShapeBuilder writes the form value builder of one nested struct.
func renderShapeBuilder(b *bytes.Buffer, d *routeDef, s formShape) {
	fn := lowerFirst(s.goName) + "FormValue"
	b.WriteString("// " + fn + " fills the form value of one " + s.goName + " (REQ-FRM-08).\n")
	b.WriteString("func " + fn + "(form, prefix string, v " + s.typeText + ", errs map[string]string) " + s.goName + "Form {\n")
	b.WriteString("\tvar f " + s.goName + "Form\n")
	for _, fld := range s.fields {
		b.WriteString("\tf." + fld.name + " = " + formFieldExpr(d, s.goName, "form", "", fld, "v."+fld.name, "errs", "prefix", s.path) + "\n")
	}
	b.WriteString("\treturn f\n}\n\n")
}

// formFieldExpr returns the Go expression of one generated field value.
// owner is the Go type that declares the field; prefix is "" at the root or
// the parent name expression inside a nested builder; cpath is the Go field
// path used for rule constraints.
func formFieldExpr(d *routeDef, owner, formExpr, urlExpr string, f routeField, value, errs, prefix, cpath string) string {
	nameExpr := strconv.Quote(f.bind)
	if prefix != "" {
		nameExpr = "(" + prefix + " + " + strconv.Quote("."+f.bind) + ")"
	}
	if f.slice {
		return owner + f.name + "Field{FormField: gx.FormField[" + f.typeText + "]{Name: " + nameExpr +
			", ID: gx.FieldID(" + formExpr + ", " + nameExpr + "), Value: " + value +
			"}, form: " + formExpr + ", errs: " + errs + "}"
	}
	if len(f.sub) > 0 {
		return lowerFirst(formTypeName(f.typeText)) + "FormValue(" + formExpr + ", " + nameExpr + ", " + value + ", " + errs + ")"
	}
	key := errs + "[" + nameExpr + "]"
	validate := "\"\""
	if urlExpr != "" {
		validate = "gx.ValidateURL(" + urlExpr + ", " + nameExpr + ")"
	}
	return "gx.FormField[" + f.typeText + "]{" +
		"Name: " + nameExpr +
		", ID: gx.FieldID(" + formExpr + ", " + nameExpr + ")" +
		", Value: " + value +
		", ErrorKey: " + key +
		", Error: gx.Translate(" + key + ", gx.DefaultMessage(" + key + "))" +
		", Constraints: " + constraintsExpr(d.formRules[joinPath(cpath, f.name)]) +
		", ValidateURL: " + validate + "}"
}

// joinPath joins a Go field path with one field name.
func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// renderFieldNameCases writes the GxFieldName switch cases of one level.
func renderFieldNameCases(b *bytes.Buffer, fields []routeField, goPrefix, namePrefix string) {
	for _, f := range fields {
		if f.slice {
			continue // slice elements have no stable pointer
		}
		if len(f.sub) > 0 {
			renderFieldNameCases(b, f.sub, goPrefix+"."+f.name, namePrefix+f.bind+".")
			continue
		}
		path := namePrefix + f.bind
		b.WriteString("\tcase any(&" + goPrefix + "." + f.name + "):\n\t\treturn " + strconv.Quote(path) + "\n")
	}
}

// constraintsExpr returns the gx.Attrs literal of a field's native
// constraints (REQ-FRM-04).
func constraintsExpr(cs []formConstraint) string {
	if len(cs) == 0 {
		return "nil"
	}
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		if c.isBool {
			parts = append(parts, "gx.Bool("+strconv.Quote(c.key)+", true)")
			continue
		}
		parts = append(parts, "gx.Attr{Key: "+strconv.Quote(c.key)+", Value: "+strconv.Quote(c.value)+", Kind: gx.AttrText}")
	}
	return "gx.Attrs{" + strings.Join(parts, ", ") + "}"
}

// renderRouteURL writes the URL method of a GET route (REQ-RTE-05).
func renderRouteURL(b *bytes.Buffer, d *routeDef) {
	_, path, _ := strings.Cut(d.pattern, " ")
	byPath := map[string]*routeField{}
	for i := range d.fields {
		if d.fields[i].path != "" {
			byPath[strings.ToLower(d.fields[i].path)] = &d.fields[i]
		}
	}
	b.WriteString("// URL returns the path of " + d.name + ".\n")
	b.WriteString("func (in " + d.name + ") URL() string {\n")
	b.WriteString("\tvar b strings.Builder\n")
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if i > 0 {
			b.WriteString("\tb.WriteString(\"/\")\n")
		}
		m := pathVarRe.FindStringSubmatch(seg)
		if m == nil {
			if seg != "" {
				b.WriteString("\tb.WriteString(" + strconv.Quote(seg) + ")\n")
			}
			continue
		}
		f := byPath[strings.ToLower(m[1])]
		if f == nil {
			continue
		}
		b.WriteString("\tb.WriteString(" + urlValue(*f) + ")\n")
	}
	var queries []routeField
	for _, f := range d.fields {
		if f.query != "" {
			queries = append(queries, f)
		}
	}
	if len(queries) > 0 {
		b.WriteString("\tq := url.Values{}\n")
		for _, f := range queries {
			b.WriteString("\tif " + queryCondition(f) + " {\n")
			b.WriteString("\t\tq.Set(" + strconv.Quote(f.query) + ", " + urlQueryValue(f) + ")\n")
			b.WriteString("\t}\n")
		}
		b.WriteString("\tif s := q.Encode(); s != \"\" {\n\t\tb.WriteString(\"?\")\n\t\tb.WriteString(s)\n\t}\n")
	}
	b.WriteString("\treturn gx.BasePath() + b.String()\n}\n\n")
}

func urlValue(f routeField) string {
	return "url.PathEscape(" + formatValue(f, "in."+f.name) + ")"
}

func urlQueryValue(f routeField) string {
	return formatValue(f, "in."+f.name)
}

// formatValue returns the string form of a field value.
func formatValue(f routeField, expr string) string {
	switch {
	case f.kind == types.String:
		return expr
	case f.kind == types.Bool:
		return "strconv.FormatBool(" + expr + ")"
	case floatBits(f.kind) > 0:
		return "strconv.FormatFloat(float64(" + expr + "), 'g', -1, 64)"
	case unsignedKind(f.kind):
		return "strconv.FormatUint(uint64(" + expr + "), 10)"
	default:
		return "strconv.FormatInt(int64(" + expr + "), 10)"
	}
}

// queryCondition omits a query value that is zero or equal to its default.
func queryCondition(f routeField) string {
	var conds []string
	if f.kind == types.String {
		conds = append(conds, "in."+f.name+` != ""`)
		if f.def != "" {
			conds = append(conds, "in."+f.name+" != "+strconv.Quote(f.def))
		}
	} else if f.kind == types.Bool {
		conds = append(conds, "in."+f.name)
		if f.def != "" {
			conds = append(conds, "in."+f.name+" != "+f.def)
		}
	} else {
		conds = append(conds, "in."+f.name+" != 0")
		if f.def != "" {
			conds = append(conds, "in."+f.name+" != "+f.def)
		}
	}
	return strings.Join(conds, " && ")
}

func renderBindField(b *bytes.Buffer, f routeField) {
	switch {
	case f.kind == types.String:
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(v)\n")
	case f.kind == types.Bool:
		b.WriteString("\t\tx, err := gx.ParseBool(v)\n")
		b.WriteString("\t\tif err != nil {\n\t\t\treturn fmt.Errorf(" + strconv.Quote("gx: "+f.name+": %w") + ", err)\n\t\t}\n")
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(x)\n")
	case floatBits(f.kind) > 0:
		b.WriteString("\t\tx, err := strconv.ParseFloat(v, " + strconv.Itoa(floatBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\treturn fmt.Errorf(" + strconv.Quote("gx: "+f.name+": %w") + ", err)\n\t\t}\n")
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(x)\n")
	case unsignedKind(f.kind):
		b.WriteString("\t\tx, err := strconv.ParseUint(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\treturn fmt.Errorf(" + strconv.Quote("gx: "+f.name+": %w") + ", err)\n\t\t}\n")
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(x)\n")
	default:
		b.WriteString("\t\tx, err := strconv.ParseInt(v, 10, " + strconv.Itoa(intBits(f.kind)) + ")\n")
		b.WriteString("\t\tif err != nil {\n\t\t\treturn fmt.Errorf(" + strconv.Quote("gx: "+f.name+": %w") + ", err)\n\t\t}\n")
		b.WriteString("\t\tin." + f.name + " = " + f.typeText + "(x)\n")
	}
}

func intBits(k types.BasicKind) int {
	switch k {
	case types.Int8, types.Uint8:
		return 8
	case types.Int16, types.Uint16:
		return 16
	case types.Int32, types.Uint32:
		return 32
	case types.Int64, types.Uint64:
		return 64
	default:
		return 0
	}
}

func floatBits(k types.BasicKind) int {
	switch k {
	case types.Float32:
		return 32
	case types.Float64:
		return 64
	default:
		return 0
	}
}

func unsignedKind(k types.BasicKind) bool {
	switch k {
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		return true
	}
	return false
}

func zeroLiteral(f routeField) string {
	if f.kind == types.String {
		return `""`
	}
	if f.kind == types.Bool {
		return "false"
	}
	if floatBits(f.kind) > 0 {
		return "0"
	}
	return "0"
}

func defaultLiteral(f routeField) string {
	if f.kind == types.String {
		return strconv.Quote(f.def)
	}
	return f.def
}
