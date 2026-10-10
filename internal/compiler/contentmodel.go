package compiler

import (
	"strings"
)

// The content model checks of gx check (REQ-AUT-23). Each one finds markup
// that is a defect in each render: a browser repairs it or a user cannot use
// it. They read one .gx file, so they report only what one file shows: an
// element whose parent is in a different component has no finding.
//
// They run in gx check and not in gx generate: a page with such a defect
// still compiles and runs.

// interactiveNames are the elements that take a click or the focus of the
// keyboard. HTML does not allow one inside a link or a button.
var interactiveNames = map[string]bool{
	"a": true, "button": true, "input": true, "select": true, "textarea": true,
	"details": true, "label": true, "iframe": true, "embed": true,
}

// phrasingOnly are the elements whose open tag closes a paragraph: the
// parser of a browser ends the p before them.
var closesParagraph = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "details": true, "div": true, "dl": true,
	"fieldset": true, "figcaption": true, "figure": true, "footer": true, "form": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "header": true, "hr": true, "main": true, "menu": true,
	"nav": true, "ol": true, "p": true, "pre": true, "section": true, "table": true, "ul": true,
}

// parentsOf holds, for an element that the parser of a browser moves or
// drops outside its parents, the parents that it can have.
var parentsOf = map[string][]string{
	"li":       {"ul", "ol", "menu"},
	"tr":       {"table", "thead", "tbody", "tfoot"},
	"td":       {"tr"},
	"th":       {"tr"},
	"thead":    {"table"},
	"tbody":    {"table"},
	"tfoot":    {"table"},
	"caption":  {"table"},
	"colgroup": {"table"},
	"col":      {"colgroup", "table"},
	"dt":       {"dl", "div"},
	"dd":       {"dl", "div"},
	"option":   {"select", "datalist", "optgroup"},
	"optgroup": {"select"},
	"summary":  {"details"},
	"legend":   {"fieldset"},
}

// childrenOf holds, for a part of a table, the elements that it can hold. A
// browser moves each other element to the place before the table.
var childrenOf = map[string][]string{
	"table": {"caption", "colgroup", "thead", "tbody", "tfoot", "tr"},
	"thead": {"tr"},
	"tbody": {"tr"},
	"tfoot": {"tr"},
	"tr":    {"td", "th"},
}

// anywhere are the elements that each parent can hold: the parser of a
// browser leaves them where they are.
var anywhere = map[string]bool{"script": true, "template": true, "style": true}

// checkContentModel returns the content model findings of one file.
func checkContentModel(f *File) []Diagnostic {
	c := &contentCheck{file: f, labelFor: map[string]bool{}, ids: map[string][]idUse{}}
	// The labels of the file, by the id that each one names.
	walkElements(f.Body, func(el *Element) {
		if el.Name == "label" {
			if a := findAttr(el, "for"); a != nil {
				c.labelFor[attrKey(a)] = true
			}
		}
	})
	c.walk(f.Body, nil, nil)
	c.duplicateIDs()
	return c.out
}

// headingHints returns the warning of a heading that skips a level inside
// one file (REQ-AUT-23). It does not fail a check: a component does not know
// the level of the heading of its caller.
func headingHints(f *File) []Diagnostic {
	var out []Diagnostic
	last := 0
	walkElements(f.Body, func(el *Element) {
		if len(el.Name) != 2 || el.Name[0] != 'h' || el.Name[1] < '1' || el.Name[1] > '6' {
			return
		}
		level := int(el.Name[1] - '0')
		if last > 0 && level > last+1 {
			out = append(out, Diagnostic{
				Code: CodeHeadingOrder, File: f.File, Line: el.At.Line, Col: el.At.Col, Hint: true,
				Msg: "the heading <" + el.Name + "> comes after <h" + string(rune('0'+last)) + ">: it skips a level",
				Fix: "use <h" + string(rune('0'+last+1)) + ">, or change the level of the heading before it",
			})
		}
		last = level
	})
	return out
}

type idUse struct {
	el *Element
	// branch holds, for each control around the element, the control and
	// the branch of it that holds the element.
	branch []branchStep
}

type branchStep struct {
	control *Control
	arm     int
	loop    bool
}

type contentCheck struct {
	file     *File
	labelFor map[string]bool
	ids      map[string][]idUse
	out      []Diagnostic
	// slot counts the component tags around the node: the content of a
	// component goes where the component puts it, for example inside its
	// label.
	slot int
}

func (c *contentCheck) report(code string, el *Element, msg, fix string) {
	c.out = append(c.out, Diagnostic{Code: code, File: c.file.File, Line: el.At.Line, Col: el.At.Col, Msg: msg, Fix: fix})
}

// findAttr returns the attribute of an element with the name, or nil.
func findAttr(el *Element, name string) *Attr {
	for i := range el.Attrs {
		if el.Attrs[i].Name == name && el.Attrs[i].Kind != AttrSpread {
			return &el.Attrs[i]
		}
	}
	return nil
}

// hasSpread reports whether the element has a spread: its attributes are
// then not all in the file.
func hasSpread(el *Element) bool {
	for i := range el.Attrs {
		if el.Attrs[i].Kind == AttrSpread {
			return true
		}
	}
	return false
}

// attrKey is the text of an attribute value for a comparison: a static
// value and an expression with the same text are different keys.
func attrKey(a *Attr) string {
	if a.Kind == AttrExpr {
		return "{" + strings.TrimSpace(a.Value) + "}"
	}
	return a.Value
}

// walk visits the nodes below the element stack parents. A component tag
// ends the stack: the component decides where its children go.
func (c *contentCheck) walk(ns []Node, parents []*Element, branch []branchStep) {
	for _, n := range ns {
		switch t := n.(type) {
		case *Control:
			loop := t.Kind == "for"
			c.walk(t.Body, parents, append(branch[:len(branch):len(branch)], branchStep{control: t, arm: 0, loop: loop}))
			c.walk(t.Else, parents, append(branch[:len(branch):len(branch)], branchStep{control: t, arm: 1}))
			for i, cs := range t.Cases {
				c.walk(cs.Body, parents, append(branch[:len(branch):len(branch)], branchStep{control: t, arm: i + 2}))
			}
		case *Element:
			if _, _, comp := componentTag(t.Name); comp || strings.HasPrefix(t.Name, ":") {
				// The parent of the content of a component is in the
				// file of the component.
				c.slot++
				c.walk(t.Children, nil, branch)
				c.slot--
				continue
			}
			c.element(t, parents, branch)
			c.walk(t.Children, append(parents[:len(parents):len(parents)], t), branch)
		}
	}
}

func (c *contentCheck) element(el *Element, parents []*Element, branch []branchStep) {
	if a := findAttr(el, "id"); a != nil && a.Kind == AttrString && a.Value != "" {
		c.ids[a.Value] = append(c.ids[a.Value], idUse{el: el, branch: branch})
	}
	switch el.Name {
	case "img":
		if findAttr(el, "alt") == nil && !hasSpread(el) && findAttr(el, "role") == nil && findAttr(el, "aria-hidden") == nil {
			c.report(CodeImgAlt, el, "the <img> has no alt attribute", `write alt="..." with the text of the image, or alt="" for an image that is only decoration`)
		}
	case "input", "select", "textarea":
		c.control(el, parents)
	}
	if len(parents) == 0 {
		return
	}
	parent := parents[len(parents)-1]
	// An interactive element inside a link or a button.
	if interactiveNames[el.Name] && !(el.Name == "input" && attrIs(el, "type", "hidden")) {
		for i := len(parents) - 1; i >= 0; i-- {
			if p := parents[i].Name; p == "a" || p == "button" {
				if el.Name == "label" && p != "button" {
					break
				}
				c.report(CodeNestedInteractive, el, "the <"+el.Name+"> is inside a <"+p+">: an interactive element cannot hold a second one",
					"move the <"+el.Name+"> out of the <"+p+">")
				break
			}
		}
	}
	if el.Name == "form" {
		for _, p := range parents {
			if p.Name == "form" {
				c.report(CodeBadParent, el, "the <form> is inside a <form>: a browser drops the inner form",
					"put the second <form> after the first one, or give a control of the first one the form attribute")
				break
			}
		}
	}
	if allowed, ok := childrenOf[parent.Name]; ok && !anywhere[el.Name] && !strings.Contains(el.Name, "-") {
		good := false
		for _, name := range allowed {
			good = good || el.Name == name
		}
		if _, hasRule := parentsOf[el.Name]; !good && !hasRule {
			c.report(CodeBadParent, el, "the <"+el.Name+"> is inside a <"+parent.Name+">: a browser moves it to the place before the table",
				"put the <"+el.Name+"> inside a <td> or a <th>")
		}
	}
	// An element that the parser of a browser moves or drops.
	if allowed, ok := parentsOf[el.Name]; ok {
		good := false
		for _, name := range allowed {
			if parent.Name == name {
				good = true
			}
		}
		// A template and a custom element hold what their script puts
		// there.
		if !good && parent.Name != "template" && !strings.Contains(parent.Name, "-") {
			c.report(CodeBadParent, el, "the <"+el.Name+"> is inside a <"+parent.Name+">: a browser moves or drops it there",
				"put the <"+el.Name+"> inside a <"+strings.Join(allowed, "> or a <")+">")
		}
	}
	if closesParagraph[el.Name] {
		for i := len(parents) - 1; i >= 0; i-- {
			p := parents[i].Name
			if p == "p" {
				c.report(CodeBadParent, el, "the <"+el.Name+"> is inside a <p>: a browser ends the paragraph before it",
					"use a <div> in the place of the <p>, or a <span> in the place of the <"+el.Name+">")
				break
			}
			// These elements start a new place for block content.
			if p == "button" || p == "td" || p == "th" || p == "li" || p == "div" || p == "object" || p == "blockquote" {
				break
			}
		}
	}
}

// attrIs reports whether a static attribute has the value.
func attrIs(el *Element, name, value string) bool {
	a := findAttr(el, name)
	return a != nil && a.Kind == AttrString && strings.EqualFold(a.Value, value)
}

// control checks that a form control has a label that the file shows.
func (c *contentCheck) control(el *Element, parents []*Element) {
	if hasSpread(el) || c.slot > 0 {
		// In the content of a component, the component can write the
		// label around its children.
		return
	}
	if el.Name == "input" {
		for _, kind := range []string{"hidden", "submit", "button", "reset", "image"} {
			if attrIs(el, "type", kind) {
				return
			}
		}
		if t := findAttr(el, "type"); t != nil && t.Kind == AttrExpr {
			return
		}
	}
	for _, name := range []string{"aria-label", "aria-labelledby", "title", "attr:aria-label", "attr:aria-labelledby"} {
		if findAttr(el, name) != nil {
			return
		}
	}
	for _, p := range parents {
		if p.Name == "label" {
			return
		}
	}
	if id := findAttr(el, "id"); id != nil && c.labelFor[attrKey(id)] {
		return
	}
	c.report(CodeControlLabel, el, "the <"+el.Name+"> has no label",
		"put it inside a <label>, give it an id that a <label for> names, or write aria-label")
}

// duplicateIDs reports two elements with one static id that one render can
// hold: they are not in two branches of one if or switch.
func (c *contentCheck) duplicateIDs() {
	for id, uses := range c.ids {
		// One element with a static id in the body of a loop is the id on
		// each element that the loop writes.
		for _, use := range uses {
			for _, step := range use.branch {
				if step.loop {
					c.report(CodeDuplicateID, use.el, "the id "+Quoted(id)+" is on an element in a loop: each turn of the loop writes an element with that id",
						"make the id from a value of the loop, for example id={\"row-\" + it.ID}")
					break
				}
			}
		}
		for i := 1; i < len(uses); i++ {
			for j := 0; j < i; j++ {
				if exclusive(uses[i].branch, uses[j].branch) {
					continue
				}
				c.report(CodeDuplicateID, uses[i].el, "the id "+Quoted(id)+" is on two elements of the component: an id names one element",
					"give each element its own id")
				break
			}
		}
	}
}

// exclusive reports whether two elements are in two different branches of
// one control, so one render holds one of them.
func exclusive(a, b []branchStep) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].control != b[i].control {
			return false
		}
		if a[i].arm != b[i].arm {
			return true
		}
	}
	return false
}
