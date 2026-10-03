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
	types      map[any]types.Type
	quals      map[*File]map[int]map[string]bool // default identifiers owned by the declaring package
	nodeIface  *types.Interface
	errIface   *types.Interface
	stringer   *types.Interface
	routeFiles map[string][]byte       // generated route code, keyed by output path
	urlRoutes  map[string]bool         // package.Type of GET route structs
	routes     []*routeDef             // every route struct
	pkgs       []*packages.Package     // loaded packages
	routePages map[string]string       // package.Type of a route -> its page value
	pageRoutes map[types.Object]string // page var -> package.Type of its route
	routeKeys  map[string]bool         // package.Type of every route struct
	routeMeth  map[string]string       // package.Type of a route -> its method
	actions    map[string][]token.Position
	routeDefs  map[string]*routeDef // package.Type of a route -> its definition
	sigTypes   map[*File]map[string]types.Type
	sigActions map[string]bool // actions with signal-bound fields
}

// synthRef maps a synthetic probe file name to the .gx position to report.
type synthRef struct {
	file *File
	line int // 0 means keep the reported line and column
	col  int
	code string // default GX2000
}

type probe struct {
	file  *File
	synth string
	sites []any          // emission order of {expr} and expression attributes
	defs  map[int]string // props field index -> synthetic default probe name
	frags []string       // synthetic names of the fragment scope probes
}

// analyze type-checks every .gx file in dirs. It generates shadow Go files in
// memory and never writes them to disk.
func (l *loader) analyze(root string, dirs []string) (*typesResult, []Diagnostic) {
	res := &typesResult{
		types:      map[any]types.Type{},
		quals:      map[*File]map[int]map[string]bool{},
		routeFiles: map[string][]byte{},
		urlRoutes:  map[string]bool{},
		routePages: map[string]string{},
		pageRoutes: map[types.Object]string{},
		routeKeys:  map[string]bool{},
		routeMeth:  map[string]string{},
		actions:    map[string][]token.Position{},
		routeDefs:  map[string]*routeDef{},
		sigTypes:   map[*File]map[string]types.Type{},
		sigActions: map[string]bool{},
	}
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
			for _, fs := range pr.frags {
				refs[fs] = synthRef{file: f, code: CodeFragment}
			}
			path := filepath.Join(dir, base+"_gx.go")
			parseProbe[path] = pr
			overlay[path] = src
			for idx, name := range pr.defs {
				fld := f.Props[idx]
				refs[name] = synthRef{file: f, line: fld.At.Line, col: fld.At.Col}
			}
		}
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:     root,
		Overlay: overlay,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return res, nil
	}
	res.pkgs = pkgs
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
			code := ref.code
			if code == "" {
				code = CodeType
			}
			d := Diagnostic{
				Code: code,
				File: ref.file.File,
				Line: line,
				Col:  col,
				Msg:  e.Msg,
			}
			if code == CodeFragment {
				d.Fix = "add the undefined name to the #fragment params"
				if name, ok := strings.CutPrefix(e.Msg, "undefined: "); ok {
					d.Fix = "add " + name + " to the #fragment params"
				}
			}
			diags = append(diags, d)
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
	res.collectActions(pkgs)
	routes, rdiags := collectRoutes(pkgs, res.actions)
	res.routes = routes
	diags = append(diags, rdiags...)
	for _, d := range routes {
		key := d.pkg.PkgPath + "." + d.name
		res.routeKeys[key] = true
		res.routeDefs[key] = d
		if d.hasSignals {
			res.sigActions[key] = true
		}
		method, _, _ := strings.Cut(d.pattern, " ")
		res.routeMeth[key] = method
		if method == "GET" || method == "HEAD" {
			res.urlRoutes[key] = true
		}
	}
	res.routeFiles = renderRouteFiles(routes)
	diags = append(diags, res.checkMounted(pkgs)...)
	diags = append(diags, checkDuplicatePatterns(routes)...)
	diags = append(diags, l.checkAttributes(res, dirs)...)
	diags = append(diags, l.checkActionInvocations(res, dirs)...)
	diags = append(diags, l.checkSignals(dirs)...)
	diags = append(diags, l.checkKeys(dirs)...)
	diags = append(diags, checkSafeHTML(pkgs)...)
	diags = append(diags, checkRoutePackages(pkgs)...)
	return res, diags
}

// checkSafeHTML reports a conversion of a non-constant value to gx.SafeHTML
// unless the same line carries //gx:trusted (SI-01).
func checkSafeHTML(pkgs []*packages.Package) []Diagnostic {
	var out []Diagnostic
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			trusted := map[int]bool{}
			for _, cg := range file.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "gx:trusted") {
						trusted[pkg.Fset.Position(c.Pos()).Line] = true
					}
				}
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 1 {
					return true
				}
				var obj types.Object
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					obj = pkg.TypesInfo.Uses[fun]
				case *ast.SelectorExpr:
					obj = pkg.TypesInfo.Uses[fun.Sel]
				}
				if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "github.com/alternayte/gx" || obj.Name() != "SafeHTML" {
					return true
				}
				if tv, ok := pkg.TypesInfo.Types[call.Args[0]]; ok && tv.Value != nil {
					return true // a constant string is trusted
				}
				pos := pkg.Fset.Position(call.Pos())
				if trusted[pos.Line] {
					return true
				}
				out = append(out, Diagnostic{
					Code: CodeTrustedHTML,
					File: pos.Filename,
					Line: pos.Line,
					Col:  pos.Column,
					Msg:  "conversion to gx.SafeHTML needs //gx:trusted <reason>",
					Fix:  "add //gx:trusted <reason> on the same line",
				})
				return true
			})
		}
	}
	return out
}

// checkKeys reports a loop that needs a key and has none (REQ-AUT-14).
func (l *loader) checkKeys(dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkNodes(f.Body, func(n Node) {
				c, ok := n.(*Control)
				if !ok || c.Kind != "for" {
					return
				}
				if !loopNeedsKey(l, p, f, c.Body) || loopHasKey(c.Body) {
					return
				}
				out = append(out, Diagnostic{
					Code: CodeLoopKey,
					File: f.File,
					Line: c.At.Line,
					Col:  c.At.Col,
					Msg:  "loop needs a key: add key={expr} or a #fragment with a key parameter",
				})
			})
		}
	}
	return out
}

// loopNeedsKey reports whether the loop body holds a node that keeps client
// state: a form control or a component with signals (REQ-AUT-14).
func loopNeedsKey(l *loader, p *Package, f *File, ns []Node) bool {
	needs := false
	walkElements(ns, func(el *Element) {
		switch strings.ToLower(el.Name) {
		case "input", "select", "textarea":
			needs = true
			return
		}
		if qual, name, ok := componentTag(el.Name); ok {
			if comp, _, _ := resolveComponent(l, p, f, qual, name); comp != nil && len(comp.File.Signals) > 0 {
				needs = true
			}
		}
	})
	return needs
}

// loopHasKey reports whether the loop body carries key={expr} or a fragment
// whose first parameter is key.
func loopHasKey(ns []Node) bool {
	found := false
	walkElements(ns, func(el *Element) {
		for i := range el.Attrs {
			a := &el.Attrs[i]
			if a.Kind == AttrExpr && a.Name == "key" {
				found = true
			}
			if a.Kind == AttrFragment && firstIdent(strings.TrimSpace(a.Value)) == "key" {
				found = true
			}
		}
	})
	return found
}

// checkSignals reports a server expression that reads a signal (REQ-AUT-15).
func (l *loader) checkSignals(dirs []string) []Diagnostic {
	var out []Diagnostic
	report := func(f *File, n Node) {
		switch t := n.(type) {
		case *Expr:
			if strings.Contains(t.Data, "$") {
				out = append(out, Diagnostic{Code: CodeSignal, File: f.File, Line: t.At.Line, Col: t.At.Col, Msg: "a signal is only valid in a client expression"})
			}
		case *Let:
			if strings.Contains(t.Expr, "$") {
				out = append(out, Diagnostic{Code: CodeSignal, File: f.File, Line: t.At.Line, Col: t.At.Col, Msg: "a signal is only valid in a client expression"})
			}
		case *Control:
			if strings.Contains(t.Header, "$") {
				out = append(out, Diagnostic{Code: CodeSignal, File: f.File, Line: t.At.Line, Col: t.At.Col, Msg: "a signal is only valid in a client expression"})
			}
		}
	}
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkNodes(f.Body, func(n Node) { report(f, n) })
		}
	}
	return out
}

// walkNodes calls fn for every body node below ns.
func walkNodes(ns []Node, fn func(Node)) {
	for _, n := range ns {
		fn(n)
		switch t := n.(type) {
		case *Element:
			walkNodes(t.Children, fn)
		case *Control:
			walkNodes(t.Body, fn)
			walkNodes(t.Else, fn)
			for _, c := range t.Cases {
				walkNodes(c.Body, fn)
			}
		}
	}
}

// isClientDirective reports whether an attribute holds a client expression
// (REQ-ACT-07).
func isClientDirective(name string) bool {
	if name == "show" || name == "text" {
		return true
	}
	for _, prefix := range []string{"bind:", "class:", "attr:", "on:"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// checkAttributes reports context mistakes the probe cannot see: a class
// directive that is not a bool (REQ-AUT-10), a dynamic event attribute
// (REQ-AUT-12), a dynamic URL attribute (SI-02) and a non-gx.Style style
// (REQ-AUT-12).
func (l *loader) checkAttributes(res *typesResult, dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkElements(f.Body, func(el *Element) {
				if _, _, isComp := componentTag(el.Name); isComp {
					return // component attributes are props
				}
				for i := range el.Attrs {
					a := &el.Attrs[i]
					if a.Kind != AttrExpr {
						continue
					}
					if strings.Contains(a.Value, "$") && !isClientDirective(a.Name) {
						out = append(out, Diagnostic{
							Code: CodeSignal,
							File: f.File,
							Line: a.At.Line,
							Col:  a.At.Col,
							Msg:  "a signal is only valid in a client expression",
						})
						continue
					}
					if _, ok := strings.CutPrefix(a.Name, "class:"); ok {
						if !strings.Contains(a.Value, "$") {
							if t := res.types[a]; t != nil && t.String() != "bool" {
								out = append(out, Diagnostic{
									Code: CodeType,
									File: f.File,
									Line: a.At.Line,
									Col:  a.At.Col,
									Msg:  "attribute " + Quoted(a.Name) + " needs a bool expression, got " + t.String(),
								})
							}
						}
						continue
					}
					if isDirective(a.Name) {
						continue
					}
					if strings.HasPrefix(a.Name, "on") {
						out = append(out, Diagnostic{
							Code: CodeEventAttr,
							File: f.File,
							Line: a.At.Line,
							Col:  a.At.Col,
							Msg:  "attribute " + Quoted(a.Name) + " cannot take an expression",
						})
						continue
					}
					if isURLAttr(a.Name) {
						if !res.isURLValue(res.types[a]) {
							out = append(out, Diagnostic{
								Code: CodeURLAttr,
								File: f.File,
								Line: a.At.Line,
								Col:  a.At.Col,
								Msg:  "attribute " + Quoted(a.Name) + " needs a typed route or gx.URL, not a dynamic string",
							})
						}
						continue
					}
					if a.Name == "style" {
						if t := res.types[a]; t == nil || t.String() != "github.com/alternayte/gx.Style" {
							out = append(out, Diagnostic{
								Code: CodeType,
								File: f.File,
								Line: a.At.Line,
								Col:  a.At.Col,
								Msg:  "attribute \"style\" needs a gx.Style value",
							})
						}
					}
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
		if gd, ok := n.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			l.collectSignalTypes(res, pkg, pr.file, gd)
			return true
		}
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

// collectSignalTypes records the Go type of every gx signal declaration in
// a probe file, keyed by the lower-first signal name (REQ-ACT-03).
func (l *loader) collectSignalTypes(res *typesResult, pkg *packages.Package, f *File, gd *ast.GenDecl) {
	for _, spec := range gd.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, id := range vs.Names {
			name, ok := strings.CutPrefix(id.Name, "_gxSig_")
			if !ok {
				continue
			}
			obj, _ := pkg.TypesInfo.Defs[id].(*types.Var)
			if obj == nil {
				continue
			}
			if res.sigTypes[f] == nil {
				res.sigTypes[f] = map[string]types.Type{}
			}
			res.sigTypes[f][lowerFirst(name)] = obj.Type()
		}
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

// isURLValue reports whether a value of type t can stand in an href, src,
// action or formaction attribute (REQ-RTE-05). A route type that is not a
// GET route never qualifies, even though every route has a URL method.
func (r *typesResult) isURLValue(t types.Type) bool {
	if t == nil {
		return false
	}
	if t.String() == "github.com/alternayte/gx.URL" {
		return true
	}
	if _, ok := t.(*types.Named); ok {
		if key := namedTypeKey(t); key != "" && r.routeKeys[key] {
			return r.urlRoutes[key]
		}
	}
	sel := types.NewMethodSet(t).Lookup(nil, "URL")
	if sel == nil {
		return false
	}
	sig, ok := sel.Obj().Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	return sig.Results().At(0).Type().String() == "string"
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
	w := &probeWriter{l: l, pkg: pkg, file: f, synth: synth, pr: pr, b: &b, record: true}
	fmt.Fprintf(&b, "func _gxProbe%s(p %sProps) {\n", name, name)
	writeSignalDecls(&b, synth, f, true)
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
	for i, el := range fragmentElements(f.Body) {
		fragSynth := fmt.Sprintf("%s-f%d", synth, i)
		pr.frags = append(pr.frags, fragSynth)
		fmt.Fprintf(&b, "\n//line %s:%d:1\nfunc _gxFrag%s_%d() {\n", fragSynth, el.At.Line, synth, i)
		fmt.Fprintf(&b, "//line %s:%d:1\nvar p %sProps\n_ = p\n", fragSynth, el.At.Line, name)
		writeSignalDecls(&b, fragSynth, f, false)
		for _, param := range splitParams(fragmentParams(el)) {
			if ident := firstIdent(param); ident != "" && ident != "_" {
				fmt.Fprintf(&b, "//line %s:%d:1\nvar %s\n_ = %s\n", fragSynth, el.At.Line, param, ident)
			}
		}
		fw := &probeWriter{l: l, pkg: pkg, file: f, synth: fragSynth, pr: pr, b: &b}
		fw.fragmentBody(el)
		b.WriteString("}\n")
	}
	return pr, b.Bytes()
}

// writeSignalDecls declares one probe variable per signal so client
// expressions type-check (REQ-ACT-03). The main probe uses the initial
// value, which may read p; a fragment probe only needs the type.
func writeSignalDecls(b *bytes.Buffer, synth string, f *File, withDefault bool) {
	for i, s := range f.Signals {
		ds := fmt.Sprintf("%s-s%d", synth, i)
		if withDefault && s.HasDefault {
			fmt.Fprintf(b, "//line %s:%d:1\nvar _gxSig_%s %s = %s\n", ds, s.At.Line, s.Name, s.Type, s.Default)
			continue
		}
		fmt.Fprintf(b, "//line %s:%d:1\nvar _gxSig_%s %s\n", ds, s.At.Line, s.Name, s.Type)
	}
}

type probeWriter struct {
	l      *loader
	pkg    *Package
	file   *File
	synth  string
	pr     *probe
	b      *bytes.Buffer
	record bool
}

// fragmentBody probes the body of a fragment element. It does not record
// sites: the fragment scope probe only reports free variables (GX2008).
func (w *probeWriter) fragmentBody(el *Element) {
	for i := range el.Attrs {
		a := &el.Attrs[i]
		switch a.Kind {
		case AttrExpr:
			writeProbeExpr(w.b, a.Value, a.ValueAt, w.synth, w.pr, a, w.record)
		case AttrSpread:
			writeProbeSpread(w.b, a.Value, a.ValueAt, w.synth)
		}
	}
	w.nodes(el.Children)
}

func (w *probeWriter) nodes(ns []Node) {
	for _, n := range ns {
		switch t := n.(type) {
		case *Expr:
			writeProbeExpr(w.b, t.Data, t.DataAt, w.synth, w.pr, t, w.record)
		case *Element:
			for i := range t.Attrs {
				a := &t.Attrs[i]
				switch a.Kind {
				case AttrExpr:
					writeProbeExpr(w.b, a.Value, a.ValueAt, w.synth, w.pr, a, w.record)
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

func writeProbeExpr(b *bytes.Buffer, expr string, at Pos, synth string, pr *probe, site any, record bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.Contains(expr, "$") || at.Line <= 0 {
		return
	}
	if record {
		pr.sites = append(pr.sites, site)
		// The prefix "_=" is two columns wide, so the expression starts
		// at the column the directive names.
		col := at.Col - 2
		if col < 1 {
			col = 1
		}
		fmt.Fprintf(b, "//line %s:%d:%d\n_=%s\n", synth, at.Line, col, expr)
		return
	}
	const prefix = "var _ = "
	col := at.Col - len(prefix)
	if col < 1 {
		col = 1
	}
	fmt.Fprintf(b, "//line %s:%d:%d\n%s%s\n", synth, at.Line, col, prefix, expr)
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

// canonicalPath resolves symlinks in the enclosing directory so that overlay
// keys match the paths go/packages reports.
func canonicalPath(path string) string {
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return path
	}
	return filepath.Join(dir, filepath.Base(path))
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
