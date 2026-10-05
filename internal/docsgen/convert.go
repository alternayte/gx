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
	"sort"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// snippet is the example code of one fixture, in .gx syntax (REQ-DOC-04).
type snippet struct {
	Code string
	// Tag is true when the fixture converts to a component tag. Otherwise
	// Code is the fallback expression and Why says what stopped the tag
	// form.
	Tag bool
	Why string
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
	// fileImports maps the qualifiers of the fixtures file to import
	// paths; imports collects the ones the snippet uses.
	fileImports map[string]string
	imports     map[string]string
}

// newConverter prepares the conversion of one component's fixtures.
func newConverter(reg *registry, it *item, comp *component) *converter {
	c := &converter{reg: reg, it: it, comp: comp, fileImports: map[string]string{}}
	for _, spec := range comp.file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		} else if other := reg.itemByPath(p); other != nil {
			name = other.PkgName
		}
		c.fileImports[name] = p
	}
	return c
}

// itemByPath finds the registry item of an import path.
func (reg *registry) itemByPath(importPath string) *item {
	for _, it := range reg.Items {
		if it.PkgPath == importPath {
			return it
		}
	}
	return nil
}

// convert returns the example code of one fixture.
func (c *converter) convert(fx fixture) snippet {
	c.imports = map[string]string{}
	el, err := c.fixtureTag(fx)
	if err != nil {
		code := fmt.Sprintf("{%s.%s(%s.%s[%s])}", c.it.PkgName, c.comp.Name, c.it.PkgName, c.comp.FixturesVar, strconv.Quote(fx.Name))
		return snippet{Code: code, Why: err.Error(), Imports: map[string]string{c.it.PkgName: c.it.PkgPath}}
	}
	var b strings.Builder
	writeNode(&b, markupNode{el: el}, 0)
	return snippet{Code: strings.TrimRight(b.String(), "\n"), Tag: true, Imports: c.imports}
}

// fixtureTag converts the prop literal of a fixture to the tag of its
// component.
func (c *converter) fixtureTag(fx fixture) (*markupElement, error) {
	if c.comp.Gx == nil {
		return nil, errors.New("the component is a Go function, not a .gx file")
	}
	lit, ok := fx.Value.(*ast.CompositeLit)
	if !ok || lit.Type != nil {
		return nil, errors.New("the fixture is not a plain prop literal")
	}
	return c.tag(c.it.PkgName, c.it.PkgPath, c.comp.Name, c.comp.Gx, lit)
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
			// The compiler takes only a constant gx.CodeFile call of
			// a file in the app here (GX8004).
			return nil, fmt.Errorf("the prop %s is a gx.Code, which markup sets only from a file of the app", key.Name)
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
	src := string(c.comp.src[start:end])
	sort.Sort(sort.Reverse(sort.IntSlice(inserts)))
	for _, at := range inserts {
		at -= start
		src = src[:at] + c.it.PkgName + "." + src[at:]
	}
	return formatExpr(src)
}

// offset returns the byte offset of a position in the fixtures file.
func (c *converter) offset(pos token.Pos) int { return c.comp.fset.Position(pos).Offset }

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

// plainString returns the value of a string literal that fits a quoted
// attribute: no quote, brace or line break.
func plainString(expr ast.Expr) *string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || strings.ContainsAny(s, "\"{}\n\r\t\\") {
		return nil
	}
	return &s
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
