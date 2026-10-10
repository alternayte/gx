package compiler

import (
	"go/ast"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// classesPath is the class list the compiler writes for Tailwind
// (REQ-STY-02).
const classesPath = ".gx/classes.txt"

// collectClasses returns every class the compiler can see: static class
// values, class: directives and the string literals of Go files in packages
// that hold .gx files (REQ-STY-02). The result is sorted and unique.
func collectClasses(dirs []string, l *loader, pkgs []*packages.Package) []string {
	seen := map[string]bool{}
	add := func(text string) {
		for _, class := range strings.Fields(text) {
			if class != "" {
				seen[class] = true
			}
		}
	}
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			collectFileClasses(f.Body, add)
		}
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			path := pkg.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind.String() != "STRING" {
					return true
				}
				if s, ok := unquoteGo(lit.Value); ok {
					add(s)
				}
				return true
			})
		}
	}
	out := make([]string, 0, len(seen))
	for class := range seen {
		out = append(out, class)
	}
	sort.Strings(out)
	return out
}

// collectFileClasses walks a .gx body for class strings.
func collectFileClasses(nodes []Node, add func(string)) {
	for _, node := range nodes {
		switch t := node.(type) {
		case *Element:
			for _, a := range t.Attrs {
				switch {
				case a.Name == "class" && a.Kind == AttrString:
					add(a.Value)
				case a.Name == "class" && a.Kind == AttrExpr:
					collectQuoted(a.Value, add)
				case strings.HasPrefix(a.Name, "class:"):
					add(strings.TrimPrefix(a.Name, "class:"))
				case strings.HasSuffix(a.Name, "Class") && a.Kind == AttrString:
					// A prop that holds classes, such as bodyClass of
					// gx.Head: its value is in generated code only, which
					// the class list does not read.
					add(a.Value)
				case strings.HasSuffix(a.Name, "Class") && a.Kind == AttrExpr:
					collectQuoted(a.Value, add)
				}
			}
			collectFileClasses(t.Children, add)
		case *Control:
			collectFileClasses(t.Body, add)
			collectFileClasses(t.Else, add)
			for _, c := range t.Cases {
				collectFileClasses(c.Body, add)
			}
		}
	}
}

// collectQuoted adds the quoted strings of an expression.
func collectQuoted(expr string, add func(string)) {
	for i := 0; i < len(expr); i++ {
		if expr[i] != '"' && expr[i] != '\'' && expr[i] != '`' {
			continue
		}
		q := expr[i]
		j := i + 1
		for j < len(expr) && expr[j] != q {
			if expr[j] == '\\' {
				j++
			}
			j++
		}
		if j <= len(expr) {
			add(expr[i+1 : j])
		}
		i = j
	}
}

// unquoteGo unquotes a Go string literal.
func unquoteGo(lit string) (string, bool) {
	if len(lit) < 2 {
		return "", false
	}
	if lit[0] == '`' {
		return strings.ReplaceAll(lit[1:len(lit)-1], "\r", ""), true
	}
	if lit[0] != '"' {
		return "", false
	}
	out, err := strconv.Unquote(lit)
	return out, err == nil
}

// classesBytes renders the class list.
func classesBytes(classes []string) []byte {
	var b strings.Builder
	for _, class := range classes {
		b.WriteString(class)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// classesFilePath is the output path of the class list.
func classesFilePath(root string) string {
	return filepath.Join(root, classesPath)
}
