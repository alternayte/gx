package gx_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pluginRegistryFindings lists what makes a package a runtime plugin
// registry: an init function, a declaration that names a plugin, or a
// Register function. Test files are skipped; every build tag is read.
func pluginRegistryFindings(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		report := func(ident *ast.Ident, kind string) {
			lower := strings.ToLower(ident.Name)
			switch {
			case kind == "func" && ident.Name == "init":
				out = append(out, name+": func init")
			case strings.Contains(lower, "plugin"):
				out = append(out, name+": "+kind+" "+ident.Name)
			case kind == "func" && strings.HasPrefix(ident.Name, "Register"):
				out = append(out, name+": func "+ident.Name)
			}
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					report(d.Name, "func")
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						report(s.Name, "type")
					case *ast.ValueSpec:
						for _, n := range s.Names {
							report(n, "var")
						}
					}
				}
			}
		}
	}
	return out
}

// TestREQ_PLG_06_NoRuntimePluginRegistry covers the architecture rule:
// runtime extension is plain Go, and package gx holds no plugin registry
// and no init registration (REQ-PLG-06).
func TestREQ_PLG_06_NoRuntimePluginRegistry(t *testing.T) {
	if found := pluginRegistryFindings(t, "."); len(found) > 0 {
		t.Fatalf("package gx has a plugin registry:\n%s", strings.Join(found, "\n"))
	}

	// The scan must see a registry when one exists.
	seeded := t.TempDir()
	src := "package gx\n\ntype Plugin interface{ Name() string }\n\nvar plugins []Plugin\n\nfunc RegisterPlugin(p Plugin) { plugins = append(plugins, p) }\n\nfunc init() {}\n"
	if err := os.WriteFile(filepath.Join(seeded, "plugins.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if found := pluginRegistryFindings(t, seeded); len(found) != 4 {
		t.Fatalf("the scan found %d of 4 seeded registry parts: %v", len(found), found)
	}
}
