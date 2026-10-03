package lsp

import (
	"regexp"
	"sort"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
)

// codeAction is one LSP code action.
type codeAction struct {
	Title       string          `json:"title"`
	Kind        string          `json:"kind"`
	Diagnostics []lspDiagnostic `json:"diagnostics,omitempty"`
	Edit        *workspaceEdit  `json:"edit,omitempty"`
}

// workspaceEdit is one LSP workspace edit.
type workspaceEdit struct {
	Changes map[string][]map[string]any `json:"changes"`
}

var (
	propMsg   = regexp.MustCompile(`missing required prop "([^"]+)" on <([^>]+)>`)
	compoMsg  = regexp.MustCompile(`unknown component "([^"]+)"(?:; did you mean "([^"]+)")?`)
	undefined = regexp.MustCompile(`undefined: ([A-Za-z_][A-Za-z0-9_]*)`)
	nearest   = regexp.MustCompile(`did you mean "([^"]+)"`)
)

// codeActions returns quick fixes for the given diagnostics plus organize
// imports (REQ-DEV-08).
func codeActions(doc *document, m *compiler.Model, r rng, diags []lspDiagnostic) any {
	var out []codeAction
	f := fileFor(m, doc.Path)
	for _, d := range diags {
		switch d.Code {
		case "GX2008":
			if a, ok := fragmentParamAction(doc, d); ok {
				out = append(out, a)
			}
		case "GX2001":
			if a, ok := missingPropAction(doc, m, d); ok {
				out = append(out, a)
			}
		case "GX2002":
			if a, ok := unknownComponentAction(doc, m, f, d); ok {
				out = append(out, a)
			}
		}
	}
	if edit := organizeImports(doc, m, f); edit != nil {
		out = append(out, codeAction{Title: "Organize imports", Kind: "source.organizeImports", Edit: edit})
	}
	if out == nil {
		return []codeAction{}
	}
	return out
}

// fragmentParamAction adds a free variable to the #fragment params
// (REQ-DEV-08).
func fragmentParamAction(doc *document, d lspDiagnostic) (codeAction, bool) {
	m := undefined.FindStringSubmatch(d.Message)
	if m == nil {
		return codeAction{}, false
	}
	name := m[1]
	// Find the fragment attribute whose line holds the diagnostic.
	for _, idx := range allIndexes(doc.Text, "#") {
		line, col := positionAt(doc.Text, idx)
		if line != d.Range.Start.Line {
			continue
		}
		rest := doc.Text[idx+1:]
		nameEnd := 0
		for nameEnd < len(rest) && isWordByte(rest[nameEnd]) {
			nameEnd++
		}
		if nameEnd == 0 {
			continue
		}
		fragName := rest[:nameEnd]
		params := ""
		if nameEnd < len(rest) && rest[nameEnd] == '(' {
			end := strings.IndexByte(rest[nameEnd:], ')')
			if end < 0 {
				continue
			}
			params = rest[nameEnd+1 : nameEnd+end]
		}
		newText := "#" + fragName
		if params == "" {
			newText += "(" + name + ")"
		} else {
			newText += "(" + params + ", " + name + ")"
		}
		edit := replaceAt(doc, idx, nameEnd+1, newText)
		_ = col
		return codeAction{
			Title:       "Add " + name + " to the #" + fragName + " params",
			Kind:        "quickfix",
			Diagnostics: []lspDiagnostic{d},
			Edit:        &workspaceEdit{Changes: map[string][]map[string]any{pathToURI(doc.Path): {edit}}},
		}, true
	}
	return codeAction{}, false
}

// missingPropAction inserts a required prop on the component tag
// (REQ-DEV-08).
func missingPropAction(doc *document, m *compiler.Model, d lspDiagnostic) (codeAction, bool) {
	msg := propMsg.FindStringSubmatch(d.Message)
	if msg == nil {
		return codeAction{}, false
	}
	prop, tag := msg[1], msg[2]
	zero := "nil"
	if comp := resolveTag(m, fileFor(m, doc.Path), tag); comp != nil {
		for _, p := range comp.Props {
			if p.Name == prop {
				zero = zeroValue(p.Type)
			}
		}
	}
	attr := lowerFirst(prop)
	insert := " " + attr + "={" + zero + "}"
	// Insert after the tag name.
	offset := doc.offsetAt(d.Range.Start)
	if offset >= len(doc.Text) || doc.Text[offset] != '<' {
		if i := strings.IndexByte(doc.Text[offset:], '<'); i >= 0 {
			offset += i
		}
	}
	end := offset + 1
	for end < len(doc.Text) && (isWordByte(doc.Text[end]) || doc.Text[end] == '.') {
		end++
	}
	edit := replaceAt(doc, end, 0, insert)
	return codeAction{
		Title:       "Add required prop " + prop,
		Kind:        "quickfix",
		Diagnostics: []lspDiagnostic{d},
		Edit:        &workspaceEdit{Changes: map[string][]map[string]any{pathToURI(doc.Path): {edit}}},
	}, true
}

// unknownComponentAction imports the nearest component and qualifies the
// tag (REQ-DEV-08).
func unknownComponentAction(doc *document, m *compiler.Model, f *compiler.File, d lspDiagnostic) (codeAction, bool) {
	if f == nil {
		return codeAction{}, false
	}
	msg := compoMsg.FindStringSubmatch(d.Message)
	if msg == nil {
		return codeAction{}, false
	}
	tag := msg[1]
	_, base := splitTag(tag)
	var match *compiler.ComponentRef
	if suggested := msg[2]; suggested != "" {
		if comp := resolveTag(m, f, suggested); comp != nil {
			match = comp
		} else {
			for i := range m.Components {
				if m.Components[i].Name == suggested {
					match = &m.Components[i]
					break
				}
			}
		}
	}
	if match == nil {
		best, bestDist := -1, 3
		for i := range m.Components {
			if d := levenshtein(base, m.Components[i].Name); d < bestDist {
				best, bestDist = i, d
			}
		}
		if best >= 0 {
			match = &m.Components[best]
		}
	}
	if match == nil {
		return codeAction{}, false
	}
	newTag := match.Name
	changes := map[string][]map[string]any{}
	if qualifierFor(f, match.PkgPath) == "" {
		importQual := importBase(match.PkgPath)
		newTag = importQual + "." + match.Name
		if edit := addImport(doc, `"`+match.PkgPath+`"`); edit != nil {
			changes[pathToURI(doc.Path)] = append(changes[pathToURI(doc.Path)], edit)
		}
	}
	// Replace the tag name in the source.
	offset := doc.offsetAt(d.Range.Start)
	end := offset + 1
	for end < len(doc.Text) && (isWordByte(doc.Text[end]) || doc.Text[end] == '.') {
		end++
	}
	changes[pathToURI(doc.Path)] = append(changes[pathToURI(doc.Path)], replaceAt(doc, offset+1, end-offset-1, newTag))
	changes[pathToURI(doc.Path)] = sortEdits(changes[pathToURI(doc.Path)])
	return codeAction{
		Title:       "Import " + match.PkgPath + " and use " + newTag,
		Kind:        "quickfix",
		Diagnostics: []lspDiagnostic{d},
		Edit:        &workspaceEdit{Changes: changes},
	}, true
}

// organizeImports sorts the imports, drops unused ones and adds the
// packages of dotted component tags (REQ-TLS-04).
func organizeImports(doc *document, m *compiler.Model, f *compiler.File) *workspaceEdit {
	if f == nil {
		return nil
	}
	used := map[string]bool{}
	collect := func(text string) {
		for _, imp := range f.Imports {
			if strings.Contains(text, compiler.ImportQualifier(imp.Raw)+".") {
				used[imp.Raw] = true
			}
		}
	}
	for _, node := range f.Body {
		walkFileNode(node, func(s string) { collect(s) })
	}
	var raws []string
	for _, imp := range f.Imports {
		if used[imp.Raw] {
			raws = append(raws, imp.Raw)
		}
	}
	// Add the packages of dotted tags that are not imported yet.
	for _, node := range f.Body {
		collectTagImports(node, m, f, &raws)
	}
	sort.Strings(raws)
	raws = dedupe(raws)
	block := renderImports(raws)
	start, end := importSection(doc.Text)
	if start < 0 {
		if block == "" {
			return nil
		}
		// Insert after the package line.
		lineEnd := strings.IndexByte(doc.Text, '\n')
		if lineEnd < 0 {
			return nil
		}
		edit := replaceAt(doc, lineEnd+1, 0, "\n"+block)
		return &workspaceEdit{Changes: map[string][]map[string]any{pathToURI(doc.Path): {edit}}}
	}
	if doc.Text[start:end] == block {
		return nil
	}
	edit := replaceAt(doc, start, end-start, block)
	return &workspaceEdit{Changes: map[string][]map[string]any{pathToURI(doc.Path): {edit}}}
}

func collectTagImports(node compiler.Node, m *compiler.Model, f *compiler.File, raws *[]string) {
	switch t := node.(type) {
	case *compiler.Element:
		if qual, name := splitTag(t.Name); qual != "" {
			path := importPathOf(f, qual)
			if path != "" {
				if !hasImport(*raws, path) {
					*raws = append(*raws, `"`+path+`"`)
				}
			} else {
				var candidates []string
				for i := range m.Components {
					if m.Components[i].Name != name {
						continue
					}
					if importBase(m.Components[i].PkgPath) == qual {
						candidates = append(candidates, m.Components[i].PkgPath)
					}
				}
				if len(candidates) == 1 && !hasImport(*raws, candidates[0]) {
					*raws = append(*raws, `"`+candidates[0]+`"`)
				}
			}
		}
		for _, child := range t.Children {
			collectTagImports(child, m, f, raws)
		}
	case *compiler.Control:
		for _, child := range t.Body {
			collectTagImports(child, m, f, raws)
		}
		for _, child := range t.Else {
			collectTagImports(child, m, f, raws)
		}
		for _, c := range t.Cases {
			for _, child := range c.Body {
				collectTagImports(child, m, f, raws)
			}
		}
	}
}

func hasImport(raws []string, path string) bool {
	for _, raw := range raws {
		if compiler.ImportPath(raw) == path {
			return true
		}
	}
	return false
}

// importSection returns the byte range of the import block, or -1.
func importSection(text string) (int, int) {
	lines := strings.SplitAfter(text, "\n")
	offset := 0
	start := -1
	end := -1
	depth := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if strings.HasPrefix(trimmed, "import (") {
				start = offset
				depth = true
			} else if strings.HasPrefix(trimmed, "import ") {
				start = offset
				end = offset + len(line)
			}
		} else if depth {
			if trimmed == ")" {
				end = offset + len(line)
				break
			}
		} else if strings.HasPrefix(trimmed, "import ") {
			end = offset + len(line)
		} else if trimmed != "" {
			break
		}
		offset += len(line)
	}
	if start < 0 {
		return -1, -1
	}
	if end < 0 {
		end = start
	}
	return start, end
}

func renderImports(raws []string) string {
	if len(raws) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("import (\n")
	for _, raw := range raws {
		b.WriteString("\t" + raw + "\n")
	}
	b.WriteString(")\n")
	return b.String()
}

func addImport(doc *document, raw string) map[string]any {
	block := renderImports([]string{raw})
	start, end := importSection(doc.Text)
	if start < 0 {
		lineEnd := strings.IndexByte(doc.Text, '\n')
		if lineEnd < 0 {
			return nil
		}
		return replaceAt(doc, lineEnd+1, 0, "\n"+block)
	}
	insert := "\t" + raw + "\n"
	return replaceAt(doc, end, 0, insert)
}

func dedupe(xs []string) []string {
	out := xs[:0]
	for i, x := range xs {
		if i > 0 && xs[i-1] == x {
			continue
		}
		out = append(out, x)
	}
	return out
}

func zeroValue(typ string) string {
	switch {
	case typ == "string":
		return `""`
	case typ == "bool":
		return "false"
	case strings.HasPrefix(typ, "int"), strings.HasPrefix(typ, "float"), strings.HasPrefix(typ, "uint"):
		return "0"
	case typ == "gx.Node", typ == "gx.Attrs", strings.HasPrefix(typ, "[]"), strings.HasPrefix(typ, "map["), strings.HasPrefix(typ, "func"):
		return "nil"
	default:
		return "nil"
	}
}

func importBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func walkFileNode(node compiler.Node, fn func(string)) {
	switch t := node.(type) {
	case *compiler.Expr:
		fn(t.Data)
	case *compiler.Let:
		fn(t.Expr)
	case *compiler.Element:
		for _, a := range t.Attrs {
			fn(a.Value)
		}
		for _, child := range t.Children {
			walkFileNode(child, fn)
		}
	case *compiler.Control:
		fn(t.Header)
		for _, child := range t.Body {
			walkFileNode(child, fn)
		}
		for _, child := range t.Else {
			walkFileNode(child, fn)
		}
		for _, c := range t.Cases {
			fn(c.Header)
			for _, child := range c.Body {
				walkFileNode(child, fn)
			}
		}
	}
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
