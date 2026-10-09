package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// The sidebar groups of the component pages, in sidebar order.
const (
	groupComponents = "Components"
	groupBlocks     = "Blocks"
	groupDocsKit    = "Docs kit"
	groupIcons      = "Icons"
)

// groups lists the sidebar groups in sidebar order.
var groups = []string{groupComponents, groupBlocks, groupDocsKit, groupIcons}

// manifestFile is one file of a registry item manifest.
type manifestFile struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// manifest is the part of gx-item.json the docs pages show.
type manifest struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Description  string         `json:"description"`
	Kind         string         `json:"kind"`
	Files        []manifestFile `json:"files"`
	Dependencies []string       `json:"registryDependencies"`
	Tokens       []string       `json:"requiredTokens"`
}

// section is one H2 section of USAGE.md.
type section struct {
	Title string
	Body  string
}

// usage is the parsed USAGE.md of one item.
type usage struct {
	Title    string
	Intro    string
	Usage    string
	Do       string
	Dont     string
	Keyboard string
	// Other holds the sections with any other heading, in file order.
	Other []section
}

// fixture is one named prop set of a component.
type fixture struct {
	Name string
	// Value is the prop literal; Source is its text in the fixtures file.
	Value  ast.Expr
	Source string
}

// component is one component of an item that has a fixtures file.
type component struct {
	Name string
	// Gx is the parsed .gx file. It is nil for a component written in Go.
	Gx *compiler.File
	// FixturesVar is the declared gx.Fixtures variable; PropsType is its
	// type argument.
	FixturesVar string
	PropsType   ast.Expr
	// Wrap is the optional <Name>Wrap function of the fixtures file.
	Wrap     string
	Fixtures []fixture
	// file is the parsed fixtures file; in is its text.
	file *ast.File
	in   *source
}

// item is one registry item.
type item struct {
	manifest
	Dir     string
	PkgName string
	PkgPath string
	Title   string
	Group   string
	Usage   usage
	// Components holds the main component first, then the rest by name.
	Components []*component
	// IconSet is true when every component is one gx.Icon call.
	IconSet bool
	// Server is true when the item text names behaviour a server drives.
	Server bool
	// names holds the package-level identifiers of the item package.
	names map[string]bool
	// structs holds the fields of the props structs that the Go files of
	// the item declare, by type name.
	structs map[string][]prop
	// helpers holds the unexported package-level values and one-line
	// functions of the item package. The converter writes their value in
	// place of their name.
	helpers map[string]*helper
}

// prop is one row of an API reference table.
type prop struct {
	Name, Type, Default, Doc string
	// Required is true for a prop with no default.
	Required bool
}

// source is one parsed Go file of an item.
type source struct {
	src  []byte
	fset *token.FileSet
	// imports maps each package qualifier of the file to its import path.
	imports map[string]string
}

// helper is an unexported constant, variable or function of an item
// package that a fixture names. A function helper is one return statement.
type helper struct {
	in     *source
	value  ast.Expr
	params []string
	isFunc bool
}

// registry is the loaded registry directory.
type registry struct {
	// Root is the repository root; Module is its module path.
	Root   string
	Module string
	Items  []*item
	byName map[string]*item
	// props caches the parsed .gx file of a component by directory and
	// name.
	props map[string]*compiler.File
}

// loadRegistry reads every item directory under root/registry: a directory
// with a gx-item.json (REQ-DOC-02).
func loadRegistry(root string) (*registry, error) {
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	reg := &registry{Root: root, Module: modulePath(mod), byName: map[string]*item{}, props: map[string]*compiler.File{}}
	if reg.Module == "" {
		return nil, fmt.Errorf("docsgen: %s has no module line", filepath.Join(root, "go.mod"))
	}
	entries, err := os.ReadDir(filepath.Join(root, "registry"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, "registry", e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "gx-item.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		it := &item{Dir: dir, PkgPath: reg.Module + "/registry/" + e.Name()}
		if err := json.Unmarshal(data, &it.manifest); err != nil {
			return nil, fmt.Errorf("docsgen: %s: %w", filepath.Join(dir, "gx-item.json"), err)
		}
		if it.Name == "" {
			return nil, fmt.Errorf("docsgen: %s has no name", filepath.Join(dir, "gx-item.json"))
		}
		if err := reg.loadItem(it); err != nil {
			return nil, err
		}
		reg.Items = append(reg.Items, it)
		reg.byName[it.Name] = it
	}
	sort.Slice(reg.Items, func(i, j int) bool { return reg.Items[i].Name < reg.Items[j].Name })
	return reg, nil
}

// modulePath returns the module path of a go.mod file.
func modulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// loadItem reads the usage text, the package names and the components of
// one item.
func (reg *registry) loadItem(it *item) error {
	text, err := os.ReadFile(filepath.Join(it.Dir, "USAGE.md"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	it.Usage = parseUsage(string(text))
	it.Title = it.Usage.Title
	if it.Title == "" || it.Title == it.Name {
		it.Title = titleOf(it.Name)
	}
	it.Server = serverWord.MatchString(it.Description) || serverWord.MatchString(it.Usage.Intro)

	entries, err := os.ReadDir(it.Dir)
	if err != nil {
		return err
	}
	it.names = map[string]bool{}
	it.helpers = map[string]*helper{}
	it.structs = map[string][]prop{}
	fset := token.NewFileSet()
	var fixtureFiles []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(it.Dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return fmt.Errorf("docsgen: %w", err)
		}
		it.PkgName = file.Name.Name
		declaredNames(file, it.names)
		declaredHelpers(file, &source{src: src, fset: fset, imports: reg.fileImports(file)}, it.helpers)
		declaredProps(file, src, fset, it.structs)
		if strings.HasSuffix(name, ".fixtures.go") {
			fixtureFiles = append(fixtureFiles, name)
		}
	}
	for _, name := range fixtureFiles {
		comp, err := reg.loadComponent(it, name)
		if err != nil {
			return err
		}
		if comp != nil {
			it.Components = append(it.Components, comp)
		}
	}
	main := strings.ToLower(strings.ReplaceAll(it.Name, "-", ""))
	sort.SliceStable(it.Components, func(i, j int) bool {
		a, b := it.Components[i], it.Components[j]
		am, bm := strings.ToLower(a.Name) == main, strings.ToLower(b.Name) == main
		if am != bm {
			return am
		}
		return a.Name < b.Name
	})
	it.IconSet = len(it.Components) > 0
	for _, comp := range it.Components {
		if !isIconComponent(comp.Gx) {
			it.IconSet = false
		}
	}
	switch {
	case it.IconSet:
		it.Group = groupIcons
	case it.Name == "docs" || strings.HasPrefix(it.Name, "docs-") || strings.HasPrefix(it.Name, "starlight"):
		it.Group = groupDocsKit
	case it.Kind == "block":
		it.Group = groupBlocks
	default:
		it.Group = groupComponents
	}
	return nil
}

// serverWord finds the word "server" in an item description.
var serverWord = regexp.MustCompile(`(?i)\bserver\b`)

// declaredNames adds the package-level identifiers of file to names.
func declaredNames(file *ast.File, names map[string]bool) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						names[n.Name] = true
					}
				}
			}
		}
	}
}

// fileImports maps the package qualifiers of a Go file to import paths. An
// import of a registry item has the package name of the item.
func (reg *registry) fileImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		} else if dir, ok := reg.packageDir(p); ok {
			if pkg := packageName(dir); pkg != "" {
				name = pkg
			}
		}
		out[name] = p
	}
	return out
}

// packageName returns the package name of the Go files in dir, or "".
func packageName(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.PackageClauseOnly)
		if err == nil {
			return file.Name.Name
		}
	}
	return ""
}

// declaredHelpers adds the unexported helpers of file to helpers: a
// constant or variable with one value, and a function whose body is one
// return of one value.
func declaredHelpers(file *ast.File, in *source, helpers map[string]*helper) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || ast.IsExported(d.Name.Name) || d.Body == nil || len(d.Body.List) != 1 || d.Type.TypeParams != nil {
				continue
			}
			ret, ok := d.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}
			h := &helper{in: in, value: ret.Results[0], isFunc: true}
			variadic := false
			for _, field := range d.Type.Params.List {
				if _, ok := field.Type.(*ast.Ellipsis); ok {
					variadic = true
				}
				for _, n := range field.Names {
					h.params = append(h.params, n.Name)
				}
			}
			if !variadic {
				helpers[d.Name.Name] = h
			}
		case *ast.GenDecl:
			if d.Tok != token.CONST && d.Tok != token.VAR {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || ast.IsExported(vs.Names[0].Name) {
					continue
				}
				helpers[vs.Names[0].Name] = &helper{in: in, value: vs.Values[0]}
			}
		}
	}
}

// declaredProps adds the fields of every <Name>Props struct of file to
// structs. A component that is a Go function has no props block; its props
// are this struct.
func declaredProps(file *ast.File, src []byte, fset *token.FileSet, structs map[string][]prop) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !strings.HasSuffix(ts.Name.Name, "Props") {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				typ := string(src[fset.Position(field.Type.Pos()).Offset:fset.Position(field.Type.End()).Offset])
				for _, name := range field.Names {
					if ast.IsExported(name.Name) {
						structs[ts.Name.Name] = append(structs[ts.Name.Name], prop{
							Name: name.Name, Type: typ, Doc: strings.TrimSpace(field.Doc.Text()),
						})
					}
				}
			}
		}
	}
}

// props returns the API reference rows of a component: the fields of its
// props block, or of its props struct when the component is a Go function.
func (it *item) props(comp *component) []prop {
	if comp.Gx == nil {
		return it.structs[comp.Name+"Props"]
	}
	var out []prop
	for _, f := range comp.Gx.Props {
		out = append(out, prop{Name: f.Name, Type: f.Type, Default: f.Default, Required: !f.HasDefault, Doc: f.Doc})
	}
	return out
}

// loadComponent reads one <Name>.fixtures.go file: the gx.Fixtures literal
// in source order, and the optional wrap function. It returns nil when the
// file declares no fixtures.
func (reg *registry) loadComponent(it *item, fileName string) (*component, error) {
	path := filepath.Join(it.Dir, fileName)
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("docsgen: %w", err)
	}
	comp := &component{Name: strings.TrimSuffix(fileName, ".fixtures.go"), file: file}
	comp.in = &source{src: src, fset: fset, imports: reg.fileImports(file)}
	comp.Gx = reg.componentFile(it.Dir, comp.Name)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == comp.Name+"Wrap" {
			comp.Wrap = fn.Name.Name
			continue
		}
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok || !(isFixturesType(lit.Type) || (lit.Type == nil && isFixturesType(vs.Type))) {
				continue
			}
			comp.FixturesVar = vs.Names[0].Name
			if lit.Type != nil {
				comp.PropsType = lit.Type.(*ast.IndexExpr).Index
			} else {
				comp.PropsType = vs.Type.(*ast.IndexExpr).Index
			}
			if !ast.IsExported(comp.FixturesVar) {
				return nil, fmt.Errorf("docsgen: %s: the fixtures variable %s is not exported", path, comp.FixturesVar)
			}
			seen := map[string]bool{}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					return nil, fmt.Errorf("docsgen: %s: a fixture has no name", fset.Position(elt.Pos()))
				}
				key, ok := kv.Key.(*ast.BasicLit)
				if !ok || key.Kind != token.STRING {
					return nil, fmt.Errorf("docsgen: %s: a fixture name is not a string literal", fset.Position(kv.Key.Pos()))
				}
				name, err := strconv.Unquote(key.Value)
				if err != nil || name == "" || seen[name] {
					return nil, fmt.Errorf("docsgen: %s: the fixture name %s is empty or repeated", fset.Position(kv.Key.Pos()), key.Value)
				}
				seen[name] = true
				comp.Fixtures = append(comp.Fixtures, fixture{
					Name:   name,
					Value:  kv.Value,
					Source: string(src[fset.Position(kv.Pos()).Offset:fset.Position(kv.End()).Offset]),
				})
			}
		}
	}
	if comp.FixturesVar == "" || len(comp.Fixtures) == 0 {
		return nil, nil
	}
	return comp, nil
}

// isFixturesType reports whether a type expression is gx.Fixtures[...].
func isFixturesType(expr ast.Expr) bool {
	index, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	sel, ok := index.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Fixtures" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "gx"
}

// componentFile returns the parsed .gx file of a component, or nil when
// the component has none or it does not parse.
func (reg *registry) componentFile(dir, name string) *compiler.File {
	path := filepath.Join(dir, name+".gx")
	if f, ok := reg.props[path]; ok {
		return f
	}
	var file *compiler.File
	if src, err := os.ReadFile(path); err == nil {
		if f, diags := compiler.ParseFile(path, src); len(diags) == 0 {
			file = f
		}
	}
	reg.props[path] = file
	return file
}

// packageDir maps an import path inside the repository module to its
// directory.
func (reg *registry) packageDir(importPath string) (string, bool) {
	rel, ok := strings.CutPrefix(importPath, reg.Module+"/")
	if !ok {
		return "", false
	}
	return filepath.Join(reg.Root, filepath.FromSlash(rel)), true
}

// isIconComponent reports whether a component body is one gx.Icon call, the
// shape `gx icons pin` writes.
func isIconComponent(f *compiler.File) bool {
	if f == nil {
		return false
	}
	nodes := compiler.VisibleNodes(f.Body)
	var exprs []*compiler.Expr
	for _, n := range nodes {
		switch t := n.(type) {
		case *compiler.Text:
			if strings.TrimSpace(t.Data) != "" {
				return false
			}
		case *compiler.Expr:
			exprs = append(exprs, t)
		default:
			return false
		}
	}
	return len(exprs) == 1 && strings.HasPrefix(strings.TrimSpace(exprs[0].Data), "gx.Icon(")
}

// titleOf turns an item name into a page title: "docs-shell" gives
// "Docs shell".
func titleOf(name string) string {
	words := strings.ReplaceAll(name, "-", " ")
	if words == "" {
		return ""
	}
	return strings.ToUpper(words[:1]) + words[1:]
}

// parseUsage splits USAGE.md into its title, its intro and its H2
// sections. A heading inside a code fence stays text.
func parseUsage(text string) usage {
	var u usage
	var intro strings.Builder
	var cur *section
	var sections []*section
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
		} else if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
		} else if rest, ok := strings.CutPrefix(line, "# "); ok && u.Title == "" && cur == nil {
			u.Title = strings.TrimSpace(rest)
			continue
		} else if rest, ok := strings.CutPrefix(line, "## "); ok {
			cur = &section{Title: strings.TrimSpace(rest)}
			sections = append(sections, cur)
			continue
		}
		if cur == nil {
			intro.WriteString(line + "\n")
		} else {
			cur.Body += line + "\n"
		}
	}
	u.Intro = strings.TrimSpace(intro.String())
	for _, s := range sections {
		body := strings.TrimSpace(s.Body)
		switch strings.ToLower(s.Title) {
		case "usage", "use":
			u.Usage = body
		case "do":
			u.Do = body
		case "don't", "dont", "don’t":
			u.Dont = body
		case "keyboard":
			u.Keyboard = body
		default:
			u.Other = append(u.Other, section{Title: s.Title, Body: body})
		}
	}
	return u
}
