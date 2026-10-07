package compiler

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Check checks every .gx package under root and returns the diagnostics,
// ordered by file, line and column. It checks props and components, then
// type-checks every server expression.
func Check(root string) []Diagnostic {
	return CheckWith(root, CheckOptions{})
}

// CheckOptions configure one check (REQ-CNT-10).
type CheckOptions struct {
	// ExternalLinks requests every external content link.
	ExternalLinks bool
	// Client is the HTTP client of the external check.
	Client *http.Client
	// Adapter checks the files for this adapter and not for the adapter
	// key of gx.toml (REQ-ACT-09). The registry has no gx.toml, and its
	// items must compile for each adapter.
	Adapter string
}

// CheckWith checks a module with options (REQ-CNT-10).
func CheckWith(root string, opt CheckOptions) []Diagnostic {
	root = absoluteRoot(root)
	l := newLoader()
	l.adapter = opt.Adapter
	dirs := collectDirs(root)
	var out []Diagnostic
	for _, dir := range dirs {
		out = append(out, l.checkDir(dir)...)
	}
	res, tdiags := l.analyze(root, dirs)
	out = append(out, tdiags...)
	out = append(out, checkContent(root, res.collections, nil)...)
	if opt.ExternalLinks {
		out = append(out, checkExternalLinks(res.collections, nil, opt.Client)...)
	}
	sortDiags(out)
	return out
}

// CheckApp is the whole check of an app, as `gx check` runs it: every
// diagnostic of CheckWith, the diagnostics that only the generator can give,
// and a GX1002 for each generated file that is missing or stale. A
// diagnostic that both passes find is reported once.
//
// The check and the generator share one analysis: the generator starts with
// the same steps as CheckWith, so a second analysis finds nothing new and
// takes as long as the first (NFR-05).
func CheckApp(root string, opt CheckOptions) []Diagnostic {
	files, out, res, _, _ := generate(root, nil)
	seen := map[Diagnostic]bool{}
	for _, d := range out {
		seen[d] = true
	}
	add := func(diags []Diagnostic) {
		for _, d := range diags {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	if res != nil {
		add(checkContent(absoluteRoot(root), res.collections, nil))
		if opt.ExternalLinks {
			add(checkExternalLinks(res.collections, nil, opt.Client))
		}
	}
	if files != nil {
		add(staleFiles(files))
	}
	sortDiags(out)
	return out
}

// absoluteRoot makes the tree root absolute so that overlay keys match the
// paths go/packages reports.
func absoluteRoot(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

// collectDirs returns every directory under root that holds a .gx file.
func collectDirs(root string) []string {
	dirs := map[string]bool{}
	islandDirs := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "testdata", ".gx-build":
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".gx") {
			dirs[filepath.Dir(path)] = true
		}
		if isIslandName(d.Name()) {
			islandDirs[filepath.Dir(path)] = true
		}
		return nil
	})
	// A directory with an island and no .gx file is a package of the
	// analysis too: its island needs its checks and its generated code.
	for dir := range islandDirs {
		if dirs[dir] {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		if dirHasIsland(dir, names) {
			dirs[dir] = true
		}
	}
	sorted := make([]string, 0, len(dirs))
	for dir := range dirs {
		sorted = append(sorted, dir)
	}
	sort.Strings(sorted)
	return sorted
}

// checkDir checks one package directory.
func (l *loader) checkDir(dir string) []Diagnostic {
	p := l.load(dir)
	out := append([]Diagnostic{}, p.Diags...)
	out = append(out, checkIslands(p)...)
	for _, f := range p.Files {
		for _, s := range f.Signals {
			if !s.HasDefault {
				out = append(out, Diagnostic{
					Code: CodeSignalDefault,
					File: f.File,
					Line: s.At.Line,
					Col:  s.At.Col,
					Msg:  "signal " + Quoted(s.Name) + " needs an initial value",
					Fix:  "write " + s.Name + " " + s.Type + " = <value>",
				})
			}
		}
		if len(f.Signals) > 0 && firstTopLevelElement(f.Body) == nil {
			// The first values of the signals go on the first top-level
			// HTML element; with none, the browser never gets them.
			out = append(out, Diagnostic{
				Code: CodeSignalRoot,
				File: f.File,
				Line: f.Signals[0].At.Line - 1,
				Col:  1,
				Msg:  "a component with signals needs a top-level HTML element to hold their first values",
				Fix:  "put the markup inside one HTML element, for example <div>...</div>",
			})
		}
		out = l.checkNodes(p, f, f.Body, out)
	}
	return out
}

func (l *loader) checkNodes(p *Package, file *File, ns []Node, diags []Diagnostic) []Diagnostic {
	for _, n := range ns {
		switch t := n.(type) {
		case *Element:
			diags = l.checkElement(p, file, t, diags)
			if !t.HasRaw {
				diags = l.checkNodes(p, file, t.Children, diags)
			}
		case *Control:
			diags = l.checkNodes(p, file, t.Body, diags)
			diags = l.checkNodes(p, file, t.Else, diags)
			for _, c := range t.Cases {
				diags = l.checkNodes(p, file, c.Body, diags)
			}
		}
	}
	return diags
}

func (l *loader) checkElement(p *Package, file *File, el *Element, diags []Diagnostic) []Diagnostic {
	qual, name, isComp := componentTag(el.Name)
	if !isComp {
		return diags
	}
	comp, target, _ := resolveComponent(l, p, file, qual, name)
	if comp == nil {
		diags = append(diags, Diagnostic{
			Code: CodeUnknownComponent,
			File: file.File,
			Line: el.At.Line,
			Col:  el.At.Col,
			Msg:  "unknown component " + Quoted(el.Name) + nearestComponent(target, name),
		})
		return diags
	}

	provided := map[string]bool{}
	for _, a := range el.Attrs {
		if a.Kind == AttrSpread {
			diags = append(diags, Diagnostic{
				Code: CodeSpread,
				File: file.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "attributes cannot spread onto <" + el.Name + ">; pass a gx.Attrs prop instead",
			})
			continue
		}
		if a.Kind == AttrFragment || isDirective(a.Name) {
			continue
		}
		if comp.Island != nil && a.Name == islandLoadAttr {
			if _, _, why := islandLoad(&a); why != "" {
				diags = append(diags, Diagnostic{
					Code: CodeIslandLoad,
					File: file.File,
					Line: a.At.Line,
					Col:  a.At.Col,
					Msg:  "the load attribute of <" + el.Name + "> " + why + "; use eager, idle, visible or media(<query>)",
				})
			}
			continue
		}
		prop, ok := findProp(comp, a.Name)
		if !ok {
			diags = append(diags, Diagnostic{
				Code: CodeUnknownAttr,
				File: file.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "unknown attribute " + Quoted(a.Name) + " on <" + el.Name + ">",
			})
			continue
		}
		if a.Kind == AttrString && prop.Type != "string" {
			diags = append(diags, Diagnostic{
				Code: CodeStaticStringProp,
				File: file.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "attribute " + Quoted(a.Name) + " is a static string but prop " + Quoted(prop.Name) + " has type " + Quoted(prop.Type),
			})
		}
		if a.Kind == AttrBool && prop.Type != "bool" {
			diags = append(diags, Diagnostic{
				Code: CodeStaticStringProp,
				File: file.File,
				Line: a.At.Line,
				Col:  a.At.Col,
				Msg:  "attribute " + Quoted(a.Name) + " is a boolean attribute but prop " + Quoted(prop.Name) + " has type " + Quoted(prop.Type),
			})
		}
		provided[prop.Name] = true
	}
	slots := map[string]bool{}
	for _, child := range el.Children {
		slotEl, ok := child.(*Element)
		if !ok || !strings.HasPrefix(slotEl.Name, ":") {
			continue
		}
		slotName := strings.TrimPrefix(slotEl.Name, ":")
		if slots[slotName] {
			diags = append(diags, Diagnostic{
				Code: CodeDuplicateSlot,
				File: file.File,
				Line: slotEl.At.Line,
				Col:  slotEl.At.Col,
				Msg:  "slot " + Quoted(slotName) + " is given twice on <" + el.Name + ">",
			})
			continue
		}
		slots[slotName] = true
		prop, ok := findProp(comp, slotName)
		if !ok {
			diags = append(diags, Diagnostic{
				Code: CodeUnknownAttr,
				File: file.File,
				Line: slotEl.At.Line,
				Col:  slotEl.At.Col,
				Msg:  "unknown slot " + Quoted(slotName) + " on <" + el.Name + ">",
			})
			continue
		}
		provided[prop.Name] = true
	}
	if hasDefaultContent(el.Children) {
		if _, ok := findProp(comp, "children"); ok {
			provided["Children"] = true
		} else {
			diags = append(diags, Diagnostic{
				Code: CodeUnknownAttr,
				File: file.File,
				Line: el.At.Line,
				Col:  el.At.Col,
				Msg:  "component <" + el.Name + "> has no Children prop",
			})
		}
	}
	for _, prop := range comp.Props {
		if prop.HasDefault || provided[prop.Name] {
			continue
		}
		diags = append(diags, Diagnostic{
			Code: CodeRequiredProp,
			File: file.File,
			Line: el.At.Line,
			Col:  el.At.Col,
			Msg:  "missing required prop " + Quoted(prop.Name) + " on <" + el.Name + ">",
		})
	}
	return diags
}

// hasDefaultContent reports whether children hold content for the default
// slot. Whitespace text, comments and named slots do not count.
func hasDefaultContent(ns []Node) bool {
	for _, n := range ns {
		switch t := n.(type) {
		case *Text:
			if strings.TrimSpace(t.Data) != "" {
				return true
			}
		case *Comment:
		case *Element:
			if !strings.HasPrefix(t.Name, ":") {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func findProp(c *Component, attr string) (Prop, bool) {
	for _, p := range c.Props {
		if lowerFirst(p.Name) == attr {
			return p, true
		}
	}
	return Prop{}, false
}

// nearestComponent returns a suggestion for an unknown component name.
func nearestComponent(p *Package, name string) string {
	if p == nil {
		return ""
	}
	best, bestDist := "", 3
	consider := func(candidate string) {
		d := levenshtein(name, candidate)
		if d < bestDist || (d == bestDist && candidate < best) {
			best, bestDist = candidate, d
		}
	}
	for candidate := range p.Files {
		consider(candidate)
	}
	for candidate := range p.Islands {
		consider(candidate)
	}
	for candidate := range p.Elements {
		consider(candidate)
	}
	if best == "" || bestDist > 2 {
		return ""
	}
	return "; did you mean " + Quoted(best) + "?"
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func sortDiags(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Col != b.Col {
			return a.Col < b.Col
		}
		return a.Code < b.Code
	})
}
