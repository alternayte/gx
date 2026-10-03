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
	needStrconv, needFmt := false, false
	for _, d := range defs {
		for _, f := range d.fields {
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
	b.WriteString("\t\"net/http\"\n")
	if needStrconv {
		b.WriteString("\t\"strconv\"\n")
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
			source := "r.PathValue(" + strconv.Quote(f.path) + ")"
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
	}
	src, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes()
	}
	return src
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
