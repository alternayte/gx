package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"maps"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// snippet is the example code of one fixture, in .gx syntax (REQ-DOC-04).
type snippet struct {
	Code string
	// Tag is true when Code is the component tag of the fixture. Call is
	// true when the component is a Go function and Code is its call, the
	// only form such a component has. Otherwise Code renders the fixture
	// by its name and Why says what stopped the conversion.
	Tag  bool
	Call bool
	Why  string
	// Action is the action code that pushes the fixture, for a component
	// that renders one gx.ToastPatch. It is "" for any other component.
	Action string
	// Imports maps each package qualifier of Code to its import path.
	Imports map[string]string
}

// converter turns the fixtures of one component into .gx markup. It reads
// syntax only: a construct it does not know becomes a Go expression, and a
// fixture that names something outside its package's public API becomes a
// reference to the fixture itself.
type converter struct {
	reg  *registry
	it   *item
	comp *component
	// in is the text the converter reads: the fixture with every helper
	// written out.
	in *source
	// fileImports maps the qualifiers of the fixture to import paths;
	// imports collects the ones the snippet uses.
	fileImports map[string]string
	imports     map[string]string
	// action is the action code of the fixture in hand.
	action string
}

// newConverter prepares the conversion of one component's fixtures.
func newConverter(reg *registry, it *item, comp *component) *converter {
	return &converter{reg: reg, it: it, comp: comp}
}

// convert returns the example code of one fixture.
func (c *converter) convert(fx fixture) snippet {
	c.imports = map[string]string{}
	c.fileImports = maps.Clone(c.comp.in.imports)
	c.action = ""
	code, call, err := c.fixtureCode(fx)
	if err != nil {
		code := fmt.Sprintf("{%s.%s(%s.%s[%s])}", c.it.PkgName, c.comp.Name, c.it.PkgName, c.comp.FixturesVar, strconv.Quote(fx.Name))
		return snippet{Code: code, Why: err.Error(), Imports: map[string]string{c.it.PkgName: c.it.PkgPath}}
	}
	return snippet{Code: code, Tag: !call, Call: call, Action: c.action, Imports: c.imports}
}

// fixtureCode converts the prop literal of a fixture to the tag of its
// component, or to the call of a component that is a Go function.
func (c *converter) fixtureCode(fx fixture) (code string, call bool, err error) {
	lit, ok := fx.Value.(*ast.CompositeLit)
	if !ok || lit.Type != nil {
		return "", false, errors.New("the fixture is not a plain prop literal")
	}
	text, err := c.expand(fx.Value, c.comp.in, nil, 0)
	if err != nil {
		return "", false, err
	}
	// The literal has no type of its own; the props type of the fixtures
	// variable makes it an expression.
	typ := c.comp.in
	text = string(typ.src[typ.fset.Position(c.comp.PropsType.Pos()).Offset:typ.fset.Position(c.comp.PropsType.End()).Offset]) + text
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "", text, 0)
	if err != nil {
		return "", false, err
	}
	c.in = &source{src: []byte(text), fset: fset}
	if c.comp.Gx == nil {
		props, err := c.qualify(expr)
		if err != nil {
			return "", false, err
		}
		c.imports[c.it.PkgName] = c.it.PkgPath
		return "{" + c.it.PkgName + "." + c.comp.Name + "(" + props + ")}", true, nil
	}
	el, err := c.tag(c.it.PkgName, c.it.PkgPath, c.comp.Name, c.comp.Gx, expr.(*ast.CompositeLit))
	if err != nil {
		return "", false, err
	}
	for _, elt := range expr.(*ast.CompositeLit).Elts {
		kv := elt.(*ast.KeyValueExpr)
		for _, f := range c.comp.Gx.Props {
			if isIdent(kv.Key, f.Name) && strings.TrimSpace(f.Type) == "gx.ToastPatch" {
				c.action = c.toastAction(kv.Value)
			}
		}
	}
	var b strings.Builder
	writeNode(&b, markupNode{el: el}, 0)
	return strings.TrimRight(b.String(), "\n"), false, nil
}

// maxExpand bounds the helpers one fixture expands through, so a helper
// that names itself ends.
const maxExpand = 16

// expand returns the source of expr with every helper of the item package
// written out: the value of an unexported constant or variable, and the
// returned expression of an unexported one-line function with its
// arguments in place of its parameters. A user of the component has no
// access to an unexported name, so the example shows the value.
func (c *converter) expand(expr ast.Expr, in *source, args map[string]string, depth int) (string, error) {
	if depth > maxExpand {
		return "", errors.New("the fixture helpers are too deep")
	}
	for qual, p := range in.imports {
		if _, ok := c.fileImports[qual]; !ok {
			c.fileImports[qual] = p
		}
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	var err error
	offset := func(pos token.Pos) int { return in.fset.Position(pos).Offset }
	// skip holds the identifiers that name a struct field or a selected
	// member; they are not package-level names.
	skip := map[*ast.Ident]bool{}
	ast.Inspect(expr, func(n ast.Node) bool {
		if err != nil {
			return false
		}
		switch t := n.(type) {
		case *ast.SelectorExpr:
			skip[t.Sel] = true
		case *ast.CompositeLit:
			if _, isMap := t.Type.(*ast.MapType); isMap {
				break
			}
			for _, elt := range t.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if key, ok := kv.Key.(*ast.Ident); ok {
						skip[key] = true
					}
				}
			}
		case *ast.CallExpr:
			name, ok := t.Fun.(*ast.Ident)
			if !ok {
				break
			}
			h := c.it.helpers[name.Name]
			if _, shadowed := args[name.Name]; h == nil || !h.isFunc || shadowed {
				break
			}
			if len(t.Args) != len(h.params) || t.Ellipsis.IsValid() {
				err = fmt.Errorf("the fixture calls the helper %s with a different argument list", name.Name)
				return false
			}
			inner := map[string]string{}
			for i, arg := range t.Args {
				text, e := c.expand(arg, in, args, depth+1)
				if e != nil {
					err = e
					return false
				}
				inner[h.params[i]] = parenthesize(arg, text)
			}
			text, e := c.expand(h.value, h.in, inner, depth+1)
			if e != nil {
				err = e
				return false
			}
			edits = append(edits, edit{offset(t.Pos()), offset(t.End()), parenthesize(h.value, text)})
			return false
		case *ast.Ident:
			if skip[t] {
				break
			}
			if text, ok := args[t.Name]; ok {
				edits = append(edits, edit{offset(t.Pos()), offset(t.End()), text})
				break
			}
			if h := c.it.helpers[t.Name]; h != nil && !h.isFunc {
				text, e := c.expand(h.value, h.in, nil, depth+1)
				if e != nil {
					err = e
					return false
				}
				edits = append(edits, edit{offset(t.Pos()), offset(t.End()), parenthesize(h.value, text)})
			}
		}
		return true
	})
	if err != nil {
		return "", err
	}
	start := offset(expr.Pos())
	text := string(in.src[start:offset(expr.End())])
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		text = text[:e.start-start] + e.text + text[e.end-start:]
	}
	return text, nil
}

// parenthesize returns the text of an operator expression in parentheses,
// so it keeps its meaning in the place of a name.
func parenthesize(expr ast.Expr, text string) string {
	if _, ok := expr.(*ast.BinaryExpr); ok {
		return "(" + text + ")"
	}
	return text
}

// markupNode is one node of generated .gx markup: text, an expression or an
// element.
type markupNode struct {
	text string
	expr string
	el   *markupElement
}

// markupElement is a component tag or a named slot.
type markupElement struct {
	name     string
	attrs    []string
	children []markupNode
}

// tag converts one prop literal to a component tag. Scalar props become
// attributes, Children becomes child content and every other gx.Node prop
// becomes a named slot.
func (c *converter) tag(qual, importPath, name string, file *compiler.File, lit *ast.CompositeLit) (*markupElement, error) {
	props := map[string]compiler.Field{}
	for _, f := range file.Props {
		props[f.Name] = f
	}
	el := &markupElement{name: qual + "." + name}
	c.imports[qual] = importPath
	provided := map[string]bool{}
	var slots []markupNode
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return nil, errors.New("a prop literal has a field without a name")
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil, errors.New("a prop literal has a field without a name")
		}
		prop, ok := props[key.Name]
		if !ok {
			return nil, fmt.Errorf("the props block has no field %s", key.Name)
		}
		attr := lowerFirst(key.Name)
		typ := strings.TrimSpace(prop.Type)
		switch {
		case typ == "gx.Node":
			kids, err := c.nodes(kv.Value)
			if err != nil {
				return nil, err
			}
			if len(kids) == 0 {
				continue
			}
			if key.Name == "Children" {
				el.children = kids
			} else {
				slots = append(slots, markupNode{el: &markupElement{name: ":" + attr, children: kids}})
			}
		case typ == "string" && plainString(kv.Value) != nil:
			el.attrs = append(el.attrs, attr+`="`+*plainString(kv.Value)+`"`)
		case typ == "bool" && isIdent(kv.Value, "true"):
			el.attrs = append(el.attrs, attr)
		case typ == "gx.Code":
			// Markup sets a gx.Code only from a file of the app: the
			// compiler takes a constant gx.CodeFile call here and
			// nothing else (GX8004).
			file := codeFile(kv.Value)
			if file == "" {
				return nil, fmt.Errorf("the prop %s is a gx.Code that names no file", key.Name)
			}
			el.attrs = append(el.attrs, attr+"={gx.CodeFile("+strconv.Quote(file)+`, "")}`)
		default:
			expr, err := c.qualify(kv.Value)
			if err != nil {
				return nil, err
			}
			el.attrs = append(el.attrs, attr+"={"+expr+"}")
		}
		provided[key.Name] = true
	}
	// A required prop that the fixture leaves at its zero value is written
	// out, because the tag form does not compile without it.
	for _, prop := range file.Props {
		if prop.HasDefault || provided[prop.Name] {
			continue
		}
		attr := lowerFirst(prop.Name)
		typ := strings.TrimSpace(prop.Type)
		switch {
		case typ == "string":
			el.attrs = append(el.attrs, attr+`=""`)
		case typ == "gx.URL":
			el.attrs = append(el.attrs, attr+`={""}`)
		case typ == "bool":
			el.attrs = append(el.attrs, attr+"={false}")
		case typ == "int" || typ == "int64" || typ == "float64":
			el.attrs = append(el.attrs, attr+"={0}")
		case typ == "gx.Node":
			el.attrs = append(el.attrs, attr+"={gx.Frag()}")
		case typ == "gx.Attrs":
			el.attrs = append(el.attrs, attr+"={gx.Attrs{}}")
		case typedNil(qual, typ) != "":
			el.attrs = append(el.attrs, attr+"={"+typedNil(qual, typ)+"}")
		default:
			return nil, fmt.Errorf("the fixture leaves the required prop %s unset", prop.Name)
		}
	}
	el.children = append(slots, el.children...)
	return el, nil
}

// nodes converts a gx.Node expression to markup: gx.Text is text, gx.Frag
// is its parts side by side, a component call is a tag. Any other
// expression stays a Go expression.
func (c *converter) nodes(expr ast.Expr) ([]markupNode, error) {
	if isIdent(expr, "nil") {
		return nil, nil
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		switch {
		case isGxCall(call, "Text") && len(call.Args) == 1:
			if s := plainString(call.Args[0]); s != nil && plainText(*s) {
				return []markupNode{{text: *s}}, nil
			}
		case isGxCall(call, "Frag") && !call.Ellipsis.IsValid():
			var out []markupNode
			for _, arg := range call.Args {
				kids, err := c.nodes(arg)
				if err != nil {
					return nil, err
				}
				out = append(out, kids...)
			}
			return out, nil
		case isGxCall(call, "El") && !call.Ellipsis.IsValid():
			if el := c.htmlElement(call); el != nil {
				return []markupNode{{el: el}}, nil
			}
		default:
			if el := c.componentCall(call); el != nil {
				return []markupNode{{el: el}}, nil
			}
		}
	}
	code, err := c.qualify(expr)
	if err != nil {
		return nil, err
	}
	return []markupNode{{expr: code}}, nil
}

// toastAction returns the action code that pushes a gx.ToastPatch literal:
// the c.Toast call with one option per field. It returns "" for a value it
// cannot write as options.
func (c *converter) toastAction(expr ast.Expr) string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || !isGxSelector(lit.Type, "ToastPatch") {
		return ""
	}
	text := `""`
	var opts []string
	src := func(e ast.Expr) string { return string(c.in.src[c.offset(e.Pos()):c.offset(e.End())]) }
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return ""
		}
		key, _ := kv.Key.(*ast.Ident)
		switch {
		case key == nil:
			return ""
		case key.Name == "Text":
			text = src(kv.Value)
		case key.Name == "Kind":
			opts = append(opts, src(kv.Value))
		case key.Name == "Description":
			opts = append(opts, "gx.ToastDescription("+src(kv.Value)+")")
		case key.Name == "ID":
			opts = append(opts, "gx.ToastID("+src(kv.Value)+")")
		case key.Name == "Duration":
			opts = append(opts, "gx.ToastDuration("+src(kv.Value)+")")
		case key.Name == "Sticky" && isIdent(kv.Value, "true"):
			opts = append(opts, "gx.ToastSticky")
		case key.Name == "Action":
			link := toastLink(kv.Value)
			if link == "" {
				return ""
			}
			opts = append(opts, link)
		default:
			return ""
		}
	}
	if len(opts) < 2 {
		return "return c.Toast(" + strings.Join(append([]string{text}, opts...), ", ") + ")"
	}
	return "return c.Toast(" + text + ",\n\t" + strings.Join(opts, ",\n\t") + ")"
}

// toastLink returns the gx.ToastLink option of a gx.ToastAction literal. An
// action links to a route value, not to a path: the path "/cart" gives
// route.Cart{} and "/" gives route.Home{}.
func toastLink(expr ast.Expr) string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || !isGxSelector(lit.Type, "ToastAction") {
		return ""
	}
	var label, url *string
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return ""
		}
		switch {
		case isIdent(kv.Key, "Label"):
			label = plainString(kv.Value)
		case isIdent(kv.Key, "URL"):
			url = plainString(kv.Value)
		}
	}
	if label == nil || url == nil {
		return ""
	}
	name := "Home"
	if words := camelWords(path.Base(*url)); len(words) > 0 {
		name = ""
		for _, w := range words {
			name += strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return "gx.ToastLink(" + strconv.Quote(*label) + ", route." + name + "{})"
}

// typedNil returns the nil value of a pointer or slice type of the package
// qual, as code outside that package: "*NavItem" gives
// "(*shell.NavItem)(nil)". A bare nil has no type in an attribute. It
// returns "" for any other type.
func typedNil(qual, typ string) string {
	prefix := "*"
	if strings.HasPrefix(typ, "[]") {
		prefix = "[]"
	}
	name, ok := strings.CutPrefix(typ, prefix)
	if !ok || !token.IsIdentifier(name) {
		return ""
	}
	if ast.IsExported(name) {
		name = qual + "." + name
	} else if !universe[name] {
		return ""
	}
	if prefix == "*" {
		return "(*" + name + ")(nil)"
	}
	return "[]" + name + "(nil)"
}

// codeFile returns the File of a gx.Code literal, or "".
func codeFile(expr ast.Expr) string {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || !isGxSelector(lit.Type, "Code") {
		return ""
	}
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok && isIdent(kv.Key, "File") {
			if s := plainString(kv.Value); s != nil {
				return *s
			}
		}
	}
	return ""
}

// htmlElement converts gx.El("name", attrs, children...) to an HTML element
// when the name is a literal and every attribute is a literal text or true
// boolean attribute. It returns nil for any other call.
func (c *converter) htmlElement(call *ast.CallExpr) *markupElement {
	if len(call.Args) < 2 {
		return nil
	}
	name := plainString(call.Args[0])
	if name == nil || !htmlName.MatchString(*name) || rawTextElement[*name] {
		return nil
	}
	el := &markupElement{name: *name}
	if !isIdent(call.Args[1], "nil") {
		lit, ok := call.Args[1].(*ast.CompositeLit)
		if !ok || !isGxSelector(lit.Type, "Attrs") {
			return nil
		}
		for _, elt := range lit.Elts {
			attr, ok := htmlAttr(elt)
			if !ok {
				return nil
			}
			el.attrs = append(el.attrs, attr)
		}
	}
	saved := maps.Clone(c.imports)
	for _, arg := range call.Args[2:] {
		kids, err := c.nodes(arg)
		if err != nil {
			c.imports = saved
			return nil
		}
		el.children = append(el.children, kids...)
	}
	return el
}

// htmlName matches the name of an HTML element or attribute that markup
// writes as it is.
var htmlName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// rawTextElement names the elements whose body is not markup.
var rawTextElement = map[string]bool{"script": true, "style": true}

// htmlAttr converts one element of a gx.Attrs literal to an attribute:
// {Key: "k", Value: "v"} with an optional Kind: gx.AttrText, or
// gx.Bool("k", true).
func htmlAttr(elt ast.Expr) (string, bool) {
	if call, ok := elt.(*ast.CallExpr); ok {
		if !isGxCall(call, "Bool") || len(call.Args) != 2 || !isIdent(call.Args[1], "true") {
			return "", false
		}
		key := plainString(call.Args[0])
		if key == nil || !htmlName.MatchString(*key) {
			return "", false
		}
		return *key, true
	}
	lit, ok := elt.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	var key, value *string
	for _, field := range lit.Elts {
		kv, ok := field.(*ast.KeyValueExpr)
		if !ok {
			return "", false
		}
		switch {
		case isIdent(kv.Key, "Key"):
			key = plainString(kv.Value)
		case isIdent(kv.Key, "Value"):
			value = plainString(kv.Value)
		case isIdent(kv.Key, "Kind") && isGxSelector(kv.Value, "AttrText"):
		default:
			return "", false
		}
	}
	if key == nil || value == nil || !htmlName.MatchString(*key) {
		return "", false
	}
	return *key + `="` + *value + `"`, true
}

// isGxSelector reports whether expr is gx.<name>.
func isGxSelector(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name && isIdent(sel.X, "gx")
}

// componentCall converts Comp(CompProps{...}) or pkg.Comp(pkg.CompProps{...})
// to a tag when Comp is a .gx component of the repository. It returns nil
// for any other call.
func (c *converter) componentCall(call *ast.CallExpr) *markupElement {
	if len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return nil
	}
	lit, ok := call.Args[0].(*ast.CompositeLit)
	if !ok {
		return nil
	}
	qual, importPath, dir, name := c.it.PkgName, c.it.PkgPath, c.it.Dir, ""
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		typ, ok := lit.Type.(*ast.Ident)
		if !ok || typ.Name != fun.Name+"Props" {
			return nil
		}
		name = fun.Name
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok {
			return nil
		}
		typ, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || !isIdent(typ.X, pkg.Name) || typ.Sel.Name != fun.Sel.Name+"Props" {
			return nil
		}
		p, ok := c.fileImports[pkg.Name]
		if !ok {
			return nil
		}
		d, ok := c.reg.packageDir(p)
		if !ok {
			return nil
		}
		qual, importPath, dir, name = pkg.Name, p, d, fun.Sel.Name
	default:
		return nil
	}
	if !ast.IsExported(name) {
		return nil
	}
	file := c.reg.componentFile(dir, name)
	if file == nil {
		return nil
	}
	saved := maps.Clone(c.imports)
	el, err := c.tag(qual, importPath, name, file, lit)
	if err != nil {
		c.imports = saved
		return nil
	}
	return el
}

// universe holds the predeclared Go identifiers an expression may use.
var universe = map[string]bool{
	"true": true, "false": true, "nil": true, "iota": true,
	"any": true, "bool": true, "byte": true, "error": true, "rune": true, "string": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"append": true, "cap": true, "len": true, "make": true, "max": true, "min": true, "new": true,
}

// qualify returns the source of expr as code outside the item package: a
// package-level name gets the package qualifier. It fails for a function
// literal, an unexported name and any name it cannot place, so the caller
// falls back instead of writing wrong code.
func (c *converter) qualify(expr ast.Expr) (string, error) {
	var inserts []int
	var err error
	fail := func(format string, args ...any) {
		if err == nil {
			err = fmt.Errorf(format, args...)
		}
	}
	var walk func(n ast.Node)
	walkAll := func(list []ast.Expr) {
		for _, e := range list {
			walk(e)
		}
	}
	walk = func(n ast.Node) {
		switch t := n.(type) {
		case nil, *ast.BasicLit:
		case *ast.Ident:
			switch {
			case universe[t.Name]:
			case c.it.names[t.Name] && ast.IsExported(t.Name):
				inserts = append(inserts, c.offset(t.Pos()))
				c.imports[c.it.PkgName] = c.it.PkgPath
			case c.it.names[t.Name]:
				fail("the fixture uses the unexported name %s", t.Name)
			default:
				fail("the fixture uses the name %s, which the converter cannot place", t.Name)
			}
		case *ast.SelectorExpr:
			if pkg, ok := t.X.(*ast.Ident); ok && !c.it.names[pkg.Name] {
				if p, ok := c.fileImports[pkg.Name]; ok {
					c.imports[pkg.Name] = p
					return
				}
			}
			walk(t.X)
		case *ast.CompositeLit:
			if t.Type != nil {
				walk(t.Type)
			}
			_, isMap := t.Type.(*ast.MapType)
			for _, elt := range t.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					walk(elt)
					continue
				}
				// An identifier key names a struct field, except in a
				// map literal.
				if _, field := kv.Key.(*ast.Ident); !field || isMap {
					walk(kv.Key)
				}
				walk(kv.Value)
			}
		case *ast.CallExpr:
			walk(t.Fun)
			walkAll(t.Args)
		case *ast.ParenExpr:
			walk(t.X)
		case *ast.UnaryExpr:
			walk(t.X)
		case *ast.StarExpr:
			walk(t.X)
		case *ast.BinaryExpr:
			walk(t.X)
			walk(t.Y)
		case *ast.IndexExpr:
			walk(t.X)
			walk(t.Index)
		case *ast.IndexListExpr:
			walk(t.X)
			walkAll(t.Indices)
		case *ast.ArrayType:
			if t.Len != nil {
				walk(t.Len)
			}
			walk(t.Elt)
		case *ast.MapType:
			walk(t.Key)
			walk(t.Value)
		case *ast.FuncLit:
			fail("the fixture holds a function literal")
		default:
			fail("the fixture holds a %T expression", n)
		}
	}
	walk(expr)
	if err != nil {
		return "", err
	}
	start, end := c.offset(expr.Pos()), c.offset(expr.End())
	src := string(c.in.src[start:end])
	sort.Sort(sort.Reverse(sort.IntSlice(inserts)))
	for _, at := range inserts {
		at -= start
		src = src[:at] + c.it.PkgName + "." + src[at:]
	}
	return formatExpr(src)
}

// offset returns the byte offset of a position in the text the converter
// reads.
func (c *converter) offset(pos token.Pos) int { return c.in.fset.Position(pos).Offset }

// formatExpr prints a Go expression in gofmt form with two-space
// indentation, the indentation of .gx files.
func formatExpr(src string) (string, error) {
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "", src, 0)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := (&printer.Config{Mode: printer.UseSpaces, Tabwidth: 2}).Fprint(&b, fset, expr); err != nil {
		return "", err
	}
	return b.String(), nil
}

// plainString returns the value of a constant string that fits a quoted
// attribute: no quote, brace or line break. A sum of string literals is one
// constant, which a helper with its arguments in place gives.
func plainString(expr ast.Expr) *string {
	s, ok := constString(expr)
	if !ok || strings.ContainsAny(s, "\"{}\n\r\t\\") {
		return nil
	}
	return &s
}

// constString returns the value of a string literal or a sum of them.
func constString(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.BasicLit:
		if t.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(t.Value)
		return s, err == nil
	case *ast.ParenExpr:
		return constString(t.X)
	case *ast.BinaryExpr:
		if t.Op != token.ADD {
			return "", false
		}
		x, okX := constString(t.X)
		y, okY := constString(t.Y)
		return x + y, okX && okY
	}
	return "", false
}

// plainText reports whether a string is safe as literal markup text: the
// parser reads it back as the same text.
func plainText(s string) bool {
	if s == "" || strings.TrimSpace(s) != s {
		return false
	}
	return !strings.ContainsAny(s, "<>{}&") && !strings.Contains(s, ":=")
}

// isIdent reports whether expr is the identifier name.
func isIdent(expr ast.Expr, name string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == name
}

// isGxCall reports whether call is gx.<name>(...).
func isGxCall(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name && isIdent(sel.X, "gx")
}

// lowerFirst returns the attribute name of a prop: "Variant" gives
// "variant".
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// lineLimit is the longest line a one-line element may take.
const lineLimit = 120

// writeNode writes one node at the given depth, with two-space
// indentation.
func writeNode(b *strings.Builder, n markupNode, depth int) {
	pad := strings.Repeat("  ", depth)
	if n.el == nil {
		b.WriteString(pad + indentRest(inlineNode(n), pad) + "\n")
		return
	}
	if line, ok := inlineElement(n.el); ok && len(pad)+len(line) <= lineLimit {
		b.WriteString(pad + line + "\n")
		return
	}
	b.WriteString(pad + indentRest(openTag(n.el), pad))
	if len(n.el.children) == 0 {
		b.WriteString(" />\n")
		return
	}
	b.WriteString(">\n")
	for _, child := range n.el.children {
		writeNode(b, child, depth+1)
	}
	b.WriteString(pad + "</" + n.el.name + ">\n")
}

// openTag returns "<name attrs" without the closing bracket.
func openTag(el *markupElement) string {
	if len(el.attrs) == 0 {
		return "<" + el.name
	}
	return "<" + el.name + " " + strings.Join(el.attrs, " ")
}

// inlineNode returns the text or the expression of a node.
func inlineNode(n markupNode) string {
	if n.expr != "" {
		return "{" + n.expr + "}"
	}
	return n.text
}

// inlineElement returns the one-line form of an element with at most one
// child.
func inlineElement(el *markupElement) (string, bool) {
	open := openTag(el)
	if strings.Contains(open, "\n") || len(el.children) > 1 {
		return "", false
	}
	if len(el.children) == 0 {
		return open + " />", true
	}
	child := el.children[0]
	inner := inlineNode(child)
	if child.el != nil {
		line, ok := inlineElement(child.el)
		if !ok {
			return "", false
		}
		inner = line
	}
	if strings.Contains(inner, "\n") {
		return "", false
	}
	return open + ">" + inner + "</" + el.name + ">", true
}

// indentRest indents every line of s after the first.
func indentRest(s, pad string) string {
	return strings.ReplaceAll(s, "\n", "\n"+pad)
}
