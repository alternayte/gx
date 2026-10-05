package compiler

import (
	"bytes"
	"go/ast"
	goparser "go/parser"
	goprinter "go/printer"
	"go/token"
	"path"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

// formatOnly formats a file and sorts its imports. It does not look for
// packages, so it does not read the module cache.
var formatOnly = &imports.Options{Comments: true, TabIndent: true, TabWidth: 8, FormatOnly: true}

// packageSearches counts the files that needed the full package search.
var packageSearches atomic.Int64

// fixImports formats generated source and removes each import the code
// does not use. The generator writes every import the code needs, so the
// usual result needs no package search. A full search reads the module
// cache index for each file and breaks the cold build budget (NFR-05).
// The search runs only when a qualifier has no import.
func fixImports(filename string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, filename, src, goparser.ParseComments)
	if err != nil {
		return imports.Process(filename, src, nil)
	}
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil {
			used[id.Name] = true
		}
		return true
	})
	type unusedImport struct{ name, path string }
	var unused []unusedImport
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return imports.Process(filename, src, nil)
		}
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		local := name
		if local == "" {
			local = importBaseName(p)
		}
		if used[local] {
			delete(used, local)
			continue
		}
		unused = append(unused, unusedImport{name, p})
	}
	if len(used) > 0 {
		// A name with no import is a package-level name of another file, a
		// package whose name differs from its path, or a missing import.
		// Only the full search can tell which.
		packageSearches.Add(1)
		return imports.Process(filename, src, nil)
	}
	if len(unused) == 0 {
		return imports.Process(filename, src, formatOnly)
	}
	for _, u := range unused {
		astutil.DeleteNamedImport(fset, file, u.name, u.path)
	}
	var b bytes.Buffer
	if err := (&goprinter.Config{Mode: goprinter.UseSpaces | goprinter.TabIndent, Tabwidth: 8}).Fprint(&b, fset, file); err != nil {
		return nil, err
	}
	return imports.Process(filename, b.Bytes(), formatOnly)
}

// importBaseName returns the package name that an import path implies:
// the last element, or the element before a major version suffix.
func importBaseName(p string) string {
	base := path.Base(p)
	if len(base) > 1 && base[0] == 'v' && strings.Trim(base[1:], "0123456789") == "" {
		if dir := path.Dir(p); dir != "." {
			base = path.Base(dir)
		}
	}
	return base
}
