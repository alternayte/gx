package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// typesResult holds the Go types of .gx expressions from one analysis pass.
type typesResult struct {
	types     map[any]types.Type
	quals     map[*File]map[int]map[string]bool // default identifiers owned by the declaring package
	nodeIface *types.Interface
	errIface  *types.Interface
	stringer  *types.Interface
}

// synthRef maps a synthetic probe file name to the .gx position to report.
type synthRef struct {
	file *File
	line int // 0 means keep the reported line and column
	col  int
}

type probe struct {
	file  *File
	synth string
	sites []any          // emission order of {expr} and expression attributes
	defs  map[int]string // props field index -> synthetic default probe name
}

// analyze type-checks every .gx file in dirs. It generates shadow Go files in
// memory and never writes them to disk.
func (l *loader) analyze(root string, dirs []string) (*typesResult, []Diagnostic) {
	res := &typesResult{types: map[any]types.Type{}, quals: map[*File]map[int]map[string]bool{}}
	if findModule(root) == nil {
		return res, nil
	}
	refs := map[string]synthRef{}
	parseProbe := map[string]*probe{}
	overlay := map[string][]byte{}
	i := 0
	for _, dir := range dirs {
		p := l.load(dir)
		for base, f := range p.Files {
			if f.Package == "" || f.File == "" {
				continue
			}
			synth := fmt.Sprintf("gx%d", i)
			i++
			refs[synth] = synthRef{file: f}
			pr, src := buildProbe(l, p, f, synth)
			parseProbe[filepath.Join(dir, base+"_gx.go")] = pr
			overlay[filepath.Join(dir, base+"_gx.go")] = src
			for idx, name := range pr.defs {
				fld := f.Props[idx]
				refs[name] = synthRef{file: f, line: fld.At.Line, col: fld.At.Col}
			}
		}
	}
	if len(overlay) == 0 {
		return res, nil
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:     root,
		Overlay: overlay,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return res, nil
	}
	res.fillInterfaces(pkgs)

	var diags []Diagnostic
	for _, pkg := range pkgs {
		for _, e := range pkg.TypeErrors {
			pos := pkg.Fset.Position(e.Pos)
			ref, ok := refs[filepath.Base(pos.Filename)]
			if !ok {
				continue
			}
			line, col := pos.Line, pos.Column
			if ref.line > 0 {
				line, col = ref.line, ref.col
			}
			diags = append(diags, Diagnostic{
				Code: CodeType,
				File: ref.file.File,
				Line: line,
				Col:  col,
				Msg:  e.Msg,
			})
		}
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			pr, ok := parseProbe[path]
			if !ok {
				continue
			}
			l.collectTypes(res, pkg, file, pr)
		}
	}
	diags = append(diags, l.checkClassConditions(res, dirs)...)
	return res, diags
}

// checkClassConditions reports a class:<name> directive whose expression is
// not a bool (REQ-AUT-10).
func (l *loader) checkClassConditions(res *typesResult, dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkElements(f.Body, func(el *Element) {
				for i := range el.Attrs {
					a := &el.Attrs[i]
					if a.Kind != AttrExpr || !strings.HasPrefix(a.Name, "class:") {
						continue
					}
					if strings.Contains(a.Value, "$") {
						continue
					}
					t := res.types[a]
					if t == nil || t.String() == "bool" {
						continue
					}
					out = append(out, Diagnostic{
						Code: CodeType,
						File: f.File,
						Line: a.At.Line,
						Col:  a.At.Col,
						Msg:  "attribute " + Quoted(a.Name) + " needs a bool expression, got " + t.String(),
					})
				}
			})
		}
	}
	return out
}

// walkElements calls fn for every element below ns.
func walkElements(ns []Node, fn func(*Element)) {
	for _, n := range ns {
		switch t := n.(type) {
		case *Element:
			fn(t)
			walkElements(t.Children, fn)
		case *Control:
			walkElements(t.Body, fn)
			walkElements(t.Else, fn)
			for _, c := range t.Cases {
				walkElements(c.Body, fn)
			}
		}
	}
}

// collectTypes reads the types of the probe sites and the package-owned
// identifiers of every default expression.
func (l *loader) collectTypes(res *typesResult, pkg *packages.Package, file *ast.File, pr *probe) {
	var exprs []ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok && id.Name == "_" {
			exprs = append(exprs, as.Rhs[0])
		}
		return true
	})
	for i, site := range pr.sites {
		if i >= len(exprs) {
			break
		}
		if tv, ok := pkg.TypesInfo.Types[exprs[i]]; ok {
			res.types[site] = tv.Type
		}
	}
	for idx, name := range pr.defs {
		names := map[string]bool{}
		for ident, obj := range pkg.TypesInfo.Uses {
			if filepath.Base(pkg.Fset.Position(ident.Pos()).Filename) != name {
				continue
			}
			if obj.Pkg() != nil && obj.Pkg() == pkg.Types {
				names[obj.Name()] = true
			}
		}
		if len(names) == 0 {
			continue
		}
		if res.quals[pr.file] == nil {
			res.quals[pr.file] = map[int]map[string]bool{}
		}
		res.quals[pr.file][idx] = names
	}
}

// fillInterfaces finds the gx.Node interface and builds the error and
// fmt.Stringer interfaces.
func (r *typesResult) fillInterfaces(pkgs []*packages.Package) {
	seen := map[string]bool{}
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if p == nil || seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		if p.PkgPath == "github.com/alternayte/gx" && p.Types != nil {
			if obj := p.Types.Scope().Lookup("Node"); obj != nil {
				r.nodeIface, _ = obj.Type().Underlying().(*types.Interface)
			}
		}
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
	r.errIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	str := types.NewInterfaceType([]*types.Func{
		types.NewFunc(token.NoPos, nil, "String", types.NewSignatureType(
			nil, nil, nil, nil,
			types.NewTuple(types.NewParam(token.NoPos, nil, "", types.Typ[types.String])),
			false)),
	}, nil)
	str.Complete()
	r.stringer = str
}

// renderable reports whether a value of type t can become text.
func (r *typesResult) renderable(t types.Type) bool {
	if t == nil {
		return false
	}
	if b, ok := t.Underlying().(*types.Basic); ok {
		info := b.Info()
		return info&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsString) != 0
	}
	if r.errIface != nil && types.Implements(t, r.errIface) {
		return true
	}
	return r.stringer != nil && types.Implements(t, r.stringer)
}

// isNode reports whether a value of type t is a gx.Node.
func (r *typesResult) isNode(t types.Type) bool {
	return r.nodeIface != nil && t != nil && types.Implements(t, r.nodeIface)
}

// buildProbe returns a shadow Go file for f. The file declares the props
// type and the component function so that callers type-check, and probes
// every server expression and default. It is never written to disk.
func buildProbe(l *loader, pkg *Package, f *File, synth string) (*probe, []byte) {
	pr := &probe{file: f, synth: synth, defs: map[int]string{}}
	var b bytes.Buffer
	b.WriteString("// Code generated by gx for type checking. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", f.Package)
	b.WriteString("import (\n")
	b.WriteString("\tgx \"github.com/alternayte/gx\"\n")
	for _, im := range f.Imports {
		if importPath(im.Raw) == "github.com/alternayte/gx" {
			continue
		}
		fmt.Fprintf(&b, "\t%s\n", im.Raw)
	}
	b.WriteString(")\n\n")
	name := componentName(f)
	fmt.Fprintf(&b, "type %sProps struct {\n", name)
	for _, fld := range f.Props {
		lineDirective(&b, synth, fld.At)
		fmt.Fprintf(&b, "%s %s\n", fld.Name, fld.Type)
	}
	b.WriteString("}\n\n")
	fmt.Fprintf(&b, "func %s(p %sProps) gx.Node { return nil }\n\n", name, name)
	w := &probeWriter{l: l, pkg: pkg, file: f, synth: synth, pr: pr, b: &b}
	fmt.Fprintf(&b, "func _gxProbe%s(p %sProps) {\n", name, name)
	for idx, fld := range f.Props {
		if !fld.HasDefault {
			continue
		}
		ds := fmt.Sprintf("%s-d%d", synth, idx)
		pr.defs[idx] = ds
		fmt.Fprintf(&b, "//line %s:1:1\nvar _ %s = %s\n", ds, fld.Type, fld.Default)
	}
	w.nodes(f.Body)
	b.WriteString("}\n")
	return pr, b.Bytes()
}

type probeWriter struct {
	l     *loader
	pkg   *Package
	file  *File
	synth string
	pr    *probe
	b     *bytes.Buffer
}

func (w *probeWriter) nodes(ns []Node) {
	for _, n := range ns {
		switch t := n.(type) {
		case *Expr:
			writeProbeExpr(w.b, t.Data, t.DataAt, w.synth, w.pr, t)
		case *Element:
			for i := range t.Attrs {
				a := &t.Attrs[i]
				switch a.Kind {
				case AttrExpr:
					writeProbeExpr(w.b, a.Value, a.ValueAt, w.synth, w.pr, a)
				case AttrSpread:
					writeProbeSpread(w.b, a.Value, a.ValueAt, w.synth)
				}
			}
			if t.HasRaw {
				continue
			}
			if w.slots(t) {
				continue
			}
			w.nodes(t.Children)
		case *Let:
			if strings.Contains(t.Expr, "$") {
				continue
			}
			lineDirective(w.b, w.synth, t.At)
			fmt.Fprintf(w.b, "%s := %s\n", t.Name, t.Expr)
		case *Control:
			w.control(t)
		}
	}
}

// slots probes the children of a component element. A gx.Slot[T] slot binds
// its let value with type T, so its body is checked in that scope.
func (w *probeWriter) slots(el *Element) bool {
	qual, name, ok := componentTag(el.Name)
	if !ok {
		return false
	}
	comp, _, _ := resolveComponent(w.l, w.pkg, w.file, qual, name)
	if comp == nil {
		return false
	}
	var rest []Node
	for _, child := range el.Children {
		slotEl, ok := child.(*Element)
		if ok && strings.HasPrefix(slotEl.Name, ":") {
			if prop, found := findProp(comp, strings.TrimPrefix(slotEl.Name, ":")); found {
				if elem, ok := slotTypeArg(prop.Type); ok {
					fmt.Fprintf(w.b, "var _ = func(%s %s) {\n", slotLetName(slotEl), elem)
					w.nodes(slotEl.Children)
					fmt.Fprintf(w.b, "}\n")
					continue
				}
			}
		}
		rest = append(rest, child)
	}
	w.nodes(rest)
	return true
}

func (w *probeWriter) control(c *Control) {
	lineDirective(w.b, w.synth, c.At)
	switch c.Kind {
	case "if":
		fmt.Fprintf(w.b, "if %s {\n", c.Header)
		w.nodes(c.Body)
		w.b.WriteString("}")
		if len(c.Else) > 0 {
			w.b.WriteString(" else {\n")
			w.nodes(c.Else)
			w.b.WriteString("}")
		}
		w.b.WriteString("\n")
	case "for":
		fmt.Fprintf(w.b, "for %s {\n", c.Header)
		w.nodes(c.Body)
		w.b.WriteString("}\n")
	case "switch":
		fmt.Fprintf(w.b, "switch %s {\n", c.Header)
		for _, cs := range c.Cases {
			if cs.IsDefault {
				w.b.WriteString("default:\n")
			} else {
				fmt.Fprintf(w.b, "case %s:\n", cs.Header)
			}
			w.nodes(cs.Body)
		}
		w.b.WriteString("}\n")
	}
}

func writeProbeExpr(b *bytes.Buffer, expr string, at Pos, synth string, pr *probe, site any) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.Contains(expr, "$") || at.Line <= 0 {
		return
	}
	pr.sites = append(pr.sites, site)
	// The prefix "_=" is two columns wide, so the expression starts at
	// the column the directive names.
	col := at.Col - 2
	if col < 1 {
		col = 1
	}
	fmt.Fprintf(b, "//line %s:%d:%d\n_=%s\n", synth, at.Line, col, expr)
}

func writeProbeSpread(b *bytes.Buffer, expr string, at Pos, synth string) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.Contains(expr, "$") || at.Line <= 0 {
		return
	}
	const prefix = "var _ gx.Attrs = "
	col := at.Col - len(prefix)
	if col < 1 {
		col = 1
	}
	fmt.Fprintf(b, "//line %s:%d:%d\n%s%s\n", synth, at.Line, col, prefix, expr)
}

func lineDirective(b *bytes.Buffer, synth string, at Pos) {
	if at.Line <= 0 {
		return
	}
	col := at.Col
	if col <= 0 {
		col = 1
	}
	fmt.Fprintf(b, "//line %s:%d:%d\n", synth, at.Line, col)
}

// componentName returns the component name of f: the file base name.
func componentName(f *File) string {
	base := filepath.Base(f.File)
	return strings.TrimSuffix(base, ".gx")
}

func importPath(raw string) string {
	_, path, _ := splitImport(raw)
	return path
}
