package compiler

import (
	"regexp"
	"sort"
	"strings"
)

// PluginDirective is one directive of a plugin of the project
// (REQ-PLG-03). The gx command gives the compiler the directives of its
// plugins before a compile. A directive has a namespace: its name is
// "namespace:name".
type PluginDirective struct {
	Name   string
	Plugin string
	// Transform returns the HTML attributes that take the place of one use
	// of the directive. It runs at compile time.
	Transform func(DirectiveUse) ([]DirectiveAttr, error)
	// Behavior is the ES module that a page with the directive loads, or
	// nil.
	Behavior []byte
}

// DirectiveUse is one use of a plugin directive in a .gx file.
type DirectiveUse struct {
	// Tag is the name of the element.
	Tag string
	// Value is the static value of the attribute. HasValue is false for
	// the directive with no value.
	Value    string
	HasValue bool
}

// DirectiveAttr is one HTML attribute that a directive writes.
type DirectiveAttr struct {
	Name  string
	Value string
}

// pluginDirectives holds the directives of the plugins of this process, by
// name. The gx command sets them one time before a command runs.
var pluginDirectives = map[string]PluginDirective{}

// SetDirectives gives the compiler the directives of the plugins of the
// project. It replaces the directives of an earlier call.
func SetDirectives(directives []PluginDirective) {
	next := make(map[string]PluginDirective, len(directives))
	for _, d := range directives {
		next[d.Name] = d
	}
	pluginDirectives = next
}

// directiveModulePrefix starts the module name of the behaviour of a
// directive. No npm package has this name: a scope starts with @.
const directiveModulePrefix = "gx-directive/"

// directiveModule returns the module name of the behaviour of a directive:
// "chartzoom:zoom" gives "gx-directive/chartzoom/zoom".
func directiveModule(name string) string {
	return directiveModulePrefix + strings.ReplaceAll(name, ":", "/")
}

// DirectiveBehavior returns the ES module of a directive behaviour by its
// module name. The island bundle holds it as one entry.
func DirectiveBehavior(module string) ([]byte, bool) {
	name, ok := strings.CutPrefix(module, directiveModulePrefix)
	if !ok {
		return nil, false
	}
	d, ok := pluginDirectives[strings.Replace(name, "/", ":", 1)]
	if !ok || d.Behavior == nil {
		return nil, false
	}
	return d.Behavior, true
}

// directiveAttrName is the name of an HTML attribute that a directive can
// write.
var directiveAttrName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// directiveAttrProblem says why a directive cannot write an attribute, or
// "". A directive writes plain attributes: no directive, no event handler
// and no address (SI-02).
func directiveAttrProblem(name string) string {
	switch {
	case !directiveAttrName.MatchString(name):
		return "is not a plain attribute name in lower case"
	case strings.HasPrefix(name, "on"):
		return "is an event attribute"
	case isURLAttr(name):
		return "is an address; an address in a .gx file is a typed route or a static string"
	case name == "style" || name == "class" || name == "id" || name == elementModuleAttr:
		return "belongs to the element; use a data- attribute"
	}
	return ""
}

// lowerDirectives puts the attributes of each plugin directive of one
// element in the place of the directive (REQ-PLG-03). A use that the
// directive cannot take is GX2003.
func lowerDirectives(f *File, el *Element) []Diagnostic {
	if len(pluginDirectives) == 0 {
		return nil
	}
	var diags []Diagnostic
	report := func(a Attr, msg string) {
		diags = append(diags, Diagnostic{Code: CodeUnknownAttr, File: f.File, Line: a.At.Line, Col: a.At.Col, Msg: msg})
	}
	namespaces := map[string][]string{}
	for name := range pluginDirectives {
		ns, _, _ := strings.Cut(name, ":")
		namespaces[ns] = append(namespaces[ns], name)
	}
	out := make([]Attr, 0, len(el.Attrs))
	hasModule := false
	for _, a := range el.Attrs {
		if a.Kind == AttrModule {
			hasModule = true
		}
	}
	for _, a := range el.Attrs {
		ns, _, namespaced := strings.Cut(a.Name, ":")
		d, known := pluginDirectives[a.Name]
		if !known {
			if names := namespaces[ns]; namespaced && len(names) > 0 {
				sort.Strings(names)
				report(a, "unknown directive "+Quoted(a.Name)+"; the plugin "+Quoted(pluginDirectives[names[0]].Plugin)+" has "+strings.Join(names, ", "))
				continue
			}
			out = append(out, a)
			continue
		}
		if a.Kind != AttrString && a.Kind != AttrBool {
			report(a, "the directive "+Quoted(a.Name)+" of the plugin "+Quoted(d.Plugin)+" takes a static value; a plugin directive runs at compile time")
			continue
		}
		attrs, err := d.Transform(DirectiveUse{Tag: el.Name, Value: a.Value, HasValue: a.Kind == AttrString})
		if err != nil {
			report(a, "the directive "+Quoted(a.Name)+": "+err.Error())
			continue
		}
		bad := false
		for _, attr := range attrs {
			if problem := directiveAttrProblem(attr.Name); problem != "" {
				report(a, "the plugin "+Quoted(d.Plugin)+" writes the attribute "+Quoted(attr.Name)+" for the directive "+Quoted(a.Name)+", which "+problem)
				bad = true
			}
		}
		if bad {
			continue
		}
		for _, attr := range attrs {
			out = append(out, Attr{At: a.At, Kind: AttrString, Name: attr.Name, Value: attr.Value, ValueAt: a.At})
		}
		if d.Behavior != nil {
			if hasModule {
				report(a, "the directive "+Quoted(a.Name)+" has a behaviour module, and <"+el.Name+"> has a module already; put the directive on an element around it")
				continue
			}
			hasModule = true
			// The page loads the behaviour of the directive by this
			// attribute, as it loads the module of an imported element.
			out = append(out, Attr{At: a.At, Kind: AttrModule, Name: elementModuleAttr, Value: directiveModule(a.Name), ValueAt: a.At})
		}
	}
	el.Attrs = out
	return diags
}
