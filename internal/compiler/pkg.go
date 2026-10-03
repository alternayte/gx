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
			Type:       fld.Type,
			Default:    fld.Default,
			HasDefault: fld.HasDefault,
		})
	}
	return c, true
}

// loader caches parsed packages by directory.
type loader struct {
	pkgs map[string]*Package
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
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		f, diags := ParseFile(filepath.Join(dir, e.Name()), src)
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

// isDirective reports whether an attribute name is a gx directive rather than
// a component prop.
func isDirective(name string) bool {
	if strings.Contains(name, ":") {
		return true
	}
	switch name {
	case "show", "text", "key":
		return true
	}
	return false
}
