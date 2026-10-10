package compiler

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// DeadReport is the report of gx check --dead (REQ-DEV-14): code of the
// module that nothing uses. It is a report and never a failure: a registry
// component that the app does not use yet, and a page that only an email
// links to, are correct.
type DeadReport struct {
	// Components are the components with no caller: no tag in a .gx file
	// and no call in a Go file.
	Components []DeadComponent `json:"components"`
	// Routes are the page routes with no typed link to them.
	Routes []DeadRoute `json:"routes"`
	// Classes are the classes that app/theme.css declares and that no
	// file of the app uses.
	Classes []string `json:"classes"`
}

// DeadComponent is one component with no caller.
type DeadComponent struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	File    string `json:"file"`
}

// DeadRoute is one page route with no typed link.
type DeadRoute struct {
	Pattern string `json:"pattern"`
	Type    string `json:"type"`
}

// Dead returns the dead-code report of the module under root.
func Dead(root string) (*DeadReport, []Diagnostic) {
	root = absoluteRoot(root)
	_, diags, res, l, dirs := generate(root, nil)
	if Failed(diags) || res == nil {
		return nil, diags
	}
	report := &DeadReport{Components: []DeadComponent{}, Routes: []DeadRoute{}, Classes: []string{}}

	// A component is used by a tag of a .gx file or by a name in a Go
	// file that a person wrote.
	type compKey struct{ dir, name string }
	used := map[compKey]bool{}
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkElements(f.Body, func(el *Element) {
				qual, name, ok := componentTag(el.Name)
				if !ok {
					return
				}
				if _, target, _ := resolveComponent(l, p, f, qual, name); target != nil {
					used[compKey{filepath.Clean(target.Dir), name}] = true
				}
			})
		}
	}
	// The route types that an expression of a .gx file or of a Go file
	// makes: a typed link, a redirect or an invocation.
	linked := map[string]bool{}
	for _, dir := range dirs {
		for _, f := range l.load(dir).Files {
			walkElements(f.Body, func(el *Element) {
				for i := range el.Attrs {
					// href={route.Show{ID: 1}}: the type of the value.
					if key := namedTypeKey(res.types[&el.Attrs[i]]); key != "" {
						linked[key] = true
					}
				}
			})
		}
	}
	// A link inside a larger expression of a template, for example
	// href={gx.URL(route.About{}.URL())}: the generated code of the
	// template has the expression, with its types. The generated file of
	// a route package has no link.
	for _, pkg := range res.pkgs {
		if pkg.Name == "route" {
			continue
		}
		for _, file := range pkg.Syntax {
			if !strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "_gx.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if lit, ok := n.(*ast.CompositeLit); ok {
					if key := namedTypeKey(pkg.TypesInfo.TypeOf(lit)); key != "" {
						linked[key] = true
					}
				}
				return true
			})
		}
	}
	sourceFiles(res.pkgs, func(pkg *packages.Package, file *ast.File) {
		path := pkg.Fset.PositionFor(file.Pos(), false).Filename
		fixtures := strings.HasSuffix(path, ".fixtures.go")
		ast.Inspect(file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.Ident:
				// The fixtures of a component are not a caller of it.
				if fn, ok := pkg.TypesInfo.Uses[t].(*types.Func); ok && !fixtures {
					if at := pkg.Fset.PositionFor(fn.Pos(), false).Filename; strings.HasSuffix(at, "_gx.go") {
						used[compKey{filepath.Clean(filepath.Dir(at)), fn.Name()}] = true
					}
				}
			case *ast.CompositeLit:
				if key := namedTypeKey(pkg.TypesInfo.TypeOf(t)); key != "" {
					linked[key] = true
				}
			}
			return true
		})
	})
	for _, dir := range dirs {
		p := l.load(dir)
		for _, name := range sortedFileNames(p.Files) {
			if used[compKey{filepath.Clean(p.Dir), name}] {
				continue
			}
			rel, err := filepath.Rel(root, p.Files[name].File)
			if err != nil {
				rel = p.Files[name].File
			}
			report.Components = append(report.Components, DeadComponent{Name: name, Package: filepath.ToSlash(filepath.Dir(rel)), File: filepath.ToSlash(rel)})
		}
	}
	sort.Slice(report.Components, func(i, j int) bool { return report.Components[i].File < report.Components[j].File })

	// The page routes: the key of each route that a gx.Page binds.
	for key := range res.routePages {
		if linked[key] {
			continue
		}
		pattern := key
		if def := res.routeDefs[key]; def != nil {
			pattern = def.pattern
		}
		report.Routes = append(report.Routes, DeadRoute{Pattern: pattern, Type: key})
	}
	sort.Slice(report.Routes, func(i, j int) bool { return report.Routes[i].Pattern < report.Routes[j].Pattern })

	// The classes of the theme.
	if theme, err := os.ReadFile(filepath.Join(root, "app", "theme.css")); err == nil {
		have := map[string]bool{}
		for _, class := range collectClasses(dirs, l, res.pkgs) {
			have[class] = true
		}
		seen := map[string]bool{}
		for _, m := range themeClass.FindAllSubmatch(stripCSSComments(theme), -1) {
			name := string(m[1]) + string(m[2])
			if name == "dark" || name == "light" || have[name] || seen[name] {
				continue
			}
			seen[name] = true
			report.Classes = append(report.Classes, name)
		}
		sort.Strings(report.Classes)
	}
	return report, nil
}

// themeClass finds a class that a theme declares: a class selector at the
// start of a rule, or a Tailwind @utility.
var themeClass = regexp.MustCompile(`(?m)^\s*\.([A-Za-z_][\w-]*)\s*(?:[,{:]|$)|@utility\s+([A-Za-z_][\w-]*)`)

var cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

func stripCSSComments(css []byte) []byte { return cssComment.ReplaceAll(css, nil) }
