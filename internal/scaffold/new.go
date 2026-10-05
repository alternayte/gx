package scaffold

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

var typeName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

// Kinds lists what `gx new` can add.
var Kinds = []string{"page", "action", "form", "component", "slice"}

// New adds typed code to the app in dir (REQ-DEV-10) and returns the files
// it wrote or changed, relative to dir. The target of a page, an action or
// a form is "<slice>/<Name>"; of a component "<dir>/<Name>"; of a slice
// its package name.
func New(dir, kind, target string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	module, err := modulePath(dir)
	if err != nil {
		return nil, err
	}
	d := data{Module: module}
	var changed []string
	switch kind {
	case "slice":
		changed, err = newSlice(dir, target, d)
	case "page", "action", "form":
		changed, err = newInSlice(dir, kind, target, d)
	case "component":
		changed, err = newComponent(dir, target, d)
	default:
		return nil, fmt.Errorf("unknown kind %q; gx new makes a %s", kind, strings.Join(Kinds, ", "))
	}
	if err != nil {
		return nil, err
	}
	generated, err := Generate(dir)
	if err != nil {
		return changed, err
	}
	return append(changed, generated...), nil
}

// modulePath reads the module path of the app.
func modulePath(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("%s holds no go.mod; run gx new in the root of an app", dir)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("%s/go.mod has no module line", dir)
}

// newSlice writes a slice with one page and mounts it.
func newSlice(dir, name string, d data) ([]string, error) {
	if !packageName.MatchString(name) {
		return nil, fmt.Errorf("slice name %q: a slice is a Go package, so its name is lower-case letters and digits", name)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return nil, fmt.Errorf("%s exists; nothing is written", name)
	}
	d.Slice, d.Label = name, upperFirst(name)
	files := map[string]string{}
	for rel, text := range map[string]string{
		name + "/route/route.go":  sliceRoute,
		name + "/" + name + ".go": sliceGo,
		name + "/IndexView.gx":    sliceView,
	} {
		body, err := render(rel, text, d)
		if err != nil {
			return nil, err
		}
		files[rel] = body
	}
	written, err := writeNew(dir, files)
	if err != nil {
		return nil, err
	}
	main := filepath.Join(dir, "cmd", "app", "main.go")
	mounted, err := mountSlice(main, d.Module+"/"+name, name)
	if err != nil {
		return written, err
	}
	if mounted {
		written = append(written, "cmd/app/main.go")
	} else {
		fmt.Fprintf(os.Stderr, "gx new: add %s.Routes to the Group call of the app; cmd/app/main.go has no single Group call\n", name)
	}
	return written, nil
}

// newInSlice adds a page, an action or a form to an existing slice.
func newInSlice(dir, kind, target string, d data) ([]string, error) {
	slice, name, ok := strings.Cut(filepath.ToSlash(target), "/")
	if !ok || strings.Contains(name, "/") {
		return nil, fmt.Errorf("%s name %q: write <slice>/<Name>, for example shop/Detail", kind, target)
	}
	if !typeName.MatchString(name) {
		return nil, fmt.Errorf("%s name %q: the name is a Go type, so it starts with an upper-case letter", kind, name)
	}
	routeFile := filepath.Join(dir, slice, "route", "route.go")
	routeSrc, err := os.ReadFile(routeFile)
	if err != nil {
		return nil, fmt.Errorf("no slice %q (%s/route/route.go is missing); run gx new slice %s first", slice, slice, slice)
	}
	d.Slice, d.Type, d.Kebab, d.Label = slice, name, kebab(name), spaced(name)
	declares := func(typ string) bool {
		return regexp.MustCompile(`(?m)^type ` + typ + `\b`).Match(routeSrc)
	}
	if declares(name) || (kind == "form" && declares(name+"Page")) {
		return nil, fmt.Errorf("%s/route/route.go already declares %s; nothing is written", slice, name)
	}
	snake := strings.ReplaceAll(d.Kebab, "-", "_")
	var files map[string]string
	var routeText string
	var collect []string
	switch kind {
	case "page":
		files = map[string]string{slice + "/" + snake + "_page.go": pageGo, slice + "/" + name + "View.gx": pageView}
		routeText, collect = pageRoute, []string{name + "Page"}
	case "action":
		files = map[string]string{slice + "/" + snake + "_action.go": actionGo}
		routeText, collect = actionRoute, []string{name}
	case "form":
		files = map[string]string{slice + "/" + snake + "_form.go": formGo, slice + "/" + name + "View.gx": formView}
		routeText, collect = formRoute, []string{name, name + "Page"}
	}
	rendered := map[string]string{}
	for rel, text := range files {
		body, err := render(rel, text, d)
		if err != nil {
			return nil, err
		}
		rendered[rel] = body
	}
	addition, err := render("route", routeText, d)
	if err != nil {
		return nil, err
	}
	// Find the route list before any write, so a slice with no list
	// changes nothing.
	listFile, listSrc, err := addToCollect(filepath.Join(dir, slice), collect)
	if err != nil {
		return nil, err
	}
	written, err := writeNew(dir, rendered)
	if err != nil {
		return nil, err
	}
	next := append(bytes.TrimRight(routeSrc, "\n"), []byte("\n\n"+addition)...)
	if err := os.WriteFile(routeFile, next, 0o644); err != nil {
		return written, err
	}
	if err := os.WriteFile(listFile, listSrc, 0o644); err != nil {
		return written, err
	}
	rel, _ := filepath.Rel(dir, listFile)
	return append(written, slice+"/route/route.go", filepath.ToSlash(rel)), nil
}

// newComponent writes a component and its fixtures.
func newComponent(dir, target string, d data) ([]string, error) {
	target = filepath.ToSlash(target)
	i := strings.LastIndex(target, "/")
	if i <= 0 {
		return nil, fmt.Errorf("component name %q: write <dir>/<Name>, for example ui/badge/Badge", target)
	}
	pkgDir, name := target[:i], target[i+1:]
	if !typeName.MatchString(name) {
		return nil, fmt.Errorf("component name %q: the name is a Go identifier, so it starts with an upper-case letter", name)
	}
	pkg := pkgDir[strings.LastIndex(pkgDir, "/")+1:]
	// A package that exists keeps its name; a new one takes the name of
	// its directory.
	if existing := existingPackage(filepath.Join(dir, filepath.FromSlash(pkgDir))); existing != "" {
		pkg = existing
	} else if !packageName.MatchString(pkg) {
		return nil, fmt.Errorf("component directory %q: the last part is a Go package, so it is lower-case letters and digits", pkgDir)
	}
	d.Type, d.Pkg, d.Label = name, pkg, spaced(name)
	files := map[string]string{}
	for rel, text := range map[string]string{
		pkgDir + "/" + name + ".gx":          componentGx,
		pkgDir + "/" + name + ".fixtures.go": componentFixtures,
	} {
		body, err := render(rel, text, d)
		if err != nil {
			return nil, err
		}
		files[rel] = body
	}
	return writeNew(dir, files)
}

// existingPackage returns the package name of the Go or .gx files in dir,
// or "".
func existingPackage(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".gx")) || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "package "); ok {
				return strings.TrimSpace(rest)
			}
		}
	}
	return ""
}

// addToCollect returns the file of a slice that holds its gx.Collect list,
// with names added to the call.
func addToCollect(sliceDir string, names []string) (string, []byte, error) {
	entries, err := os.ReadDir(sliceDir)
	if err != nil {
		return "", nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_gx.go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(sliceDir, name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return "", nil, err
		}
		var call *ast.CallExpr
		ast.Inspect(file, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || call != nil {
				return call == nil
			}
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Collect" {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "gx" {
					call = c
				}
			}
			return call == nil
		})
		if call == nil {
			continue
		}
		for _, n := range names {
			call.Args = append(call.Args, ast.NewIdent(n))
		}
		var b bytes.Buffer
		if err := format.Node(&b, fset, file); err != nil {
			return "", nil, err
		}
		return path, b.Bytes(), nil
	}
	return "", nil, fmt.Errorf("%s has no gx.Collect call; add one, for example var Routes = gx.Collect()", filepath.Base(sliceDir))
}

// mountSlice adds <pkg>.Routes to the Group call of the app main and
// imports the slice. It reports false when the file has no single Group
// call.
func mountSlice(mainFile, importPath, pkg string) (bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainFile, nil, parser.ParseComments)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var groups []*ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Group" {
				groups = append(groups, c)
			}
		}
		return true
	})
	if len(groups) != 1 {
		return false, nil
	}
	groups[0].Args = append(groups[0].Args, &ast.SelectorExpr{X: ast.NewIdent(pkg), Sel: ast.NewIdent("Routes")})
	astutil.AddImport(fset, file, importPath)
	var b bytes.Buffer
	if err := format.Node(&b, fset, file); err != nil {
		return false, err
	}
	return true, os.WriteFile(mainFile, b.Bytes(), 0o644)
}

// kebab turns "AddItem" into "add-item".
func kebab(name string) string {
	var b strings.Builder
	for i, r := range name {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('-')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// spaced turns "AddItem" into "Add item".
func spaced(name string) string {
	return upperFirst(strings.ReplaceAll(kebab(name), "-", " "))
}

const sliceRoute = `// Package route holds the route types of the «.Slice» slice. A route
// package holds only route types.
package route

import "` + GxModule + `"

// Index is the first page of the slice.
type Index struct {
	gx.Route ` + "`GET /«.Slice»`" + `
}
`

const sliceGo = `// Package «.Slice» is the «.Slice» slice: its pages, actions and views.
package «.Slice»

import (
	"` + GxModule + `"
	"«.Module»/«.Slice»/route"
)

// IndexPage is the first page of the slice. The loader returns the props of
// the view.
var IndexPage = gx.Page(func(c *gx.Ctx, in route.Index) (IndexViewProps, error) {
	return IndexViewProps{}, nil
}, IndexView)

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(IndexPage)
`

const sliceView = `package «.Slice»

<gx.Head title="«.Label»" />
<h1 class="text-2xl font-semibold">«.Label»</h1>
`

const pageRoute = `// «.Type» is the «.Label» page.
type «.Type» struct {
	gx.Route ` + "`GET /«.Slice»/«.Kebab»`" + `
}
`

const pageGo = `package «.Slice»

import (
	"` + GxModule + `"
	"«.Module»/«.Slice»/route"
)

// «.Type»Page is the «.Label» page. The loader returns the props of the
// view.
var «.Type»Page = gx.Page(func(c *gx.Ctx, in route.«.Type») («.Type»ViewProps, error) {
	return «.Type»ViewProps{}, nil
}, «.Type»View)
`

const pageView = `package «.Slice»

<gx.Head title="«.Label»" />
<h1 class="text-2xl font-semibold">«.Label»</h1>
`

const actionRoute = `// «.Type» is the input of the «.Label» action. Invoke it from a .gx
// file with on:click={route.«.Type»{}}.
type «.Type» struct {
	gx.Route ` + "`POST /«.Slice»/«.Kebab»`" + `
}
`

const actionGo = `package «.Slice»

import (
	"` + GxModule + `"
	"«.Module»/«.Slice»/route"
)

// «.Type» is the «.Label» action. Answer with c.Patch, c.SetSignals,
// c.Redirect or c.Toast; nil answers 204.
var «.Type» = gx.Action(func(c *gx.Ctx, in route.«.Type») error {
	return nil
})
`

const formRoute = `// «.Type»Page is the page that shows the «.Label» form.
type «.Type»Page struct {
	gx.Route ` + "`GET /«.Slice»/«.Kebab»`" + `
}

// «.Type» is the input of the «.Label» form.
type «.Type» struct {
	gx.Route ` + "`POST /«.Slice»/«.Kebab»`" + `
	Email    string
}

// Rules holds the checks of the form. The browser, the live validation and
// the submit all use them.
func (in *«.Type») Rules() gx.Rules {
	return gx.Rules{
		gx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),
	}
}
`

const formGo = `package «.Slice»

import (
	"` + GxModule + `"
	"«.Module»/«.Slice»/route"
)

// «.Type» is the «.Label» form. The handler runs only when every rule
// passes. gx.FieldError(&in.Email, "key") shows an error on one field.
var «.Type» = gx.Form(func(c *gx.Ctx, in *route.«.Type») error {
	return c.Redirect(route.«.Type»Page{})
}, «.Type»View)

// «.Type»Page shows the empty form.
var «.Type»Page = gx.Page(func(c *gx.Ctx, in route.«.Type»Page) («.Type»ViewProps, error) {
	return «.Type».Props(&route.«.Type»{}), nil
}, «.Type»View)
`

const formView = `package «.Slice»

import "«.Module»/«.Slice»/route"

props {
  // F is the typed form: one field per input field.
  F route.«.Type»Form
}

<gx.Head title="«.Label»" />
<h1 class="text-2xl font-semibold">«.Label»</h1>
<form {...p.F.Attrs()} class="mt-4 grid max-w-sm gap-2">
  <label for={p.F.Email.ID}>Email</label>
  <input {...p.F.Email.Attrs()} type="email" data-gx-validate="blur" class="rounded-md border border-border px-3 py-2" />
  <p id={p.F.Email.ID + "-error"} role="alert" class="text-sm text-destructive">{p.F.Email.Error}</p>
  <button type="submit" class="rounded-md bg-primary px-3 py-1.5 text-sm text-primary-foreground">Send</button>
</form>
`

const componentGx = `package «.Pkg»

props {
  // Children is the content.
  Children gx.Node
}

<div class="rounded-md border border-border p-4">{p.Children}</div>
`

const componentFixtures = `package «.Pkg»

import "` + GxModule + `"

// «.Type»Fixtures are the examples of «.Type» in the dev gallery.
var «.Type»Fixtures = gx.Fixtures[«.Type»Props]{
	"Default": {Children: gx.Text("«.Label»")},
}
`
