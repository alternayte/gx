package compiler

import (
	"go/ast"
	"strconv"
	"strings"
)

// optimisticPrefix starts the name of an optimistic directive (REQ-ACT-18).
const optimisticPrefix = "optimistic:"

// lowerOptimistic gives each optimistic:<event> directive of a file the name
// of a capture handler of the event. The handler then runs before the on:
// handler of the element that invokes the action, and each pass reads it as
// an on: handler with signal statements.
func lowerOptimistic(f *File) {
	walkElements(f.Body, func(el *Element) {
		for i := range el.Attrs {
			a := &el.Attrs[i]
			if event, ok := strings.CutPrefix(a.Name, optimisticPrefix); ok && a.Kind == AttrExpr {
				a.Optimistic = true
				a.Name = "on:" + event + ".capture"
			}
		}
	})
}

// onEvent returns the event of an on: attribute name, with no modifier.
func onEvent(name string) string {
	event, _ := splitOnSpec(strings.TrimPrefix(name, "on:"))
	return event
}

// optimisticAction returns the on: attribute of the element that invokes an
// action on the event of the optimistic directive a, or nil.
func (g *gen) optimisticAction(el *Element, a *Attr) *Attr {
	for i := range el.Attrs {
		b := &el.Attrs[i]
		if b == a || b.Optimistic || b.Kind != AttrExpr || !strings.HasPrefix(b.Name, "on:") || onEvent(b.Name) != onEvent(a.Name) {
			continue
		}
		if key := namedTypeKey(g.res.types[b]); key != "" && g.res.routeKeys[key] {
			return b
		}
	}
	return nil
}

// hasOptimistic reports whether the element has an optimistic directive for
// the event of the on: attribute a.
func hasOptimistic(el *Element, a *Attr) bool {
	for i := range el.Attrs {
		b := &el.Attrs[i]
		if b.Optimistic && onEvent(b.Name) == onEvent(a.Name) {
			return true
		}
	}
	return false
}

// keepExpr returns the Go expression of the text that saves each signal that
// the statements of an optimistic directive write. The runtime puts the
// values back when the action fails.
func (g *gen) keepExpr(site *clientSite) (string, bool) {
	block, ok := site.node.(*ast.BlockStmt)
	if !ok {
		return "", false
	}
	base := strconv.Quote(g.file.Package + "." + g.name)
	var parts []string
	seen := map[string]bool{}
	add := func(e ast.Expr) {
		id, ok := e.(*ast.Ident)
		if !ok {
			return
		}
		name, ok := strings.CutPrefix(id.Name, "_gxSig_")
		if !ok || seen[name] {
			return
		}
		seen[name] = true
		parts = append(parts, "gx.KeepSignal("+base+", "+g.keyExpr()+", "+strconv.Quote(lowerFirst(name))+")")
	}
	ast.Inspect(block, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				add(lhs)
			}
		case *ast.IncDecStmt:
			add(s.X)
		}
		return true
	})
	if len(parts) == 0 {
		return "", false
	}
	return "gx.Keep(" + strings.Join(parts, ", ") + ")", true
}
