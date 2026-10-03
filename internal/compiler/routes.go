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
	pkg     *packages.Package
	file    string
	name    string
	pattern string
	fields  []routeField
	pos     token.Position
}

// routeField is one bindable field of a route struct.
type routeField struct {
	name     string
	typeText string
	kind     types.BasicKind
	path     string
	query    string
	def      string
	pos      token.Position
}

var pathVarRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(\.\.\.)?\}`)

// collectRoutes finds every route struct in the loaded packages and checks
// its pattern against its fields (REQ-RTE-02).
func collectRoutes(pkgs []*packages.Package) ([]*routeDef, []Diagnostic) {
	var defs []*routeDef
	var diags []Diagnostic
	for _, pkg := range pkgs {
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
					diags = append(diags, routeFields(def, stype)...)
					defs = append(defs, def)
				}
			}
		}
	}
	return defs, diags
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
		if !isPath && queryName == "" {
			continue // not a request-bound field
		}
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
			typeText: types.TypeString(f.Type(), nil),
			kind:     kind,
			query:    queryName,
			def:      tag.Get("default"),
			pos:      pos,
		}
		if isPath {
			field.path = pathName
			bound[strings.ToLower(pathName)] = true
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

// bindKind returns the basic kind of a bindable field type.
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
						if st, ok := ts.Type.(*ast.StructType); ok {
							if _, isRoute := routePattern(pkg, st); isRoute {
								continue
							}
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
		if strings.HasPrefix(d.pattern, "GET ") || strings.HasPrefix(d.pattern, "HEAD ") {
			needURL = true
			needGx = true
		}
		for _, f := range d.fields {
			if f.path != "" {
				needGx = true
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
	if needURL {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString(")\n\n")
	for _, d := range defs {
		b.WriteString("// Pattern returns the method and pattern of " + d.name + ".\n")
		b.WriteString("func (" + d.name + ") Pattern() string { return " + strconv.Quote(d.pattern) + " }\n\n")
		b.WriteString("// Bind fills " + d.name + " from the request.\n")
		b.WriteString("func (in *" + d.name + ") Bind(r *http.Request) error {\n")
		for _, f := range d.fields {
			if f.path == "" && f.query == "" {
				continue
			}
			source := "gx.PathValue(r, " + strconv.Quote(f.path) + ")"
			if f.path == "" {
				source = "r.URL.Query().Get(" + strconv.Quote(f.query) + ")"
			}
			b.WriteString("\tif v := " + source + "; v != \"\" {\n")
			renderBindField(&b, f)
			b.WriteString("\t}\n")
			if f.query != "" && f.def != "" {
				b.WriteString("\tif in." + f.name + " == " + zeroLiteral(f) + " {\n")
				b.WriteString("\t\tin." + f.name + " = " + defaultLiteral(f) + "\n\t}\n")
			}
		}
		b.WriteString("\treturn nil\n}\n\n")
		if strings.HasPrefix(d.pattern, "GET ") || strings.HasPrefix(d.pattern, "HEAD ") {
			renderRouteURL(&b, d)
		}
	}
	src, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes()
	}
	return src
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
		b.WriteString("\t\tx, err := strconv.ParseBool(v)\n")
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
