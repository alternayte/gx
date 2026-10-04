package compiler

import (
	"strings"
)

// checkTransitions reports two elements in one template that use the same
// transition value (GX5002, REQ-STY-08).
func checkTransitions(dirs []string, l *loader) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		for _, f := range p.Files {
			seen := map[string]bool{}
			walkNodes(f.Body, func(n Node) {
				el, ok := n.(*Element)
				if !ok {
					return
				}
				for i := range el.Attrs {
					a := &el.Attrs[i]
					if a.Kind != AttrExpr || a.Name != "transition" {
						continue
					}
					norm := strings.Join(strings.Fields(a.Value), " ")
					if norm == "" {
						continue
					}
					if seen[norm] {
						out = append(out, Diagnostic{
							Code: CodeTransition,
							File: f.File,
							Line: a.ValueAt.Line,
							Col:  a.ValueAt.Col,
							Msg:  "duplicate transition " + Quoted(norm) + "; the browser skips a transition with a duplicate name",
							Fix:  "give the elements different keys",
						})
						continue
					}
					seen[norm] = true
				}
			})
		}
	}
	return out
}
