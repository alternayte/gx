package compiler

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Module is the enclosing Go module of a package.
type Module struct {
	Dir  string
	Path string
}

// Package is the parsed .gx files of one directory.
type Package struct {
	Dir    string
	Module *Module
	Files  map[string]*File // base name without .gx
	Diags  []Diagnostic
}

// Prop is one component prop, from the props block.
type Prop struct {
	Name       string
	Doc        string
	Type       string
	Default    string
	HasDefault bool
}

// Component is one component definition.
type Component struct {
	Name  string
	File  *File
	Props []Prop
}

// component returns the component named name in this package.
func (p *Package) component(name string) (*Component, bool) {
	f, ok := p.Files[name]
	if !ok {
		return nil, false
	}
	c := &Component{Name: name, File: f}
	for _, fld := range f.Props {
		c.Props = append(c.Props, Prop{
			Name:       fld.Name,
			Doc:        fld.Doc,
			Type:       fld.Type,
			Default:    fld.Default,
			HasDefault: fld.HasDefault,
		})
	}
	return c, true
}

// loader caches parsed packages by directory.
type loader struct {
	pkgs    map[string]*Package
	overlay map[string][]byte
}

func newLoader() *loader {
	return &loader{pkgs: map[string]*Package{}}
}

func (l *loader) load(dir string) *Package {
	dir = filepath.Clean(dir)
	if p, ok := l.pkgs[dir]; ok {
		return p
	}
	p := &Package{Dir: dir, Files: map[string]*File{}, Module: findModule(dir)}
	l.pkgs[dir] = p
	entries, err := os.ReadDir(dir)
	if err != nil {
		return p
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gx") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var src []byte
		if data, ok := l.overlay[filepath.Clean(path)]; ok {
			src = data
		} else {
			var readErr error
			src, readErr = os.ReadFile(path)
			if readErr != nil {
				continue
			}
		}
		f, diags := ParseFile(path, src)
		p.Diags = append(p.Diags, diags...)
		if f != nil {
			p.Files[strings.TrimSuffix(e.Name(), ".gx")] = f
		}
	}
	return p
}

// importedPackage resolves a qualifier in function file to a package.
func (l *loader) importedPackage(p *Package, file *File, qual string) (*Package, bool) {
	if p.Module == nil {
		return nil, false
	}
	path, ok := findImport(file, qual)
	if !ok {
		return nil, false
	}
	switch {
	case path == p.Module.Path:
		return l.load(p.Module.Dir), true
	case strings.HasPrefix(path, p.Module.Path+"/"):
		rel := strings.TrimPrefix(path, p.Module.Path+"/")
		return l.load(filepath.Join(p.Module.Dir, filepath.FromSlash(rel))), true
	default:
		return nil, false
	}
}

// findImport returns the import path for the qualifier qual.
func findImport(file *File, qual string) (string, bool) {
	for _, im := range file.Imports {
		alias, path, ok := splitImport(im.Raw)
		if ok && alias == qual {
			return path, true
		}
	}
	return "", false
}

// splitImport splits an import spec into qualifier and path.
func splitImport(raw string) (alias, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if raw[0] == '"' {
		p, err := strconv.Unquote(raw)
		if err != nil {
			return "", "", false
		}
		return pathBase(p), p, true
	}
	i := strings.IndexAny(raw, " \t")
	if i < 0 {
		p, err := strconv.Unquote(raw)
		if err != nil {
			return "", "", false
		}
		return pathBase(p), p, true
	}
	alias = raw[:i]
	rest := strings.TrimSpace(raw[i:])
	p, err := strconv.Unquote(rest)
	if err != nil {
		return "", "", false
	}
	return alias, p, true
}

func pathBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// findModule walks up from dir to the nearest go.mod.
func findModule(dir string) *Module {
	for d := filepath.Clean(dir); ; {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			if path := modulePath(data); path != "" {
				return &Module{Dir: d, Path: path}
			}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil
		}
		d = parent
	}
}

func modulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module"); ok {
			rest = strings.TrimSpace(rest)
			rest = strings.Trim(rest, `"`)
			if rest != "" {
				return rest
			}
		}
	}
	return ""
}

// lowerFirst lowercases the first byte of an exported identifier.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// componentTag splits an element tag into a package qualifier and a component
// name. A tag is a component when it is exported and dotted, or when it starts
// with an uppercase letter.
func componentTag(tag string) (qual, name string, ok bool) {
	if tag == "" || tag[0] == ':' || tag[0] == '-' {
		return "", "", false
	}
	if i := strings.IndexByte(tag, '.'); i > 0 {
		qual, name = tag[:i], tag[i+1:]
		return qual, name, isExportedIdent(name)
	}
	if tag[0] >= 'A' && tag[0] <= 'Z' {
		return "", tag, true
	}
	return "", "", false
}

// resolveComponent finds the component a tag refers to.
func resolveComponent(l *loader, pkg *Package, file *File, qual, name string) (*Component, *Package, string) {
	if qual == "" {
		comp, _ := pkg.component(name)
		return comp, pkg, ""
	}
	if name == "Head" && qual == "gx" {
		return gxHeadComponent(), nil, qual
	}
	if path, ok := findImport(file, qual); ok && path == "github.com/alternayte/gx" && name == "Head" {
		return gxHeadComponent(), nil, qual
	}
	target, ok := l.importedPackage(pkg, file, qual)
	if !ok {
		return nil, nil, ""
	}
	comp, _ := target.component(name)
	return comp, target, qual
}

// gxHeadComponent is the runtime Head component (REQ-RTE-11).
func gxHeadComponent() *Component {
	return &Component{Name: "Head", Props: []Prop{
		{Name: "Title", Type: "string", HasDefault: true, Default: `""`},
		{Name: "Meta", Type: "[]gx.Meta", HasDefault: true, Default: "nil"},
		{Name: "Links", Type: "[]gx.Link", HasDefault: true, Default: "nil"},
		{Name: "Lang", Type: "string", HasDefault: true, Default: `""`},
		{Name: "HtmlClass", Type: "string", HasDefault: true, Default: `""`},
		{Name: "BodyClass", Type: "string", HasDefault: true, Default: `""`},
	}}
}

// slotTypeArg returns T of a gx.Slot[T] prop type.
func slotTypeArg(typ string) (string, bool) {
	const prefix = "gx.Slot["
	if !strings.HasPrefix(typ, prefix) || !strings.HasSuffix(typ, "]") {
		return "", false
	}
	return strings.TrimSpace(typ[len(prefix) : len(typ)-1]), true
}

// slotLetName returns the bound name of a <:name let={v}> slot.
func slotLetName(slot *Element) string {
	if slot == nil {
		return "_"
	}
	for i := range slot.Attrs {
		a := &slot.Attrs[i]
		if a.Kind == AttrExpr && a.Name == "let" {
			if name := strings.TrimSpace(a.Value); name != "" {
				return name
			}
		}
	}
	return "_"
}

// fragmentElements returns every element that carries a #fragment, in source
// order.
func fragmentElements(ns []Node) []*Element {
	var out []*Element
	walkElements(ns, func(el *Element) {
		for i := range el.Attrs {
			if el.Attrs[i].Kind == AttrFragment {
				out = append(out, el)
				return
			}
		}
	})
	return out
}

// fragmentParams returns the text inside the parentheses of a #name(params)
// fragment.
func fragmentParams(el *Element) string {
	for i := range el.Attrs {
		if el.Attrs[i].Kind == AttrFragment {
			return strings.TrimSpace(el.Attrs[i].Value)
		}
	}
	return ""
}

// splitParams splits a Go parameter list at top-level commas.
func splitParams(params string) []string {
	if strings.TrimSpace(params) == "" {
		return nil
	}
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(params[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(params[start:]))
	return out
}

// firstIdent returns the first identifier of a parameter declaration.
func firstIdent(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '_' || isLetter(s[i]) {
			j := i
			for j < len(s) && (s[j] == '_' || isLetter(s[j]) || isDigit(s[j])) {
				j++
			}
			return s[i:j]
		}
	}
	return ""
}

// isDirective reports whether an attribute name is a gx directive rather than
// a component prop.
func isDirective(name string) bool {
	if strings.Contains(name, ":") {
		return true
	}
	switch name {
	case "show", "text", "key", "transition":
		return true
	}
	return false
}
