package goload

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNFR_05_DependenciesComeFromExportData pins the load budget: a package
// of the app is checked from source with its overlay, and a dependency has
// types and no syntax, so the loader did not parse it.
func TestNFR_05_DependenciesComeFromExportData(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write("go.mod", "module app\n\ngo 1.25.0\n")
	write("lib/lib.go", "package lib\n\nfunc Name() string { return \"lib\" }\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	// The overlay adds a file that is not on the disk. It imports a package
	// of the app and a package of the standard library.
	extra := filepath.Join(dir, "extra.go")
	overlay := map[string][]byte{extra: []byte("package main\n\nimport (\n\t\"net/http\"\n\n\t\"app/lib\"\n)\n\nvar handler http.Handler\n\nvar name = lib.Name()\n")}

	pkgs, err := (&Loader{Dir: dir}).Load(overlay)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		t.Fatalf("loaded %d packages, want 2", len(pkgs))
	}
	for _, p := range pkgs {
		if len(p.TypeErrors) > 0 || len(p.Errors) > 0 {
			t.Fatalf("%s: errors %v %v", p.PkgPath, p.TypeErrors, p.Errors)
		}
		if p.Types == nil || p.TypesInfo == nil || len(p.Syntax) == 0 {
			t.Fatalf("%s: a package of the app has no types or no syntax", p.PkgPath)
		}
		if p.PkgPath != "app" {
			continue
		}
		if p.Types.Scope().Lookup("handler") == nil {
			t.Fatalf("the overlay file is not part of the package")
		}
		dep := p.Imports["net/http"]
		if dep == nil || dep.Types == nil || dep.Types.Scope().Lookup("Handler") == nil {
			t.Fatalf("net/http has no types")
		}
		if len(dep.Syntax) > 0 {
			t.Fatalf("NFR-05: the loader parsed net/http from source")
		}
	}
}
