package compiler

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Island is one TypeScript island: a .ts file in a Go package that exports
// a default mount function (REQ-ISL-01). Its props are the Go struct
// <Name>Props of the same package.
type Island struct {
	Name string
	// File is the path of the .ts file.
	File string
	// Props holds the exported fields of the props struct, in source order.
	Props []Prop
	// HasProps is false when the package declares no <Name>Props struct.
	HasProps bool
	// file is the empty component file that stands for the island where
	// the compiler wants a .gx file: an island has no markup and no signals.
	file *File
}

// component returns the island as a component, so that a tag resolves to it
// like a tag of a .gx component.
func (i *Island) component() *Component {
	return &Component{Name: i.Name, File: i.file, Props: i.Props, Island: i}
}

// isIslandName reports whether a file name can be an island: <Name>.ts with
// an exported Go identifier as its name. A declaration file, a generated
// props file and a test file have a second dot and never match.
func isIslandName(name string) bool {
	base, ok := strings.CutSuffix(name, ".ts")
	return ok && isExportedIdent(base)
}

// The three forms of a default export: `export default ...`,
// `export { name as default }` and `export { default } from "..."`.
var (
	exportDefault     = regexp.MustCompile(`(^|[^\w$.])export\s+default($|[^\w$])`)
	exportAsDefault   = regexp.MustCompile(`(^|[^\w$.])export\s*\{[^}]*[^\w$]as\s+default\s*[,}]`)
	exportDefaultName = regexp.MustCompile(`(^|[^\w$.])export\s*\{\s*default\s*[,}]`)
)

// hasDefaultExport reports whether TypeScript source has a default export.
// It reads the source with comments and string contents removed, so the
// words in a comment or in a string do not count.
func hasDefaultExport(src []byte) bool {
	code := stripTSLiterals(src)
	return exportDefault.Match(code) || exportAsDefault.Match(code) || exportDefaultName.Match(code)
}

// stripTSLiterals returns src with every comment removed and the content of
// every string and template literal emptied. Code inside the ${...} of a
// template stays.
func stripTSLiterals(src []byte) []byte {
	out := make([]byte, 0, len(src))
	// templates holds, for each open ${...} of a template literal, the count
	// of braces open inside it. The template continues when the brace that
	// opened the expression closes.
	var templates []int
	i := 0
	template := func() {
		// i is after the opening backtick or after the } that ends an
		// expression.
		for i < len(src) {
			switch {
			case src[i] == '\\':
				i += 2
			case src[i] == '`':
				out = append(out, '`')
				i++
				return
			case src[i] == '$' && i+1 < len(src) && src[i+1] == '{':
				out = append(out, '$', '{')
				i += 2
				templates = append(templates, 0)
				return
			default:
				i++
			}
		}
	}
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(string(src[i+2:]), "*/")
			if end < 0 {
				i = len(src)
			} else {
				i += end + 4
			}
			out = append(out, ' ')
		case c == '"' || c == '\'':
			out = append(out, c)
			i++
			for i < len(src) && src[i] != c && src[i] != '\n' {
				if src[i] == '\\' {
					i++
				}
				i++
			}
			if i < len(src) && src[i] == c {
				out = append(out, c)
				i++
			}
		case c == '`':
			out = append(out, c)
			i++
			template()
		case c == '{' && len(templates) > 0:
			templates[len(templates)-1]++
			out = append(out, c)
			i++
		case c == '}' && len(templates) > 0:
			out = append(out, c)
			i++
			if templates[len(templates)-1] == 0 {
				templates = templates[:len(templates)-1]
				template()
			} else {
				templates[len(templates)-1]--
			}
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}

// loadIslands finds the islands of a package directory. entries is the
// directory listing.
func (l *loader) loadIslands(p *Package, entries []os.DirEntry) {
	var candidates []string
	hasGo := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch {
		case isIslandName(e.Name()):
			candidates = append(candidates, e.Name())
		case isPackageGoFile(e.Name()):
			hasGo = true
		}
	}
	if len(candidates) == 0 || (!hasGo && len(p.Files) == 0) {
		return
	}
	var structs map[string]*ast.StructType
	pkgName := ""
	for _, name := range candidates {
		path := filepath.Join(p.Dir, name)
		src, ok := l.read(path)
		if !ok || !hasDefaultExport(src) {
			continue
		}
		if structs == nil {
			structs, pkgName = l.packageStructs(p.Dir, entries)
			if pkgName == "" {
				for _, f := range p.Files {
					pkgName = f.Package
					break
				}
			}
		}
		isl := &Island{Name: strings.TrimSuffix(name, ".ts"), File: path}
		isl.file = &File{File: path, Package: pkgName}
		if st, ok := structs[isl.Name+"Props"]; ok {
			isl.HasProps = true
			isl.Props = islandProps(st)
		}
		if p.Islands == nil {
			p.Islands = map[string]*Island{}
		}
		p.Islands[isl.Name] = isl
	}
}

// read returns the content of an input file. An open buffer wins over the
// disk.
func (l *loader) read(path string) ([]byte, bool) {
	if data, ok := l.overlay[filepath.Clean(path)]; ok {
		return data, true
	}
	data, err := os.ReadFile(path)
	return data, err == nil
}

// isPackageGoFile reports whether name is a hand-written Go file of the
// package: not a test and not generated code.
func isPackageGoFile(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && !strings.HasSuffix(name, "_gx.go")
}

// packageStructs parses the Go files of a directory and returns the struct
// types it declares at package level, with the package name. The parse is
// syntax only: the island check runs before the type analysis, as the
// check of a .gx props block does.
func (l *loader) packageStructs(dir string, entries []os.DirEntry) (map[string]*ast.StructType, string) {
	out := map[string]*ast.StructType{}
	pkgName := ""
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !isPackageGoFile(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		src, ok := l.read(path)
		if !ok {
			continue
		}
		file, err := goparser.ParseFile(fset, path, src, goparser.SkipObjectResolution)
		if file == nil || (err != nil && file.Name == nil) {
			continue
		}
		if pkgName == "" {
			pkgName = file.Name.Name
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					out[ts.Name.Name] = st
				}
			}
		}
	}
	return out, pkgName
}

// islandProps returns the exported named fields of a props struct. Every
// prop is optional: a Go struct field has its zero value when the tag does
// not set it.
func islandProps(st *ast.StructType) []Prop {
	var out []Prop
	for _, field := range st.Fields.List {
		typ := types.ExprString(field.Type)
		doc := ""
		if field.Doc != nil {
			doc = strings.TrimSpace(field.Doc.Text())
		}
		for _, name := range field.Names {
			if !name.IsExported() {
				continue
			}
			out = append(out, Prop{Name: name.Name, Doc: doc, Type: typ, HasDefault: true})
		}
	}
	return out
}

// checkIslands returns GX6001 for each island of the package that has no
// props struct (REQ-ISL-01).
func checkIslands(p *Package) []Diagnostic {
	var out []Diagnostic
	for _, name := range islandNames(p) {
		isl := p.Islands[name]
		if isl.HasProps {
			continue
		}
		out = append(out, Diagnostic{
			Code: CodeIslandProps,
			File: isl.File,
			Line: 1,
			Col:  1,
			Msg:  "island " + Quoted(isl.Name) + " has no props struct " + Quoted(isl.Name+"Props") + " in its Go package",
			Fix:  "declare type " + isl.Name + "Props struct { ... } in a Go file of this directory",
		})
	}
	return out
}

// islandNames returns the island names of a package in order.
func islandNames(p *Package) []string {
	names := make([]string, 0, len(p.Islands))
	for name := range p.Islands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// dirHasIsland reports whether a directory holds a Go file and a .ts file
// that is an island. names is the directory listing.
func dirHasIsland(dir string, names []string) bool {
	hasGo := false
	for _, name := range names {
		if isPackageGoFile(name) || strings.HasSuffix(name, ".gx") {
			hasGo = true
			break
		}
	}
	if !hasGo {
		return false
	}
	for _, name := range names {
		if !isIslandName(name) {
			continue
		}
		if src, err := os.ReadFile(filepath.Join(dir, name)); err == nil && hasDefaultExport(src) {
			return true
		}
	}
	return false
}
