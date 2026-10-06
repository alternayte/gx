// Package goload loads the Go packages of an app for the compiler and the
// analyzers.
//
// go/packages can type-check a module, but with an overlay it loads every
// dependency from source: the standard library and each module the app
// needs. The compiler always has an overlay, so each analysis parsed and
// checked all of that again (NFR-05). This loader lists the packages with
// go/packages, type-checks the packages of the app from source and reads
// each dependency from the export data that the Go build cache holds.
package goload

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/tools/go/gcexportdata"
	"golang.org/x/tools/go/packages"
)

// Loader loads the packages below one directory. It keeps the export file
// of each dependency, so a second load of the same app lists them one time.
type Loader struct {
	// Dir is the directory of the app.
	Dir string

	exports map[string]string // package ID -> export file
}

// Load lists the packages that "./..." names below Dir and type-checks them
// from source. overlay holds file contents that replace or add files. Each
// returned package has Fset, Syntax, Types, TypesInfo and TypeErrors. A
// package that the returned ones import has Types and no Syntax.
func (l *Loader) Load(overlay map[string][]byte) ([]*packages.Package, error) {
	roots, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:     l.Dir,
		Overlay: overlay,
	}, "./...")
	if err != nil {
		return nil, err
	}
	isRoot := map[*packages.Package]bool{}
	for _, p := range roots {
		isRoot[p] = true
	}
	if err := l.listExports(roots, isRoot); err != nil {
		return nil, err
	}
	c := &checker{
		fset:    token.NewFileSet(),
		overlay: overlay,
		exports: l.exports,
		isRoot:  isRoot,
		view:    map[string]*types.Package{},
		state:   map[*packages.Package]int{},
	}
	for _, p := range roots {
		c.check(p)
	}
	return roots, nil
}

// listExports finds the export file of each dependency that a root imports
// and that the loader has not seen.
func (l *Loader) listExports(roots []*packages.Package, isRoot map[*packages.Package]bool) error {
	if l.exports == nil {
		l.exports = map[string]string{}
	}
	missing := map[string]bool{}
	for _, p := range roots {
		for _, dep := range p.Imports {
			if isRoot[dep] || dep.PkgPath == "unsafe" {
				continue
			}
			if _, ok := l.exports[dep.ID]; !ok {
				missing[dep.ID] = true
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	ids := make([]string, 0, len(missing))
	for id := range missing {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// The listing holds each package that the dependencies import too. A
	// dependency with no export data is checked from source, and its own
	// imports then come from export data, so one package has one identity.
	deps, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedExportFile | packages.NeedImports | packages.NeedDeps,
		Dir:  l.Dir,
	}, ids...)
	if err != nil {
		return err
	}
	packages.Visit(deps, nil, func(dep *packages.Package) {
		l.exports[dep.ID] = dep.ExportFile
	})
	return nil
}

// The states of a root package during the check.
const (
	checking = 1
	checked  = 2
)

type checker struct {
	fset    *token.FileSet
	overlay map[string][]byte
	exports map[string]string
	isRoot  map[*packages.Package]bool
	// view holds every package read from export data. The reader needs one
	// map for all reads, so that one type has one identity.
	view  map[string]*types.Package
	state map[*packages.Package]int
}

// check type-checks one root package after the roots it imports.
func (c *checker) check(p *packages.Package) {
	if c.state[p] != 0 {
		return
	}
	c.state[p] = checking
	for _, dep := range p.Imports {
		if c.isRoot[dep] {
			c.check(dep)
		}
	}
	p.Fset = c.fset
	p.Types = types.NewPackage(p.PkgPath, p.Name)
	p.TypesInfo = &types.Info{
		Types:        map[ast.Expr]types.TypeAndValue{},
		Defs:         map[*ast.Ident]types.Object{},
		Uses:         map[*ast.Ident]types.Object{},
		Implicits:    map[ast.Node]types.Object{},
		Instances:    map[*ast.Ident]types.Instance{},
		Scopes:       map[ast.Node]*types.Scope{},
		Selections:   map[*ast.SelectorExpr]*types.Selection{},
		FileVersions: map[*ast.File]string{},
	}
	files := p.CompiledGoFiles
	if len(files) == 0 {
		files = p.GoFiles
	}
	for _, name := range files {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		src, ok := c.overlay[name]
		if !ok {
			data, err := os.ReadFile(name)
			if err != nil {
				p.Errors = append(p.Errors, packages.Error{Pos: name, Msg: err.Error(), Kind: packages.ParseError})
				p.IllTyped = true
				continue
			}
			src = data
		}
		file, err := parser.ParseFile(c.fset, name, src, parser.AllErrors|parser.ParseComments)
		if err != nil {
			p.IllTyped = true
			var list scanner.ErrorList
			if errors.As(err, &list) {
				for _, e := range list {
					p.Errors = append(p.Errors, packages.Error{Pos: e.Pos.String(), Msg: e.Msg, Kind: packages.ParseError})
				}
			} else {
				p.Errors = append(p.Errors, packages.Error{Pos: name, Msg: err.Error(), Kind: packages.ParseError})
			}
		}
		if file != nil {
			p.Syntax = append(p.Syntax, file)
		}
	}
	conf := &types.Config{
		Importer: importerFunc(func(path string) (*types.Package, error) { return c.importFor(p, path) }),
		Sizes:    types.SizesFor("gc", runtime.GOARCH),
		Error: func(err error) {
			p.IllTyped = true
			var te types.Error
			if errors.As(err, &te) {
				p.TypeErrors = append(p.TypeErrors, te)
				p.Errors = append(p.Errors, packages.Error{Pos: te.Fset.Position(te.Pos).String(), Msg: te.Msg, Kind: packages.TypeError})
			}
		},
	}
	if p.Module != nil && p.Module.GoVersion != "" {
		conf.GoVersion = "go" + p.Module.GoVersion
	}
	// The Error function holds every error. The return value is the first.
	_ = types.NewChecker(conf, c.fset, p.Types, p.TypesInfo).Files(p.Syntax)
	c.state[p] = checked
}

// importFor resolves one import of a root package.
func (c *checker) importFor(p *packages.Package, path string) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	dep := p.Imports[path]
	if dep == nil {
		return nil, fmt.Errorf("no package for the import %q", path)
	}
	if c.isRoot[dep] {
		if c.state[dep] != checked {
			return nil, fmt.Errorf("import cycle through %q", path)
		}
		return dep.Types, nil
	}
	if dep.Types != nil {
		return dep.Types, nil
	}
	export := c.exports[dep.ID]
	if export == "" && len(dep.GoFiles)+len(dep.CompiledGoFiles) == 0 {
		return nil, fmt.Errorf("no package for the import %q", path)
	}
	if export == "" {
		// The build cache has no export data when the dependency does not
		// compile, for example when go.sum lacks a module that it needs.
		// Its declarations still give the types that the app uses.
		return c.fromSource(dep), nil
	}
	f, err := os.Open(export)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := gcexportdata.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("export data of %q: %w", path, err)
	}
	tpkg, err := gcexportdata.Read(r, c.fset, c.view, dep.PkgPath)
	if err != nil {
		return nil, fmt.Errorf("export data of %q: %w", path, err)
	}
	dep.Types = tpkg
	dep.Fset = c.fset
	return tpkg, nil
}

// fromSource type-checks the declarations of a dependency that has no
// export data. It skips function bodies and keeps no errors: the Go build
// reports them.
func (c *checker) fromSource(dep *packages.Package) *types.Package {
	if dep.Types != nil {
		return dep.Types
	}
	dep.Fset = c.fset
	dep.Types = types.NewPackage(dep.PkgPath, dep.Name)
	files := dep.CompiledGoFiles
	if len(files) == 0 {
		files = dep.GoFiles
	}
	var syntax []*ast.File
	for _, name := range files {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if file, _ := parser.ParseFile(c.fset, name, nil, parser.SkipObjectResolution); file != nil {
			syntax = append(syntax, file)
		}
	}
	conf := &types.Config{
		Importer:         importerFunc(func(path string) (*types.Package, error) { return c.importFor(dep, path) }),
		Sizes:            types.SizesFor("gc", runtime.GOARCH),
		IgnoreFuncBodies: true,
		Error:            func(error) {},
	}
	if dep.Module != nil && dep.Module.GoVersion != "" {
		conf.GoVersion = "go" + dep.Module.GoVersion
	}
	_ = types.NewChecker(conf, c.fset, dep.Types, nil).Files(syntax)
	return dep.Types
}

type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }
