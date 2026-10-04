package compiler

import (
	"strings"
)

// checkRuntimeClasses reports a class value built at runtime: fmt.Sprintf,
// fmt.Sprint or string concatenation (GX5003, REQ-STY-11).
func (l *loader) checkRuntimeClasses(dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			walkNodes(f.Body, func(n Node) {
				el, ok := n.(*Element)
				if !ok {
					return
				}
				for i := range el.Attrs {
					a := &el.Attrs[i]
					if a.Kind != AttrExpr || a.Name != "class" {
						continue
					}
					if !dynamicClassExpr(a.Value) {
						continue
					}
					out = append(out, Diagnostic{
						Code: CodeRuntimeClass,
						File: f.File,
						Line: a.ValueAt.Line,
						Col:  a.ValueAt.Col,
						Msg:  "class string is built at runtime; Tailwind cannot see it",
						Fix:  "use a static class or a gx.Enum value",
					})
				}
			})
		}
	}
	return out
}

// dynamicClassExpr reports whether an expression builds a class string at
// runtime.
func dynamicClassExpr(expr string) bool {
	s := strings.TrimSpace(expr)
	if strings.HasPrefix(s, "fmt.Sprintf(") || strings.HasPrefix(s, "fmt.Sprint(") {
		return true
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\'', '`':
			q := s[i]
			i++
			for i < len(s) && s[i] != q {
				if s[i] == '\\' {
					i++
				}
				i++
			}
		case '+':
			return true
		}
	}
	return false
}
