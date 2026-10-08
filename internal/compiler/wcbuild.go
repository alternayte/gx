package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/alternayte/gx/internal/elementname"
)

// WidgetBuild is what `gx wc build` knows about one widget (REQ-ISL-10): the
// contract of its custom element with a host page.
type WidgetBuild struct {
	// Tag is the element name.
	Tag string
	// Class is the name of the element class in the type files.
	Class string
	// Dir is the directory of the package that declares the widget.
	Dir string
	// Path is the path of the GET route of the widget, with the prefix of
	// its Group call. It has no base path of the app.
	Path string
	// Attributes are the fields of the route input, in the order of the
	// struct.
	Attributes []WidgetAttribute
	// Events are the domain events of the package of the widget, by name.
	Events []WidgetEvent
	// DTS is the type file of the element, and ReactDTS the JSX types of
	// the element for a React host.
	DTS      []byte
	ReactDTS []byte
}

// WidgetAttribute is one attribute of a widget element.
type WidgetAttribute struct {
	Name string
	// Type is the TypeScript type: string, number or boolean.
	Type    string
	Default string
}

// WidgetEvent is one domain event of a widget.
type WidgetEvent struct {
	Name string
	// Detail is the TypeScript type of the detail.
	Detail string
}

// Widgets returns the widgets of the module under root, by tag. It returns
// the diagnostics of the module when the module does not check.
func Widgets(root string) ([]WidgetBuild, []Diagnostic) {
	root = absoluteRoot(root)
	l := newLoader()
	dirs := collectDirs(root)
	res, diags := l.analyze(root, dirs)
	if len(diags) > 0 {
		sortDiags(diags)
		return nil, diags
	}
	if res == nil {
		return nil, nil
	}
	mounts := collectMounts(res.pkgs)
	var out []WidgetBuild
	for _, w := range collectWidgets(res.pkgs) {
		if w.tag == "" || elementname.Problem(w.tag) != "" || w.input == nil || w.input.Obj().Pkg() == nil {
			continue
		}
		def := res.routeDefs[w.input.Obj().Pkg().Path()+"."+w.input.Obj().Name()]
		if def == nil {
			continue
		}
		b := WidgetBuild{Tag: w.tag, Class: widgetClassName(w.tag), Dir: filepath.Dir(w.at.Filename)}
		_, path, _ := strings.Cut(def.pattern, " ")
		prefix, known := "", w.varKey == ""
		for _, m := range mounts[w.varKey] {
			if m.prefixOK {
				prefix, known = m.prefix, true
				break
			}
		}
		if !known {
			diags = append(diags, Diagnostic{
				Code: CodeWidgetOrigins, File: w.at.Filename, Line: w.at.Line, Col: w.at.Column,
				Msg: "no Group call with a constant prefix mounts the widget " + w.label() + ", so its path is not known; mount it in a Group call",
			})
			continue
		}
		b.Path = "/" + strings.Trim(strings.TrimSuffix(prefix, "/")+"/"+strings.TrimPrefix(path, "/"), "/")
		for _, f := range def.fields {
			if f.query == "" {
				continue
			}
			attr := WidgetAttribute{Name: f.query, Default: f.def, Type: "string"}
			info := types.BasicInfo(0)
			if basic, ok := f.typ.Underlying().(*types.Basic); ok {
				info = basic.Info()
			}
			switch {
			case info&types.IsBoolean != 0:
				attr.Type = "boolean"
			case info&types.IsNumeric != 0:
				attr.Type = "number"
			}
			b.Attributes = append(b.Attributes, attr)
		}
		decls, events, eventDiags := widgetEvents(w, &b)
		if len(eventDiags) > 0 {
			diags = append(diags, eventDiags...)
			continue
		}
		b.Events = events
		b.DTS = widgetDTS(&b, decls)
		b.ReactDTS = widgetReactDTS(&b)
		out = append(out, b)
	}
	if len(diags) > 0 {
		sortDiags(diags)
		return nil, diags
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out, nil
}

// widgetClassName makes the class name of a tag: "acme-cart" gives
// "AcmeCart".
func widgetClassName(tag string) string {
	var b strings.Builder
	upper := true
	for _, r := range tag {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) || r > unicode.MaxASCII {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
			upper = false
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "Widget"
	}
	return b.String()
}

// widgetEvents maps the domain events of the package of a widget: each
// package variable that holds a gx.Event. It returns the TypeScript
// declarations of the detail types. The rule is one that a reader of the
// package can see: an event of a different package is not in the contract
// of the widget.
func widgetEvents(w *widgetDecl, b *WidgetBuild) (string, []WidgetEvent, []Diagnostic) {
	pkg := w.pkg
	m := &islandMapper{
		island:  &Island{Name: b.Class},
		pkg:     pkg.Types,
		objects: map[string]*islandObjectType{},
		enums:   map[string]*islandShape{},
		tsNames: map[string]string{},
		open:    map[string]bool{},
		imports: map[string]string{},
	}
	// The names that the type file declares itself.
	for _, name := range []string{b.Class + "Attributes", b.Class + "EventMap", b.Class + "Element", "GxWidgetError", "GxWidgetToken"} {
		m.tsNames[name] = "gx:" + name
	}
	var events []WidgetEvent
	var diags []Diagnostic
	seen := map[string]bool{}
	for _, file := range pkg.Syntax {
		path := pkg.Fset.Position(file.Pos()).Filename
		if strings.HasSuffix(path, "_gx.go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, val := range vs.Values {
					call, ok := ast.Unparen(val).(*ast.CallExpr)
					if !ok || !isGxFuncExpr(pkg, call.Fun, "Event") || len(call.Args) != 1 {
						continue
					}
					index, ok := call.Fun.(*ast.IndexExpr)
					if !ok {
						continue
					}
					tv, ok := pkg.TypesInfo.Types[call.Args[0]]
					if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
						continue
					}
					name := constant.StringVal(tv.Value)
					if seen[name] {
						continue
					}
					seen[name] = true
					shape, err := m.shape(pkg.TypesInfo.TypeOf(index.Index), "")
					if err != nil {
						at := pkg.Fset.Position(call.Pos())
						diags = append(diags, Diagnostic{
							Code: CodeIslandType, File: at.Filename, Line: at.Line, Col: at.Column,
							Msg: "event " + name + ": the detail " + strings.TrimPrefix(err.path, ".") + " has type " + Quoted(err.typ) + ": " + err.why,
							Fix: "use a string, a number, a bool, time.Time, a slice, a map with string keys, a struct or a pointer to one of them",
						})
						continue
					}
					events = append(events, WidgetEvent{Name: name, Detail: tsType(shape)})
				}
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Name < events[j].Name })
	var decls strings.Builder
	for _, e := range m.enumOrder {
		parts := make([]string, len(e.values))
		for i, v := range e.values {
			data, _ := json.Marshal(v)
			parts[i] = string(data)
		}
		fmt.Fprintf(&decls, "export type %s = %s;\n\n", e.alias, strings.Join(parts, " | "))
	}
	for _, obj := range m.order {
		fmt.Fprintf(&decls, "export interface %s {\n", obj.name)
		for _, f := range obj.fields {
			fmt.Fprintf(&decls, "  %s;\n", tsField(f))
		}
		decls.WriteString("}\n\n")
	}
	return decls.String(), events, diags
}

// lifecycleEvents are the events of each widget element (REQ-ISL-16,
// REQ-ISL-20), with the TypeScript type of the detail.
var lifecycleEvents = []WidgetEvent{
	{Name: "gx-ready", Detail: "null"},
	{Name: "gx-error", Detail: "GxWidgetError"},
	{Name: "gx-navigate", Detail: "{ url: string }"},
}

// widgetDTS writes the type file of a widget element.
func widgetDTS(b *WidgetBuild, decls string) []byte {
	var s strings.Builder
	s.WriteString("// Code generated by gx wc build. DO NOT EDIT.\n\n")
	s.WriteString(decls)
	fmt.Fprintf(&s, "/** The attributes of <%s>. The element sends each one to the Gx server. */\n", b.Tag)
	fmt.Fprintf(&s, "export interface %sAttributes {\n", b.Class)
	for _, a := range b.Attributes {
		if a.Default != "" {
			data, _ := json.Marshal(a.Default)
			fmt.Fprintf(&s, "  /** Default %s. */\n", data)
		}
		name := a.Name
		if !tsIdent.MatchString(name) {
			data, _ := json.Marshal(name)
			name = string(data)
		}
		fmt.Fprintf(&s, "  %s?: %s;\n", name, a.Type)
	}
	s.WriteString("}\n\n")
	s.WriteString("/** The detail of gx-error: a status and a message key. */\nexport interface GxWidgetError {\n  status: number;\n  key: string;\n  field?: string;\n}\n\n")
	s.WriteString("/** The token of the host for the widget: text, or a function that gives the text now. */\nexport type GxWidgetToken = string | (() => string | Promise<string>) | undefined;\n\n")
	fmt.Fprintf(&s, "/** The events of <%s>. */\nexport interface %sEventMap extends HTMLElementEventMap {\n", b.Tag, b.Class)
	for _, e := range append(append([]WidgetEvent{}, b.Events...), lifecycleEvents...) {
		fmt.Fprintf(&s, "  %q: CustomEvent<%s>;\n", e.Name, e.Detail)
	}
	s.WriteString("}\n\n")
	c := b.Class
	fmt.Fprintf(&s, "export declare class %sElement extends HTMLElement {\n", c)
	s.WriteString("  /** Sent as a bearer header on each request of the widget. It is never an attribute. */\n  token: GxWidgetToken;\n")
	for _, method := range []string{"addEventListener", "removeEventListener"} {
		options := "boolean | AddEventListenerOptions"
		if method == "removeEventListener" {
			options = "boolean | EventListenerOptions"
		}
		fmt.Fprintf(&s, "  %s<K extends keyof %sEventMap>(type: K, listener: (this: %sElement, ev: %sEventMap[K]) => unknown, options?: %s): void;\n", method, c, c, c, options)
		fmt.Fprintf(&s, "  %s(type: string, listener: EventListenerOrEventListenerObject, options?: %s): void;\n", method, options)
	}
	s.WriteString("}\n\n")
	fmt.Fprintf(&s, "declare global {\n  interface HTMLElementTagNameMap {\n    %q: %sElement;\n  }\n}\n", b.Tag, c)
	return []byte(s.String())
}

// widgetReactDTS writes the JSX types of a widget element for a React host.
// It is a file of its own: it needs the types of React, and a host with no
// React must not get an error from the type file of the element.
func widgetReactDTS(b *WidgetBuild) []byte {
	c := b.Class
	var s strings.Builder
	s.WriteString("// Code generated by gx wc build. DO NOT EDIT.\n\n")
	s.WriteString("import type { DetailedHTMLProps, HTMLAttributes } from \"react\";\n")
	fmt.Fprintf(&s, "import type { %sAttributes, %sElement, %sEventMap } from \"./%s\";\n\n", c, c, c, b.Tag)
	var names []string
	for _, e := range append(append([]WidgetEvent{}, b.Events...), lifecycleEvents...) {
		names = append(names, fmt.Sprintf("%q", e.Name))
	}
	fmt.Fprintf(&s, "/** The event props of <%s>: on and the name of the event, as React 19 reads them. */\n", b.Tag)
	fmt.Fprintf(&s, "type %sEvents = {\n  [K in %s as `on${K}`]?: (event: %sEventMap[K]) => void;\n};\n\n", c, strings.Join(names, " | "), c)
	s.WriteString("declare module \"react\" {\n  namespace JSX {\n    interface IntrinsicElements {\n")
	fmt.Fprintf(&s, "      %q: DetailedHTMLProps<HTMLAttributes<%sElement>, %sElement> & %sAttributes & %sEvents;\n", b.Tag, c, c, c, c)
	s.WriteString("    }\n  }\n}\n")
	return []byte(s.String())
}

// WidgetManifest writes the custom elements manifest of the widgets
// (custom-elements.json, schema 1.0.0). `gx wc check` compares it with the
// manifest of the last release (REQ-ISL-13).
func WidgetManifest(widgets []WidgetBuild) ([]byte, error) {
	type typ struct {
		Text string `json:"text"`
	}
	type attribute struct {
		Name    string `json:"name"`
		Type    typ    `json:"type"`
		Default string `json:"default,omitempty"`
	}
	type event struct {
		Name string `json:"name"`
		Type typ    `json:"type"`
	}
	type member struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
		Type typ    `json:"type"`
	}
	type declaration struct {
		Kind          string      `json:"kind"`
		Name          string      `json:"name"`
		TagName       string      `json:"tagName"`
		CustomElement bool        `json:"customElement"`
		Attributes    []attribute `json:"attributes"`
		Events        []event     `json:"events"`
		Members       []member    `json:"members"`
	}
	type ref struct {
		Name   string `json:"name"`
		Module string `json:"module"`
	}
	type export struct {
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		Declaration ref    `json:"declaration"`
	}
	type module struct {
		Kind         string        `json:"kind"`
		Path         string        `json:"path"`
		Declarations []declaration `json:"declarations"`
		Exports      []export      `json:"exports"`
	}
	manifest := struct {
		SchemaVersion string   `json:"schemaVersion"`
		Modules       []module `json:"modules"`
	}{SchemaVersion: "1.0.0", Modules: []module{}}
	for _, w := range widgets {
		d := declaration{Kind: "class", Name: w.Class + "Element", TagName: w.Tag, CustomElement: true,
			Attributes: []attribute{}, Events: []event{},
			Members: []member{{Kind: "field", Name: "token", Type: typ{Text: "GxWidgetToken"}}}}
		for _, a := range w.Attributes {
			d.Attributes = append(d.Attributes, attribute{Name: a.Name, Type: typ{Text: a.Type}, Default: a.Default})
		}
		for _, e := range append(append([]WidgetEvent{}, w.Events...), lifecycleEvents...) {
			d.Events = append(d.Events, event{Name: e.Name, Type: typ{Text: "CustomEvent<" + e.Detail + ">"}})
		}
		file := w.Tag + ".js"
		manifest.Modules = append(manifest.Modules, module{
			Kind: "javascript-module", Path: file,
			Declarations: []declaration{d},
			Exports:      []export{{Kind: "custom-element-definition", Name: w.Tag, Declaration: ref{Name: d.Name, Module: file}}},
		})
	}
	// The types of the manifest are for a reader and for tools: the text
	// keeps its angle brackets.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
