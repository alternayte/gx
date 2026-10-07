package compiler

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ElementsFile is the file that describes the imported web components of a
// package (REQ-ISL-09). `gx wc pin` writes it from the custom elements
// manifest of an npm package.
const ElementsFile = "gx-elements.json"

// The kinds of an element attribute.
const (
	ElementString = "string"
	ElementBool   = "bool"
	ElementNumber = "number"
	// ElementEnum is a string with a fixed set of values.
	ElementEnum = "enum"
)

// ElementSet is the content of an ElementsFile.
type ElementSet struct {
	// Package and Version name the npm package.
	Package  string       `json:"package"`
	Version  string       `json:"version"`
	Elements []ElementDef `json:"elements"`
}

// ElementDef is one imported custom element. A tag <pkg.Name> renders the
// element <Tag>.
type ElementDef struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
	Doc  string `json:"doc,omitempty"`
	// Module is the import specifier of the module that defines the
	// element. gx pin holds its file.
	Module     string        `json:"module"`
	Attributes []ElementAttr `json:"attributes"`
	Events     []string      `json:"events"`
	Slots      []string      `json:"slots"`
}

// ElementAttr is one attribute of an imported element.
type ElementAttr struct {
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Values []string `json:"values,omitempty"`
	Doc    string   `json:"doc,omitempty"`
}

func (d *ElementDef) attr(name string) (ElementAttr, bool) {
	for _, a := range d.Attributes {
		if a.Name == name {
			return a, true
		}
	}
	return ElementAttr{}, false
}

// loadElements reads the ElementsFile of a package directory.
func (l *loader) loadElements(p *Package) {
	path := filepath.Join(p.Dir, ElementsFile)
	data, ok := l.read(path)
	if !ok {
		return
	}
	var set ElementSet
	if err := json.Unmarshal(data, &set); err != nil {
		p.Diags = append(p.Diags, Diagnostic{Code: CodeParse, File: path, Line: 1, Col: 1,
			Msg: ElementsFile + " is not valid: " + err.Error(), Fix: "run gx wc pin again"})
		return
	}
	p.Elements = map[string]*ElementDef{}
	for i := range set.Elements {
		p.Elements[set.Elements[i].Name] = &set.Elements[i]
	}
}

// globalAttrs are the HTML attributes that every element takes.
var globalAttrs = map[string]bool{
	"id": true, "class": true, "style": true, "slot": true, "title": true, "hidden": true,
	"tabindex": true, "lang": true, "dir": true, "role": true, "part": true, "inert": true,
	"draggable": true, "autofocus": true, "popover": true, "is": true, "exportparts": true,
}

func isGlobalAttr(name string) bool {
	return globalAttrs[name] || strings.HasPrefix(name, "aria-") || strings.HasPrefix(name, "data-")
}

// lowerElements turns each tag of an imported web component in a file into
// the custom element it stands for (REQ-ISL-09). After this pass the tag is
// an HTML element for every later pass: its expressions, directives, events,
// classes and spreads work as they do on an HTML element. The pass checks
// what the manifest of the element knows: attributes, values, events and
// slots.
func (l *loader) lowerElements(p *Package, f *File) []Diagnostic {
	var diags []Diagnostic
	// The pass goes from the inside out, so a slot that holds one imported
	// element sees it as the custom element it is.
	var lower func(ns []Node)
	lower = func(ns []Node) {
		for _, n := range ns {
			switch t := n.(type) {
			case *Element:
				if !t.HasRaw {
					lower(t.Children)
				}
				diags = append(diags, l.lowerTag(p, f, t)...)
			case *Control:
				lower(t.Body)
				lower(t.Else)
				for _, c := range t.Cases {
					lower(c.Body)
				}
			}
		}
	}
	lower(f.Body)
	return diags
}

// lowerTag lowers one tag when it names an imported element.
func (l *loader) lowerTag(p *Package, f *File, el *Element) []Diagnostic {
	qual, name, ok := componentTag(el.Name)
	if !ok {
		return nil
	}
	var def *ElementDef
	if qual == "" {
		if _, isComponent := p.Files[name]; !isComponent {
			def = p.Elements[name]
		}
	} else if target, ok := l.importedPackage(p, f, qual); ok {
		def = target.Elements[name]
	}
	if def == nil {
		return nil
	}
	return lowerElement(f, el, def)
}

// lowerElement checks one tag against its element and rewrites it.
func lowerElement(f *File, el *Element, def *ElementDef) []Diagnostic {
	var diags []Diagnostic
	tag := el.Name
	report := func(code string, at Pos, msg string) {
		diags = append(diags, Diagnostic{Code: code, File: f.File, Line: at.Line, Col: at.Col, Msg: msg})
	}
	unknownAttr := func(at Pos, name string) {
		msg := "unknown attribute " + Quoted(name) + " on <" + tag + ">"
		best, bestDist := "", 3
		for _, a := range def.Attributes {
			if d := levenshtein(name, a.Name); d < bestDist {
				best, bestDist = a.Name, d
			}
		}
		if best != "" {
			msg += "; did you mean " + Quoted(best) + "?"
		}
		report(CodeUnknownAttr, at, msg)
	}
	given := map[string]bool{}
	bools := map[string]bool{}
	for _, a := range def.Attributes {
		if a.Kind == ElementBool {
			bools[a.Name] = true
		}
	}
	for i := range el.Attrs {
		a := &el.Attrs[i]
		if a.Kind == AttrSpread || a.Kind == AttrFragment {
			continue
		}
		if rest, ok := strings.CutPrefix(a.Name, "on:"); ok {
			// A custom event has a hyphen in its name; the manifest lists
			// the ones the element sends. A DOM event such as click has
			// none.
			event, _, _ := strings.Cut(rest, ".")
			if strings.Contains(event, "-") && !containsString(def.Events, event) {
				msg := "unknown event " + Quoted(event) + " on <" + tag + ">"
				if len(def.Events) > 0 {
					msg += "; the element sends " + strings.Join(def.Events, ", ")
				}
				report(CodeUnknownAttr, a.At, msg)
			}
			continue
		}
		name := a.Name
		for _, prefix := range []string{"attr:", "bind:"} {
			if rest, ok := strings.CutPrefix(a.Name, prefix); ok {
				name = rest
			}
		}
		if name == a.Name && isDirective(a.Name) {
			continue
		}
		attr, known := def.attr(name)
		if !known {
			// A bind: target can be a property of the element, which the
			// manifest of attributes does not list.
			if !isGlobalAttr(name) && !strings.HasPrefix(a.Name, "bind:") {
				unknownAttr(a.At, name)
			}
			continue
		}
		given[name] = true
		if name != a.Name {
			continue // a client expression sets the value in the browser
		}
		switch a.Kind {
		case AttrString:
			switch attr.Kind {
			case ElementEnum:
				if !containsString(attr.Values, a.Value) {
					report(CodeStaticStringProp, a.At, "attribute "+Quoted(name)+" of <"+tag+"> has the value "+Quoted(a.Value)+
						"; the values are "+strings.Join(attr.Values, ", "))
				}
			case ElementBool:
				report(CodeStaticStringProp, a.At, "attribute "+Quoted(name)+" of <"+tag+"> is a boolean attribute; write it with no value or with a bool expression")
			case ElementNumber:
				if _, err := strconv.ParseFloat(strings.TrimSpace(a.Value), 64); err != nil {
					report(CodeStaticStringProp, a.At, "attribute "+Quoted(name)+" of <"+tag+"> is a number and has the value "+Quoted(a.Value))
				}
			}
		case AttrBool:
			if attr.Kind != ElementBool {
				report(CodeStaticStringProp, a.At, "attribute "+Quoted(name)+" of <"+tag+"> needs a value; only a boolean attribute has none")
			}
		}
	}

	// A named slot of the tag is content with a slot attribute. One element
	// takes the attribute itself; other content goes into a span.
	slots := map[string]bool{}
	children := make([]Node, 0, len(el.Children))
	for _, child := range el.Children {
		slotEl, ok := child.(*Element)
		if !ok || !strings.HasPrefix(slotEl.Name, ":") {
			children = append(children, child)
			continue
		}
		slot := strings.TrimPrefix(slotEl.Name, ":")
		switch {
		case !containsString(def.Slots, slot):
			msg := "unknown slot " + Quoted(slot) + " on <" + tag + ">"
			if len(def.Slots) > 0 {
				msg += "; the element has the slots " + strings.Join(def.Slots, ", ")
			}
			report(CodeUnknownAttr, slotEl.At, msg)
			continue
		case slots[slot]:
			report(CodeDuplicateSlot, slotEl.At, "slot "+Quoted(slot)+" is given twice on <"+tag+">")
			continue
		}
		slots[slot] = true
		slotAttr := Attr{At: slotEl.At, Kind: AttrString, Name: "slot", Value: slot, ValueAt: slotEl.At}
		if only := onlyElement(slotEl.Children); only != nil {
			only.Attrs = append(only.Attrs, slotAttr)
			children = append(children, only)
			continue
		}
		children = append(children, &Element{
			At:   slotEl.At,
			Name: "span",
			Attrs: []Attr{slotAttr,
				{At: slotEl.At, Kind: AttrString, Name: "style", Value: "display:contents", ValueAt: slotEl.At}},
			Children: slotEl.Children,
		})
	}

	el.Name = def.Tag
	el.Children = children
	el.element = def
	el.elementTag = tag
	el.boolAttrs = bools
	// The page loads the module of the element by this attribute.
	el.Attrs = append(el.Attrs, Attr{At: el.At, Kind: AttrModule, Name: elementModuleAttr,
		Value: def.Module, ValueAt: el.At})
	// An element can set its own attributes, for example open or checked.
	// A morph leaves alone each attribute that this tag does not set, so
	// the element keeps that state (REQ-ISL-09).
	var keep []string
	for _, a := range def.Attributes {
		if !given[a.Name] {
			keep = append(keep, a.Name)
		}
	}
	if len(keep) > 0 {
		sort.Strings(keep)
		el.Attrs = append(el.Attrs, Attr{At: el.At, Kind: AttrString, Name: "data-preserve-attr",
			Value: strings.Join(keep, " "), ValueAt: el.At})
	}
	return diags
}

// elementModuleAttr names the module of an imported element for the
// browser.
const elementModuleAttr = "data-gx-module"

// onlyElement returns the one HTML element of a slot, when the slot holds
// that element and white space only.
func onlyElement(ns []Node) *Element {
	var only *Element
	for _, n := range ns {
		switch t := n.(type) {
		case *Text:
			if strings.TrimSpace(t.Data) != "" {
				return nil
			}
		case *Comment:
		case *Element:
			if only != nil || strings.HasPrefix(t.Name, ":") {
				return nil
			}
			if _, _, isComponent := componentTag(t.Name); isComponent {
				return nil
			}
			for i := range t.Attrs {
				if t.Attrs[i].Name == "slot" || t.Attrs[i].Kind == AttrSpread {
					return nil
				}
			}
			only = t
		default:
			return nil
		}
	}
	return only
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ElementModules returns the modules of the imported elements that the
// .gx files under root use, in order (REQ-ISL-09). The island bundler makes
// one entry file for each.
func ElementModules(root string) []string {
	root = absoluteRoot(root)
	l := newLoader()
	seen := map[string]bool{}
	for _, dir := range collectDirs(root) {
		p := l.load(dir)
		for _, f := range p.Files {
			walkElements(f.Body, func(el *Element) {
				if el.element != nil {
					seen[el.element.Module] = true
				}
			})
		}
	}
	out := make([]string, 0, len(seen))
	for module := range seen {
		out = append(out, module)
	}
	sort.Strings(out)
	return out
}
