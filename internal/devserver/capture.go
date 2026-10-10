package devserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// fixtureName is the form of the name of a captured fixture.
var fixtureName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,63}$`)

// addFixture writes one entry into the gx.Fixtures literal of
// <component>.fixtures.go in dir, and makes the file when the component has
// none (REQ-AI-12). entry is the value of the entry as Go source, and
// imports are the packages that it names.
func addFixture(dir, component, name, entry string, imports []string) error {
	if !fixtureName.MatchString(name) {
		return fmt.Errorf("the name %q is not a fixture name: use letters and digits, and start with a letter", name)
	}
	path := filepath.Join(dir, component+".fixtures.go")
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		pkg, err := packageName(dir)
		if err != nil {
			return err
		}
		src = []byte("package " + pkg + "\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"// " + component + "Fixtures are the examples of " + component + " in the dev gallery.\n" +
			"var " + component + "Fixtures = gx.Fixtures[" + component + "Props]{}\n")
	} else if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("%s does not parse: %w", filepath.Base(path), err)
	}
	lit := fixturesLiteral(file)
	if lit == nil {
		return fmt.Errorf("%s has no gx.Fixtures literal; add `var %sFixtures = gx.Fixtures[%sProps]{}`", filepath.Base(path), component, component)
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
			if s, err := strconv.Unquote(key.Value); err == nil && s == name {
				return fmt.Errorf("the fixture %q exists in %s; give a different name", name, filepath.Base(path))
			}
		}
	}

	// The entry goes before the closing brace, on its own line.
	end := fset.Position(lit.Rbrace).Offset
	var b bytes.Buffer
	b.Write(src[:end])
	if n := len(lit.Elts); n > 0 {
		last := fset.Position(lit.Elts[n-1].End()).Offset
		if !bytes.Contains(src[last:end], []byte(",")) {
			b.WriteString(",")
		}
	}
	if !bytes.HasSuffix(bytes.TrimRight(b.Bytes(), " \t"), []byte("\n")) {
		b.WriteString("\n")
	}
	b.WriteString(strconv.Quote(name) + ": " + entry + ",\n")
	b.Write(src[end:])

	fset = token.NewFileSet()
	file, err = parser.ParseFile(fset, path, b.Bytes(), parser.ParseComments)
	if err != nil {
		return fmt.Errorf("the entry of the fixture is not Go source: %w", err)
	}
	for _, imp := range imports {
		astutil.AddImport(fset, file, imp)
	}
	var out bytes.Buffer
	if err := format.Node(&out, fset, file); err != nil {
		return err
	}
	// A second pass puts a new line of the literal in its columns.
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

// fixturesLiteral finds the composite literal of the type gx.Fixtures[...].
func fixturesLiteral(file *ast.File) *ast.CompositeLit {
	var found *ast.CompositeLit
	isFixtures := func(expr ast.Expr) bool {
		index, ok := expr.(*ast.IndexExpr)
		if !ok {
			return false
		}
		sel, ok := index.X.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "Fixtures"
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range vs.Values {
				lit, ok := value.(*ast.CompositeLit)
				if !ok {
					continue
				}
				if (lit.Type != nil && isFixtures(lit.Type)) || (lit.Type == nil && vs.Type != nil && isFixtures(vs.Type)) {
					if found == nil {
						found = lit
					}
				}
			}
		}
	}
	return found
}

// packageName reads the package name of the Go files in dir.
func packageName(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, e.Name()), nil, parser.PackageClauseOnly)
		if err == nil {
			return file.Name.Name, nil
		}
	}
	return "", fmt.Errorf("%s has no Go file; run gx generate", dir)
}

// packageDir returns the directory of a package of the app module.
func packageDir(root, pkgPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	module := ""
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			module = strings.Trim(strings.TrimSpace(rest), `"`)
			break
		}
	}
	if module == "" {
		return "", fmt.Errorf("go.mod has no module path")
	}
	if pkgPath == module {
		return root, nil
	}
	rel, ok := strings.CutPrefix(pkgPath, module+"/")
	if !ok || strings.Contains(rel, "..") {
		return "", fmt.Errorf("the package %s is not a package of this module; the capture writes only into the app", pkgPath)
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

// sameSite reports whether a request comes from a page of the dev server.
// The capture writes a file, so a page of a different site cannot ask for
// it.
func sameSite(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

// serveCapture saves the props of the last render of one component as a
// named fixture (REQ-AI-12). The app gives the props as Go source; this
// server writes the file, and the watcher then builds the app again.
func (s *server) serveCapture(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
	}
	if !sameSite(r) {
		fail(http.StatusForbidden, "a capture comes from a page of the dev server only")
		return
	}
	var in struct {
		Component string `json:"component"`
		Package   string `json:"package"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		fail(http.StatusBadRequest, err.Error())
		return
	}
	if !fixtureName.MatchString(in.Name) {
		fail(http.StatusUnprocessableEntity, fmt.Sprintf("the name %q is not a fixture name: use letters and digits, and start with a letter", in.Name))
		return
	}
	dir, err := packageDir(s.opt.Dir, in.Package)
	if err != nil {
		fail(http.StatusUnprocessableEntity, err.Error())
		return
	}
	target := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(s.appPort)) + "/_gx/dev/props/fixture?" +
		url.Values{"component": {in.Component}, "package": {in.Package}}.Encode()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		fail(http.StatusInternalServerError, err.Error())
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(http.StatusBadGateway, "the app does not answer: "+err.Error())
		return
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out struct {
		Fixture string   `json:"fixture"`
		Imports []string `json:"imports"`
		Error   string   `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil || (res.StatusCode != http.StatusOK && out.Error == "") {
		fail(http.StatusBadGateway, fmt.Sprintf("the app answers %d to the capture", res.StatusCode))
		return
	}
	if res.StatusCode != http.StatusOK {
		fail(res.StatusCode, out.Error)
		return
	}
	if err := addFixture(dir, in.Component, in.Name, out.Fixture, out.Imports); err != nil {
		fail(http.StatusUnprocessableEntity, err.Error())
		return
	}
	rel, err := filepath.Rel(s.opt.Dir, filepath.Join(dir, in.Component+".fixtures.go"))
	if err != nil {
		rel = in.Component + ".fixtures.go"
	}
	fmt.Fprintf(s.opt.Log, "gx dev: saved the fixture %s of %s in %s\n", in.Name, in.Component, filepath.ToSlash(rel))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"file": filepath.ToSlash(rel), "name": in.Name})
}
