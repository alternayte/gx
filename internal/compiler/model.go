package compiler

import (
	"path/filepath"
	"sort"
	"strings"
)

// Symbol is one typed identifier in a .gx file (REQ-DEV-08). ObjectFile is
// the definition position, mapped through the //line directives.
type Symbol struct {
	File       string
	Line       int
	Col        int
	EndCol     int
	Text       string
	Type       string
	ObjectFile string
	ObjectLine int
	ObjectCol  int
}

// ComponentRef is one component of the module (REQ-DEV-08).
type ComponentRef struct {
	Name    string
	Dir     string
	PkgPath string
	File    *File
	Props   []Prop
}

// RouteRef is one route struct of the module (REQ-DEV-08, REQ-DEV-11).
type RouteRef struct {
	Key        string // import path and type name
	PkgPath    string
	Name       string
	Pattern    string
	Method     string
	File       string
	Line       int
	Col        int
	Fields     []string
	Registered bool
}

// Model is the whole-module index of one analysis pass (REQ-DEV-11). The
// LSP answers completion, hover and definition from it.
type Model struct {
	Root       string
	Files      []*File
	Components []ComponentRef
	Routes     []RouteRef
	Symbols    []Symbol
	Packages   map[string][]string // import path -> exported names
}

// Model runs the analysis and returns the whole-module index. It reuses the
// session cache, so an edit only re-checks the changed file (REQ-DEV-11).
func (s *Session) Model(root string) (*Model, []Diagnostic) {
	root = absoluteRoot(root)
	_, diags := s.Generate(root)
	s.mu.Lock()
	defer s.mu.Unlock()
	m := &Model{Root: root, Packages: map[string][]string{}}
	for _, dir := range s.dirs {
		p := s.l.load(dir)
		for _, f := range p.Files {
			m.Files = append(m.Files, f)
		}
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].File < m.Files[j].File })
	for _, f := range m.Files {
		dir := filepath.Dir(f.File)
		p := s.l.load(dir)
		pkgPath := ""
		if p.Module != nil {
			if rel, err := filepath.Rel(p.Module.Dir, dir); err == nil {
				pkgPath = p.Module.Path
				if rel != "." {
					pkgPath += "/" + filepath.ToSlash(rel)
				}
			}
		}
		comp, ok := p.component(componentName(f))
		if !ok {
			continue
		}
		m.Components = append(m.Components, ComponentRef{
			Name: comp.Name, Dir: dir, PkgPath: pkgPath, File: f, Props: comp.Props,
		})
	}
	sort.Slice(m.Components, func(i, j int) bool {
		if m.Components[i].PkgPath != m.Components[j].PkgPath {
			return m.Components[i].PkgPath < m.Components[j].PkgPath
		}
		return m.Components[i].Name < m.Components[j].Name
	})
	if s.res != nil {
		for _, d := range s.res.routes {
			if d.pkg == nil {
				continue
			}
			ref := RouteRef{
				Key:     d.pkg.PkgPath + "." + d.name,
				PkgPath: d.pkg.PkgPath,
				Name:    d.name,
				Pattern: d.pattern,
				File:    d.pos.Filename,
				Line:    d.pos.Line,
				Col:     d.pos.Column,
			}
			ref.Method, _, _ = strings.Cut(d.pattern, " ")
			for _, f := range d.fields {
				ref.Fields = append(ref.Fields, f.name)
			}
			_, ref.Registered = s.res.actions[ref.Key]
			m.Routes = append(m.Routes, ref)
		}
		sort.Slice(m.Routes, func(i, j int) bool { return m.Routes[i].Key < m.Routes[j].Key })
		for _, syms := range s.res.symbols {
			m.Symbols = append(m.Symbols, syms...)
		}
		sort.Slice(m.Symbols, func(i, j int) bool {
			if m.Symbols[i].File != m.Symbols[j].File {
				return m.Symbols[i].File < m.Symbols[j].File
			}
			if m.Symbols[i].Line != m.Symbols[j].Line {
				return m.Symbols[i].Line < m.Symbols[j].Line
			}
			return m.Symbols[i].Col < m.Symbols[j].Col
		})
		for _, p := range s.res.pkgs {
			if p.Types == nil {
				continue
			}
			m.Packages[p.PkgPath] = p.Types.Scope().Names()
		}
	}
	return m, diags
}

// Diagnostics returns the diagnostics of the last Generate call.
func (s *Session) Diagnostics() []Diagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Diagnostic(nil), s.lastDiags...)
}

// Fragments returns the fragment elements of a component body
// (REQ-AUT-13).
func Fragments(f *File) []*Element { return fragmentElements(f.Body) }

// ImportPath returns the path of an import spec as written, without the
// alias and quotes (REQ-DEV-08).
func ImportPath(raw string) string {
	_, path, ok := splitImport(raw)
	if !ok {
		return ""
	}
	return path
}

// ImportQualifier returns the name an import spec is used by. An import
// without an alias uses the last path segment.
func ImportQualifier(raw string) string {
	alias, path, ok := splitImport(raw)
	if !ok {
		return ""
	}
	if alias != "" {
		return alias
	}
	return pathBase(path)
}
