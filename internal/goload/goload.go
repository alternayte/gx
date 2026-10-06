// Package goload holds the parse step that the compiler and the analyzers
// give to go/packages.
package goload

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ParseFile returns a parse function for packages.Config. A file under root
// parses in full. A file of a dependency, such as the standard library or
// the gx module, parses without function bodies: the type checker needs the
// declarations of a dependency only, and the bodies are most of its work
// (NFR-05).
func ParseFile(root string) func(*token.FileSet, string, []byte) (*ast.File, error) {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	// A path can reach the root through a symbolic link, such as /var on
	// macOS. Both spellings of the root count.
	prefixes := []string{root + string(filepath.Separator)}
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		prefixes = append(prefixes, real+string(filepath.Separator))
	}
	under := func(path string) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return false
	}
	inRoot := func(filename string) bool {
		if under(filename) {
			return true
		}
		// An overlay file is not on the disk, so its directory resolves.
		dir, err := filepath.EvalSymlinks(filepath.Dir(filename))
		return err == nil && under(dir+string(filepath.Separator))
	}
	return func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		const mode = parser.AllErrors | parser.ParseComments
		if inRoot(filename) {
			return parser.ParseFile(fset, filename, src, mode)
		}
		// A dependency needs no comments and no identifier resolution.
		file, err := parser.ParseFile(fset, filename, src, parser.AllErrors|parser.SkipObjectResolution)
		if file == nil {
			return file, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			// The type checker reports an init function and a generic
			// function that have no body.
			if fn.Recv == nil && fn.Name.Name == "init" {
				continue
			}
			if fn.Type.TypeParams != nil && len(fn.Type.TypeParams.List) > 0 {
				continue
			}
			fn.Body = nil
		}
		return file, err
	}
}
