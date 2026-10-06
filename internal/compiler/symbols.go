package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/constant"
	"go/format"
	"go/types"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// The dev symbol table of an app (REQ-DEV-04): one generated file in each
// package that holds .gx files, with every package-level name of that
// package, and one generated package that lists those packages and the
// exported names of their direct imports. The files have the gxdev build
// tag, so a production binary holds none of them (SI-08).
const (
	symbolsFile    = "gxdev_symbols_gx.go"
	symbolsPackage = "gxdev_symbols"
)

// symbolsFilePath is the output path of the generated package that lists
// the symbol tables.
func symbolsFilePath(root string) string {
	return filepath.Join(root, symbolsPackage, "symbols_gx.go")
}

// symbolSet is the names of one package for its table.
type symbolSet struct {
	values  []string // name: Go expression of a reflect.Value
	consts  []string
	types   []string
	methods []string
}

func (s *symbolSet) write(b *bytes.Buffer, pkgPath, pkgName, gx string) {
	fmt.Fprintf(b, "%sDevPackage{\n\t\tPath: %s,\n\t\tName: %s,\n", gx, strconv.Quote(pkgPath), strconv.Quote(pkgName))
	section := func(field, typ string, entries []string) {
		if len(entries) == 0 {
			return
		}
		sort.Strings(entries)
		fmt.Fprintf(b, "\t\t%s: map[string]%s{\n", field, typ)
		for _, e := range entries {
			fmt.Fprintf(b, "\t\t\t%s,\n", e)
		}
		b.WriteString("\t\t},\n")
	}
	section("Values", "reflect.Value", s.values)
	section("Consts", "constant.Value", s.consts)
	section("Types", "reflect.Type", s.types)
	section("Methods", "reflect.Value", s.methods)
	b.WriteString("\t}")
}

// constantExpr writes an untyped constant as a call of a gx helper.
func constantExpr(gx string, v constant.Value) (string, bool) {
	switch v.Kind() {
	case constant.Bool:
		return gx + "DevBool(" + strconv.FormatBool(constant.BoolVal(v)) + ")", true
	case constant.String:
		return gx + "DevString(" + strconv.Quote(constant.StringVal(v)) + ")", true
	case constant.Int:
		return gx + "DevInt(" + strconv.Quote(v.ExactString()) + ")", true
	case constant.Float:
		f, _ := constant.Float64Val(v)
		return gx + "DevFloat(" + strconv.Quote(strconv.FormatFloat(f, 'g', -1, 64)) + ")", true
	}
	return "", false
}

// addObject adds one package-level object. qual is the qualifier of the
// package in the generated file, with its dot, or "" inside the package.
// unexported says whether the file can name unexported objects.
func (s *symbolSet) addObject(obj types.Object, qual, gx string, unexported bool) {
	name := obj.Name()
	if name == "_" || name == "init" || name == "main" || strings.HasPrefix(name, "_gx") || name == "GxDevSymbols" {
		return
	}
	if !unexported && !obj.Exported() {
		return
	}
	key := strconv.Quote(name) + ": "
	switch o := obj.(type) {
	case *types.Func:
		sig, ok := o.Type().(*types.Signature)
		if !ok || sig.TypeParams() != nil {
			return // a generic function has no value
		}
		s.values = append(s.values, key+"reflect.ValueOf("+qual+name+")")
	case *types.Var:
		s.values = append(s.values, key+"reflect.ValueOf(&"+qual+name+").Elem()")
	case *types.Const:
		if basic, ok := o.Type().(*types.Basic); ok && basic.Info()&types.IsUntyped != 0 {
			if expr, ok := constantExpr(gx, o.Val()); ok {
				s.consts = append(s.consts, key+expr)
			}
			return
		}
		s.values = append(s.values, key+"reflect.ValueOf("+qual+name+")")
	case *types.TypeName:
		named, isNamed := types.Unalias(o.Type()).(*types.Named)
		if isNamed && named.TypeParams() != nil {
			return // a generic type has no single type
		}
		if alias, ok := o.Type().(*types.Alias); ok && alias.TypeParams() != nil {
			return
		}
		s.types = append(s.types, key+"reflect.TypeFor["+qual+name+"]()")
		if !isNamed || !unexported || named.Obj() != o {
			return
		}
		// Reflection cannot call an unexported method, so the table holds
		// a function for each one.
		if _, isInterface := named.Underlying().(*types.Interface); isInterface {
			return
		}
		for i := 0; i < named.NumMethods(); i++ {
			m := named.Method(i)
			if m.Exported() {
				continue
			}
			sig := m.Type().(*types.Signature)
			if _, pointer := sig.Recv().Type().(*types.Pointer); pointer {
				s.methods = append(s.methods, strconv.Quote("*"+name+"."+m.Name())+": reflect.ValueOf((*"+name+")."+m.Name()+")")
			} else {
				s.methods = append(s.methods, strconv.Quote(name+"."+m.Name())+": reflect.ValueOf("+name+"."+m.Name()+")")
			}
		}
	}
}

// inDevBuild reports whether a source file is part of a build with the
// gxdev tag on this platform.
func inDevBuild(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, c := range group.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				continue
			}
			return expr.Eval(func(tag string) bool {
				return tag == "gxdev" || tag == runtime.GOOS || tag == runtime.GOARCH || tag == "cgo" ||
					(tag == "unix" && runtime.GOOS != "windows") || strings.HasPrefix(tag, "go1.")
			})
		}
	}
	return true
}

// platformPackages have different exported names on different systems. A
// generated table is committed and built on each system, so it leaves them
// out; a reference to one of them takes the rebuild path.
var platformPackages = map[string]bool{"syscall": true, "os/signal": true, "unsafe": true, "C": true, "runtime/cgo": true}

// importable reports whether the generated package of the app at appPath
// can import path.
func importable(appPath, path string) bool {
	if platformPackages[path] || strings.HasPrefix(path, "golang.org/x/sys/") {
		return false
	}
	i := strings.LastIndex(path, "/internal/")
	if i < 0 && !strings.HasSuffix(path, "/internal") && !strings.HasPrefix(path, "internal/") {
		return true
	}
	if strings.HasPrefix(path, "internal/") {
		return false // the standard library
	}
	parent := path
	if i >= 0 {
		parent = path[:i]
	} else {
		parent = strings.TrimSuffix(path, "/internal")
	}
	return appPath == parent || strings.HasPrefix(appPath, parent+"/")
}

// renderSymbols renders the dev symbol table: the file of each package with
// .gx files, by output path, and the package that lists them.
func renderSymbols(root string, dirs []string, l *loader, res *typesResult) map[string][]byte {
	out := map[string][]byte{}
	byPath := map[string]*packages.Package{}
	for _, pkg := range res.pkgs {
		if pkg.Types != nil {
			byPath[pkg.PkgPath] = pkg
		}
	}
	mod := findModule(root)
	if mod == nil {
		return out
	}
	appPath := modulePathOf(mod, root)

	type local struct {
		path, name string
	}
	var locals []local
	imports := map[string]*packages.Package{}
	localPaths := map[string]bool{}
	for _, dir := range dirs {
		p := l.load(dir)
		if p.Module == nil || len(p.Files) == 0 {
			continue
		}
		pkgPath := modulePathOf(p.Module, dir)
		pkg := byPath[pkgPath]
		if pkg == nil || pkg.Types == nil {
			continue
		}
		// The names of the package as the type check saw them. An object
		// of a file that is not part of a dev build is left out.
		excluded := map[string]bool{}
		for _, file := range pkg.Syntax {
			if inDevBuild(file) {
				continue
			}
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						excluded[d.Name.Name] = true
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch sp := spec.(type) {
						case *ast.TypeSpec:
							excluded[sp.Name.Name] = true
						case *ast.ValueSpec:
							for _, n := range sp.Names {
								excluded[n.Name] = true
							}
						}
					}
				}
			}
		}
		set := &symbolSet{}
		scope := pkg.Types.Scope()
		seen := map[string]bool{}
		for _, name := range scope.Names() {
			if excluded[name] {
				continue
			}
			seen[name] = true
			set.addObject(scope.Lookup(name), "", "gx.", true)
		}
		// The type check reads a short form of each .gx file. The real
		// generated code also has the fragment functions, the signals
		// types and the functions of the islands.
		for _, base := range sortedFileNames(p.Files) {
			f := p.Files[base]
			if len(f.Signals) > 0 && !seen[base+"Signals"] {
				set.types = append(set.types, strconv.Quote(base+"Signals")+": reflect.TypeFor["+base+"Signals]()")
			}
			for _, el := range fragmentElements(f.Body) {
				for i := range el.Attrs {
					if el.Attrs[i].Kind == AttrFragment {
						name := base + upperFirst(el.Attrs[i].Name)
						if !seen[name] {
							seen[name] = true
							set.values = append(set.values, strconv.Quote(name)+": reflect.ValueOf("+name+")")
						}
					}
				}
			}
		}
		for _, name := range islandNames(p) {
			if _, ok := res.islands[p.Islands[name]]; ok && !seen[name] {
				set.values = append(set.values, strconv.Quote(name)+": reflect.ValueOf("+name+")")
			}
		}

		var b bytes.Buffer
		b.WriteString("// Code generated by gx. DO NOT EDIT.\n\n//go:build gxdev\n\n")
		fmt.Fprintf(&b, "package %s\n\n", pkg.Types.Name())
		b.WriteString("import (\n\t\"go/constant\"\n\t\"reflect\"\n\n\tgx \"github.com/alternayte/gx\"\n)\n\n")
		b.WriteString("// GxDevSymbols lists the package-level names of this package for the dev\n// interpreter (REQ-DEV-04).\n")
		b.WriteString("func GxDevSymbols() gx.DevPackage {\n\treturn ")
		set.write(&b, pkgPath, pkg.Types.Name(), "gx.")
		b.WriteString("\n}\n\nvar _ constant.Value\n")
		out[filepath.Join(dir, symbolsFile)] = formatSymbols(b.Bytes())

		locals = append(locals, local{path: pkgPath, name: pkg.Types.Name()})
		localPaths[pkgPath] = true
		for path, dep := range pkg.Imports {
			imports[path] = dep
		}
	}
	if len(locals) == 0 {
		return out
	}

	// The package that lists the tables: the packages with .gx files, and
	// the exported names of what they import.
	central := appPath + "/" + symbolsPackage
	var paths []string
	for path, dep := range imports {
		if localPaths[path] || path == central || dep.Types == nil || !importable(central, path) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	// A package with no exported value, constant or type has no table and
	// no import.
	sets := map[string]*symbolSet{}
	kept := paths[:0]
	n := len(locals)
	importAlias := map[string]string{}
	for _, path := range paths {
		qual := "gx"
		if path != gxPkgPath {
			n++
			qual = "p" + strconv.Itoa(n)
		}
		set := &symbolSet{}
		scope := imports[path].Types.Scope()
		for _, name := range scope.Names() {
			set.addObject(scope.Lookup(name), qual+".", "gx.", false)
		}
		if len(set.values)+len(set.consts)+len(set.types) == 0 {
			continue
		}
		sets[path] = set
		importAlias[path] = qual
		kept = append(kept, path)
	}
	paths = kept
	var b bytes.Buffer
	b.WriteString("// Code generated by gx. DO NOT EDIT.\n\n//go:build gxdev\n\n")
	b.WriteString("// Package " + symbolsPackage + " is the dev symbol table of the app (REQ-DEV-04):\n")
	b.WriteString("// the names that interpreted template code can use with no rebuild.\n")
	fmt.Fprintf(&b, "package %s\n\nimport (\n\t\"go/constant\"\n\t\"reflect\"\n\n\tgx \"github.com/alternayte/gx\"\n", symbolsPackage)
	alias := map[string]string{}
	for i, l := range locals {
		alias[l.path] = "p" + strconv.Itoa(i+1)
		fmt.Fprintf(&b, "\t%s %s\n", alias[l.path], strconv.Quote(l.path))
	}
	for _, path := range paths {
		if path != gxPkgPath {
			fmt.Fprintf(&b, "\t%s %s\n", importAlias[path], strconv.Quote(path))
		}
	}
	b.WriteString(")\n\n// Packages returns the symbol table. The dev main of the app gives it to\n// gx.SetDevSymbols.\nfunc Packages() []gx.DevPackage {\n\treturn []gx.DevPackage{\n")
	for _, l := range locals {
		fmt.Fprintf(&b, "\t\t%s.GxDevSymbols(),\n", alias[l.path])
	}
	for _, path := range paths {
		b.WriteString("\t\t")
		sets[path].write(&b, path, imports[path].Types.Name(), "gx.")
		b.WriteString(",\n")
	}
	b.WriteString("\t}\n}\n\nvar _ constant.Value\nvar _ reflect.Value\n")
	out[symbolsFilePath(root)] = formatSymbols(b.Bytes())
	return out
}

func formatSymbols(src []byte) []byte {
	if formatted, err := format.Source(src); err == nil {
		return formatted
	}
	return src
}
