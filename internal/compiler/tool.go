package compiler

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// toolDecl is one .Tool() call on a gx.Action or a gx.Form (REQ-AI-06).
type toolDecl struct {
	// route is the key of the input type: package path and name.
	route string
	// doc is the doc comment of the variable of the action. It is the
	// description of the tool.
	doc string
	at  token.Position
}

// toolRoot returns the gx.Action or gx.Form call at the root of a call
// chain that has a .Tool() call, such as gx.Action(fn).Tool(gx.Confirm).
func toolRoot(pkg *packages.Package, expr ast.Expr) *ast.CallExpr {
	hasTool := false
	for {
		call, ok := ast.Unparen(expr).(*ast.CallExpr)
		if !ok {
			return nil
		}
		if isGxFuncExpr(pkg, call.Fun, "Action") || isGxFuncExpr(pkg, call.Fun, "Form") {
			if hasTool {
				return call
			}
			return nil
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		if sel.Sel.Name == "Tool" {
			hasTool = true
		}
		expr = sel.X
	}
}

// collectTools finds each tool of the module, by the key of its input type.
func collectTools(pkgs []*packages.Package) map[string]*toolDecl {
	out := map[string]*toolDecl{}
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
		// The doc comment of a variable, by the root call of its value.
		docs := map[*ast.CallExpr]string{}
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
				doc := vs.Doc
				if doc == nil && len(gen.Specs) == 1 {
					doc = gen.Doc
				}
				for _, val := range vs.Values {
					if root := toolRoot(pkg, val); root != nil && doc != nil {
						docs[root] = strings.Join(strings.Fields(doc.Text()), " ")
					}
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			root := toolRoot(pkg, call)
			if root == nil || len(root.Args) == 0 {
				return true
			}
			t := pkg.TypesInfo.TypeOf(root.Args[0])
			if t == nil {
				return true
			}
			sig, ok := t.Underlying().(*types.Signature)
			if !ok || sig.Params().Len() != 2 {
				return true
			}
			// The input of a form is a pointer to its route type.
			in := sig.Params().At(1).Type()
			if ptr, ok := in.(*types.Pointer); ok {
				in = ptr.Elem()
			}
			key := namedTypeKey(in)
			if key == "" || out[key] != nil {
				return true
			}
			out[key] = &toolDecl{route: key, doc: docs[root], at: pkg.Fset.Position(root.Pos())}
			return true
		})
	})
	return out
}

// toolName makes the name of a tool from its input type: the slice and the
// type, in lower case with underscores. "products/route".AddToCart gives
// "products_add_to_cart". The slice is in the name because two slices can
// have a route type of one name.
func toolName(d *routeDef) string {
	slice := d.pkg.Name
	if slice == "route" {
		slice = path.Base(path.Dir(d.pkg.PkgPath))
	}
	return snakeCase(slice) + "_" + snakeCase(d.name)
}

// snakeCase writes a Go name in lower case with underscores: "AddToCart"
// gives "add_to_cart", and "HTTPPing" gives "http_ping".
func snakeCase(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
				b.WriteByte('_')
			}
			continue
		}
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// toolRules reads the Rules method of a tool input: for the dotted Go path
// of each gx.Field call, the JSON Schema keywords of its rules
// (REQ-AI-09). A rule with no JSON Schema form, such as gx.Check, runs on
// the server only.
func toolRules(d *routeDef) (keywords map[string]map[string]any, required map[string]bool) {
	keywords, required = map[string]map[string]any{}, map[string]bool{}
	for _, file := range d.pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "Rules" || fn.Body == nil || len(fn.Recv.List) != 1 {
				continue
			}
			if receiverTypeName(fn.Recv.List[0].Type) != d.name {
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
				if keywords[name] == nil {
					keywords[name] = map[string]any{}
				}
				for _, arg := range call.Args[1:] {
					if ruleKeywords(d.pkg, arg, keywords[name]) {
						required[name] = true
					}
				}
				return true
			})
		}
	}
	return keywords, required
}

// ruleKeywords adds the JSON Schema keywords of one rule to out. It reports
// whether the rule is gx.Required. The keywords of gx.Each go under
// "items".
func ruleKeywords(pkg *packages.Package, expr ast.Expr, out map[string]any) (required bool) {
	number := func(call *ast.CallExpr) (any, bool) {
		if len(call.Args) != 1 {
			return nil, false
		}
		tv, ok := pkg.TypesInfo.Types[call.Args[0]]
		if !ok || tv.Value == nil {
			return nil, false
		}
		if n, exact := constant.Int64Val(constant.ToInt(tv.Value)); exact {
			return n, true
		}
		f, _ := constant.Float64Val(tv.Value)
		return f, true
	}
	switch t := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		switch ruleName(pkg, t) {
		case "Required":
			return true
		case "Email":
			out["format"] = "email"
		case "IsURL":
			out["format"] = "uri"
		}
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		set := func(key string) {
			if n, ok := number(t); ok {
				out[key] = n
			}
		}
		switch ruleName(pkg, sel) {
		case "MinLen":
			set("minLength")
		case "MaxLen":
			set("maxLength")
		case "Min":
			set("minimum")
		case "Max":
			set("maximum")
		case "Pattern":
			if re := patternLiteral(pkg, t); re != "" {
				out["pattern"] = re
			}
		case "OneOf":
			values := []any{}
			for _, arg := range t.Args {
				tv, ok := pkg.TypesInfo.Types[arg]
				if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
					return false
				}
				values = append(values, constant.StringVal(tv.Value))
			}
			if len(values) > 0 {
				out["enum"] = values
			}
		case "True":
			out["const"] = true
		case "Each":
			if len(t.Args) == 1 {
				items, _ := out["items"].(map[string]any)
				if items == nil {
					items = map[string]any{}
					out["items"] = items
				}
				ruleKeywords(pkg, t.Args[0], items)
			}
		}
	}
	return false
}

// toolFieldName returns the name of a top-level field in the arguments of a
// tool, and the part of the request that the binder reads it from.
func toolFieldName(f routeField) (name, in string) {
	switch {
	case f.path != "":
		return f.path, "path"
	case f.query != "":
		return f.query, "query"
	case f.signal != "":
		return f.signal, "signal"
	}
	return f.bind, "form"
}

// toolSchema builds the JSON Schema of the input of a tool. It returns the
// top-level fields in the order of the struct, and the Go path of each
// field that a JSON value cannot fill.
func toolSchema(d *routeDef) (schema map[string]any, fields [][2]string, files []string) {
	keywords, required := toolRules(d)
	// rules is false for the element of a list: the rules of the Go path
	// are the rules of the list.
	var value func(f routeField, goPath string, rules bool) map[string]any
	object := func(sub []routeField, goPath string, top bool) map[string]any {
		props := map[string]any{}
		var req []any
		for _, f := range sub {
			p := f.name
			if goPath != "" {
				p = goPath + "." + f.name
			}
			if f.file || treeHasFile([]routeField{f}) {
				files = append(files, p)
				continue
			}
			name, in := f.bind, "form"
			if top {
				name, in = toolFieldName(f)
				fields = append(fields, [2]string{name, in})
			}
			props[name] = value(f, p, true)
			// A path variable is a part of the address, so it is never
			// absent.
			if required[p] || in == "path" {
				req = append(req, name)
			}
		}
		out := map[string]any{"type": "object", "additionalProperties": false, "properties": props}
		if len(req) > 0 {
			out["required"] = req
		}
		return out
	}
	value = func(f routeField, goPath string, rules bool) map[string]any {
		var out map[string]any
		switch {
		case f.slice:
			out = map[string]any{"type": "array", "items": value(f.sub[0], goPath, false)}
			if f.arrayLen > 0 {
				out["maxItems"] = f.arrayLen
			}
			// The rules of the field are the rules of the list: gx.Each
			// gives the keywords of an item.
			item := out["items"].(map[string]any)
			for k, v := range keywords[goPath] {
				if k == "items" {
					for ik, iv := range v.(map[string]any) {
						item[ik] = iv
					}
					continue
				}
				out[k] = v
			}
			return out
		case len(f.sub) > 0:
			return object(f.sub, goPath, false)
		}
		out = map[string]any{"type": jsonType(f.kind)}
		if f.def != "" {
			out["default"] = jsonDefault(f.kind, f.def)
		}
		if rules {
			for k, v := range keywords[goPath] {
				out[k] = v
			}
		}
		return out
	}
	schema = object(d.fields, "", true)
	return schema, fields, files
}

func jsonType(kind types.BasicKind) string {
	info := types.Typ[kind].Info()
	switch {
	case info&types.IsBoolean != 0:
		return "boolean"
	case info&types.IsInteger != 0:
		return "integer"
	case info&types.IsFloat != 0:
		return "number"
	}
	return "string"
}

// jsonDefault returns the default of a tag as a value of the JSON type of
// its field.
func jsonDefault(kind types.BasicKind, text string) any {
	switch jsonType(kind) {
	case "boolean":
		if b, err := strconv.ParseBool(text); err == nil {
			return b
		}
	case "integer":
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
	case "number":
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
	}
	return text
}

// renderToolInfo writes the GxTool method of a tool input: the name, the
// description and the JSON Schema of the tool (REQ-AI-06, REQ-AI-09).
func renderToolInfo(b *bytes.Buffer, d *routeDef) {
	schema, fields, _ := toolSchema(d)
	// encoding/json writes the keys of a map in order, so the schema of one
	// input is one text.
	text, err := json.Marshal(schema)
	if err != nil {
		return
	}
	b.WriteString("// GxTool describes " + d.name + " as a tool.\n")
	b.WriteString("func (" + d.name + ") GxTool() gx.ToolInfo {\n\treturn gx.ToolInfo{\n")
	b.WriteString("\t\tName: " + strconv.Quote(toolName(d)) + ",\n")
	b.WriteString("\t\tDescription: " + strconv.Quote(d.tool.doc) + ",\n")
	b.WriteString("\t\tSchema: " + strconv.Quote(string(text)) + ",\n")
	b.WriteString("\t\tFields: []gx.ToolField{")
	for i, f := range fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{Name: " + strconv.Quote(f[0]) + ", In: " + strconv.Quote(f[1]) + "}")
	}
	b.WriteString("},\n\t}\n}\n\n")
}

// checkTools reports GX4011 for a tool that a client cannot use: a tool
// with no doc comment has no description, and a gx.File field has no JSON
// value.
func checkTools(defs []*routeDef) []Diagnostic {
	var out []Diagnostic
	for _, d := range defs {
		if d.tool == nil {
			continue
		}
		at := d.tool.at
		if d.tool.doc == "" {
			out = append(out, Diagnostic{
				Code: CodeTool, File: at.Filename, Line: at.Line, Col: at.Column,
				Msg: "the tool of " + Quoted(d.name) + " has no description; a tool takes it from the doc comment of the variable of its action",
				Fix: "declare the action as a variable and write a doc comment above it that says what the tool does",
			})
		}
		if _, _, files := toolSchema(d); len(files) > 0 {
			out = append(out, Diagnostic{
				Code: CodeTool, File: at.Filename, Line: at.Line, Col: at.Column,
				Msg: "the tool of " + Quoted(d.name) + " has the field " + Quoted(files[0]) + " of type gx.File; a tool call has JSON arguments and cannot send a file",
				Fix: "remove .Tool() from the action, or make a second action with no file for the tool",
			})
		}
	}
	return out
}

// checkToolResultSecrets reports a gx.ToolResult call whose value holds a
// gx.Secret (GX7002, SI-04). The result of a tool goes to an agent as JSON
// (REQ-AI-08).
func checkToolResultSecrets(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	sourceFiles(pkgs, func(pkg *packages.Package, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isGxFuncExpr(pkg, call.Fun, "ToolResult") || len(call.Args) != 2 {
				return true
			}
			path := secretPath(pkg.TypesInfo.TypeOf(call.Args[1]), map[types.Type]bool{})
			if path == nil {
				return true
			}
			at := pkg.Fset.Position(call.Args[1].Pos())
			where := "the value"
			if len(path) > 0 {
				where = "the field " + Quoted(strings.Join(path, "."))
			}
			out = append(out, Diagnostic{
				Code: CodeSecret, File: at.Filename, Line: at.Line, Col: at.Column,
				Msg: "tool result: " + where + " has type gx.Secret; a secret cannot cross to an agent",
			})
			return true
		})
	})
	return out
}
