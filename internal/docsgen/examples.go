package main

import (
	"go/ast"
	"strconv"
	"strings"
	"unicode"
)

// part is one fixture of an example, with its code.
type part struct {
	Comp    *component
	Fixture fixture
	Snippet snippet
}

// example is one live preview of a component page. It holds one fixture,
// or the fixtures of different components that share an Id prop: a trigger
// and its menu work only on one page.
type example struct {
	// Slug names the example in its preview URL.
	Slug  string
	Title string
	Parts []part
}

// toast reports whether the example is one toast that an action pushes. Its
// preview shows the toast on a button press.
func (ex *example) toast() bool {
	return len(ex.Parts) == 1 && ex.Parts[0].Snippet.Action != ""
}

// examplesOf returns the examples of one item: the fixtures of the main
// component first, each component in source order.
func examplesOf(reg *registry, it *item) []*example {
	type entry struct {
		part part
		id   string
	}
	var entries []entry
	for _, comp := range it.Components {
		conv := newConverter(reg, it, comp)
		for _, fx := range comp.Fixtures {
			entries = append(entries, entry{part: part{Comp: comp, Fixture: fx, Snippet: conv.convert(fx)}, id: fixtureID(fx)})
		}
	}
	// Group the fixtures that share an Id across components.
	owner := map[string]*example{}
	comps := map[string]map[*component]bool{}
	for _, e := range entries {
		if e.id == "" {
			continue
		}
		if comps[e.id] == nil {
			comps[e.id] = map[*component]bool{}
		}
		comps[e.id][e.part.Comp] = true
	}
	multi := len(it.Components) > 1
	reserved := reservedHeadings(it)
	slugs := map[string]bool{}
	titles := map[string]bool{}
	var out []*example
	for _, e := range entries {
		shared := e.id != "" && len(comps[e.id]) > 1
		if shared {
			if ex := owner[e.id]; ex != nil {
				ex.Parts = append(ex.Parts, e.part)
				continue
			}
		}
		ex := &example{Parts: []part{e.part}}
		ex.Title = words(e.part.Fixture.Name)
		if multi || reserved[strings.ToLower(ex.Title)] {
			ex.Title = e.part.Comp.Name + ": " + ex.Title
		}
		for n := 2; titles[strings.ToLower(ex.Title)]; n++ {
			ex.Title = e.part.Comp.Name + ": " + words(e.part.Fixture.Name) + " " + strconv.Itoa(n)
		}
		titles[strings.ToLower(ex.Title)] = true
		base := kebab(e.part.Comp.Name) + "-" + kebab(e.part.Fixture.Name)
		ex.Slug = base
		for n := 2; slugs[ex.Slug]; n++ {
			ex.Slug = base + "-" + strconv.Itoa(n)
		}
		slugs[ex.Slug] = true
		if shared {
			owner[e.id] = ex
		}
		out = append(out, ex)
	}
	return out
}

// reservedHeadings returns the section headings of a component page, in
// lower case. An example heading must differ from each, so every anchor of
// the page is unique.
func reservedHeadings(it *item) map[string]bool {
	out := map[string]bool{"installation": true, "usage": true, "examples": true, "api reference": true, "do and don't": true, "keyboard": true}
	for _, comp := range it.Components {
		out[strings.ToLower(referenceTitle(it, comp))] = true
	}
	for _, s := range it.Usage.Other {
		out[strings.ToLower(s.Title)] = true
	}
	return out
}

// fixtureID returns the string value of the Id prop of a fixture, or "".
func fixtureID(fx fixture) string {
	lit, ok := fx.Value.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok || !isIdent(kv.Key, "Id") {
			continue
		}
		if s := plainString(kv.Value); s != nil {
			return *s
		}
	}
	return ""
}

// camelWords splits a Go identifier at its word starts: "WithToast" gives
// "With" and "Toast".
func camelWords(name string) []string {
	var out []string
	var cur []rune
	runes := []rune(name)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = nil
			}
			continue
		}
		start := i > 0 && unicode.IsUpper(r) &&
			(unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])))
		if start && len(cur) > 0 {
			out = append(out, string(cur))
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// words turns a fixture name into heading text: "TitleAndBody" gives
// "Title and body".
func words(name string) string {
	parts := camelWords(name)
	for i, p := range parts {
		if i > 0 && !allUpper(p) {
			parts[i] = strings.ToLower(p)
		}
	}
	if len(parts) == 0 {
		return name
	}
	return strings.Join(parts, " ")
}

// kebab turns a Go identifier into a URL segment: "DropdownMenu" gives
// "dropdown-menu".
func kebab(name string) string {
	parts := camelWords(name)
	if len(parts) == 0 {
		return "x"
	}
	return strings.ToLower(strings.Join(parts, "-"))
}

// allUpper reports whether a word is an initialism.
func allUpper(s string) bool {
	return len(s) > 1 && strings.ToUpper(s) == s
}
