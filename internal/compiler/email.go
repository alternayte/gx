package compiler

import "strings"

// emailPackage is the name of a package whose components are parts of an
// email (REQ-REG-15). The registry kit `email` has this name.
const emailPackage = "email"

// checkEmail returns GX6010 for each construct of a component of an email
// package that an email cannot hold (REQ-REG-17): a class attribute, a
// signal, a client expression, an action invocation, an island and a script
// element. An email client runs no script and reads no stylesheet of the
// site. gx.RenderEmail returns an error for the same construct in a
// different package.
func checkEmail(p *Package, f *File) []Diagnostic {
	if f.Package != emailPackage {
		return nil
	}
	var out []Diagnostic
	add := func(at Pos, what, fix string) {
		out = append(out, Diagnostic{
			Code: CodeEmail, File: f.File, Line: at.Line, Col: at.Col,
			Msg: "a component of an email package has " + what + "; an email client runs no script and reads no stylesheet of the site",
			Fix: fix,
		})
	}
	for _, s := range f.Signals {
		add(s.At, "the signal "+Quoted(s.Name), "remove the signal; give the value as a prop")
	}
	walkElements(f.Body, func(el *Element) {
		if el.Name == "script" {
			add(el.At, "a script element", "remove the script")
			return
		}
		if _, ok := p.Islands[el.Name]; ok {
			add(el.At, "the island <"+el.Name+">", "remove the island; an image can show its content")
			return
		}
		for i := range el.Attrs {
			a := &el.Attrs[i]
			switch {
			case a.Name == "class" || strings.HasPrefix(a.Name, "class:"):
				add(a.At, "a class attribute on <"+el.Name+">", "write the look in a style attribute")
			case strings.HasPrefix(a.Name, "bind:"):
				add(a.At, "a signal in "+a.Name, "remove the directive")
			case strings.HasPrefix(a.Name, "on:") && !a.Optimistic && !strings.Contains(a.Value, "$"):
				add(a.At, "an action invocation in "+a.Name, "use a link to a page of the site")
			case isClientDirective(a.Name):
				add(a.At, "a client expression in "+a.Name, "write the value with a Go expression of the props")
			}
		}
	})
	return out
}
