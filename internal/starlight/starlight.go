// Package starlight converts a Starlight project into a Gx content tree
// (REQ-CNT-13): frontmatter, the sidebar config from astro.config, MDX to
// Markdown, and Starlight component tags to the docs kit. Everything it
// cannot convert lands in the report.
package starlight

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Options configure one import.
type Options struct {
	// Src is the Starlight project root.
	Src string
	// Out is the Gx content directory.
	Out string
}

// Report lists what the import did and what it could not convert.
type Report struct {
	// Converted counts the written pages.
	Converted int
	// Notes lists what a human must fix, in file order.
	Notes []string
}

// kitComponents maps a Starlight component to its docs kit component.
var kitComponents = map[string]string{
	"Tabs":       "Tabs",
	"TabItem":    "TabItem",
	"Steps":      "Steps",
	"Aside":      "Aside",
	"Card":       "Card",
	"CardGrid":   "CardGrid",
	"LinkCard":   "LinkCard",
	"LinkButton": "LinkButton",
	"Badge":      "Badge",
	"FileTree":   "FileTree",
}

// supportedKeys are the DocMeta keys the converter emits (REQ-CNT-13).
var supportedKeys = map[string]bool{
	"title": true, "description": true, "order": true, "badge": true,
	"sidebarGroup": true, "collapsed": true, "updated": true,
	"template": true, "llms": true,
}

// Convert imports a Starlight project into a Gx content directory.
func Convert(opt Options) (Report, error) {
	var report Report
	if opt.Src == "" || opt.Out == "" {
		return report, fmt.Errorf("starlight: src and out are required")
	}
	contentDir := filepath.Join(opt.Src, "src", "content", "docs")
	if _, err := os.Stat(contentDir); err != nil {
		contentDir = filepath.Join(opt.Src, "content", "docs")
	}
	if _, err := os.Stat(contentDir); err != nil {
		return report, fmt.Errorf("starlight: no content directory under %s", opt.Src)
	}
	sidebar, sidebarNotes, err := loadSidebar(opt.Src)
	report.Notes = append(report.Notes, sidebarNotes...)
	if err != nil {
		report.Notes = append(report.Notes, err.Error())
	}

	var files []string
	_ = filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") || strings.HasSuffix(d.Name(), ".mdx") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	for _, path := range files {
		rel, err := filepath.Rel(contentDir, path)
		if err != nil {
			continue
		}
		slug := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		outPath := filepath.Join(opt.Out, filepath.FromSlash(slug)+".md")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return report, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return report, err
		}
		front, body := splitFrontmatter(data)
		meta, notes := convertFrontmatter(front, sidebar, slug)
		report.Notes = append(report.Notes, notes...)
		if strings.HasSuffix(path, ".mdx") {
			body, notes = convertMDX(body)
			for _, note := range notes {
				report.Notes = append(report.Notes, rel+": "+note)
			}
		}
		var b strings.Builder
		if len(meta) > 0 {
			encoded, err := yaml.Marshal(meta)
			if err != nil {
				return report, err
			}
			b.WriteString("---\n")
			b.Write(encoded)
			b.WriteString("---\n\n")
		}
		b.Write(body)
		if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
			return report, err
		}
		report.Converted++
	}
	return report, nil
}

// splitFrontmatter returns the YAML frontmatter and the body.
func splitFrontmatter(data []byte) ([]byte, []byte) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return nil, data
	}
	rest := s[4:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, data
	}
	front := []byte(rest[:idx])
	after := rest[idx+1:]
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		after = after[nl+1:]
	}
	return front, []byte(strings.TrimPrefix(after, "\n"))
}

// convertFrontmatter translates one Starlight frontmatter into the Gx
// keys and applies the sidebar config (REQ-CNT-13).
func convertFrontmatter(front []byte, sidebar sidebarConfig, slug string) (map[string]any, []string) {
	raw := map[string]any{}
	if len(front) > 0 {
		_ = yaml.Unmarshal(front, &raw)
	}
	out := map[string]any{}
	var notes []string
	for key, value := range raw {
		if supportedKeys[key] {
			out[key] = value
			continue
		}
		switch key {
		case "sidebar":
			side, _ := value.(map[string]any)
			if order, ok := side["order"]; ok {
				out["order"] = order
			}
			if label, ok := side["label"]; ok {
				if _, exists := raw["title"]; !exists {
					out["title"] = label
				} else {
					notes = append(notes, slug+": the sidebar label is not the page title; set it in the shell nav")
				}
			}
			if hidden, ok := side["hidden"].(bool); ok && hidden {
				notes = append(notes, slug+": sidebar.hidden is not converted; keep the page out of the nav by hand")
			}
		case "hero":
			out["template"] = "splash"
		default:
			notes = append(notes, slug+": frontmatter "+key+" is not supported and was dropped")
		}
	}
	if page, ok := sidebar.pages[slug]; ok {
		if page.group != "" {
			if _, exists := out["sidebarGroup"]; !exists {
				out["sidebarGroup"] = page.group
			}
		}
		if page.order > 0 {
			if _, exists := out["order"]; !exists {
				out["order"] = page.order
			}
		}
		if page.badge != "" {
			if _, exists := out["badge"]; !exists {
				out["badge"] = page.badge
			}
		}
		if page.collapsed {
			if _, exists := out["collapsed"]; !exists {
				out["collapsed"] = true
			}
		}
	}
	if len(out) == 0 {
		return nil, notes
	}
	return out, notes
}

// importLine finds an MDX import statement.
var importLine = regexp.MustCompile(`(?m)^import\s+[^\n]*?from\s+['"]([^'"]+)['"];?[ \t]*\n?`)

// tagPattern finds an opening or closing component tag.
var tagPattern = regexp.MustCompile(`</?([A-Z][A-Za-z0-9]*)`)

// convertMDX strips imports and rewrites Starlight component tags
// (REQ-CNT-13).
func convertMDX(body []byte) ([]byte, []string) {
	var notes []string
	text := string(body)
	for _, m := range importLine.FindAllStringSubmatch(text, -1) {
		if strings.Contains(m[1], "@astrojs/starlight/components") {
			continue
		}
		notes = append(notes, "removed import of "+m[1])
	}
	text = importLine.ReplaceAllString(text, "")
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			notes = append(notes, "an import spans several lines and was left in place")
			break
		}
	}
	text = tagPattern.ReplaceAllStringFunc(text, func(tag string) string {
		name := tagPattern.FindStringSubmatch(tag)[1]
		kit, ok := kitComponents[name]
		if !ok {
			notes = append(notes, "component <"+name+"> has no docs kit counterpart")
			return tag
		}
		if strings.HasPrefix(tag, "</") {
			return "</docs." + kit
		}
		return "<docs." + kit
	})
	text = strings.ReplaceAll(text, `<docs.Aside type="`, `<docs.Aside kind="`)
	text = strings.ReplaceAll(text, `<docs.Badge text="`, `<docs.Badge label="`)
	return []byte(text), notes
}

// sidebarItem is one manual sidebar entry.
type sidebarItem struct {
	slug  string
	label string
	badge string
}

// sidebarPage is the sidebar mapping of one page.
type sidebarPage struct {
	group     string
	order     int
	badge     string
	collapsed bool
}

// sidebarConfig is the parsed manual sidebar.
type sidebarConfig struct {
	pages map[string]sidebarPage
}

// loadSidebar finds astro.config and parses its starlight sidebar. The
// notes list what the manual sidebar config could not carry over.
func loadSidebar(src string) (sidebarConfig, []string, error) {
	var path string
	for _, name := range []string{"astro.config.mjs", "astro.config.ts", "astro.config.js", "astro.config.mts"} {
		candidate := filepath.Join(src, name)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return sidebarConfig{pages: map[string]sidebarPage{}}, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return sidebarConfig{pages: map[string]sidebarPage{}}, nil, err
	}
	groups, err := parseSidebar(string(data))
	cfg := sidebarConfig{pages: map[string]sidebarPage{}}
	if err != nil {
		return cfg, nil, err
	}
	var notes []string
	for _, group := range groups {
		notes = append(notes, group.notes...)
		for i, item := range group.items {
			if item.slug == "" {
				continue
			}
			cfg.pages[item.slug] = sidebarPage{
				group:     group.label,
				order:     i + 1,
				badge:     item.badge,
				collapsed: group.collapsed,
			}
		}
	}
	return cfg, notes, nil
}

// sidebarGroup is one manual group of the Starlight sidebar.
type sidebarGroup struct {
	label     string
	collapsed bool
	items     []sidebarItem
	notes     []string
}

// parseSidebar extracts the starlight sidebar array from an astro config
// with a small JavaScript-subset parser (REQ-CNT-13).
func parseSidebar(text string) ([]sidebarGroup, error) {
	p := &jsParser{src: stripJSComments(text)}
	for p.find("sidebar") {
		p.skipSpace()
		if !p.take(':') {
			continue
		}
		p.skipSpace()
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		if _, ok := value.([]any); !ok {
			return nil, fmt.Errorf("starlight: the sidebar config is not a literal array; convert it by hand")
		}
		return sidebarGroups(value), nil
	}
	return nil, nil
}

// sidebarGroups converts parsed sidebar values into groups.
func sidebarGroups(value any) []sidebarGroup {
	list, _ := value.([]any)
	var out []sidebarGroup
	for _, entry := range list {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		group := sidebarGroup{label: stringOf(obj["label"])}
		if collapsed, ok := obj["collapsed"].(bool); ok {
			group.collapsed = collapsed
			group.notes = append(group.notes, "the sidebar group "+group.label+" is collapsed; the shell collapses it when a page sets collapsed: true")
		}
		if auto, ok := obj["autogenerate"].(map[string]any); ok {
			group.notes = append(group.notes, "the sidebar auto-generates from "+stringOf(auto["directory"])+"; folder order applies")
		}
		items, _ := obj["items"].([]any)
		for _, raw := range items {
			switch item := raw.(type) {
			case string:
				group.items = append(group.items, sidebarItem{slug: strings.TrimSuffix(strings.Trim(item, "/"), ".md")})
			case map[string]any:
				if link := stringOf(item["link"]); link != "" {
					group.notes = append(group.notes, "the sidebar link "+link+" is external; add it to the app nav")
					continue
				}
				group.items = append(group.items, sidebarItem{
					slug:  strings.TrimSuffix(strings.Trim(stringOf(item["slug"]), "/"), ".md"),
					label: stringOf(item["label"]),
					badge: stringOf(item["badge"]),
				})
			}
		}
		out = append(out, group)
	}
	return out
}

// stringOf returns a string value or "".
func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

// stripJSComments removes // and /* */ comments from JavaScript.
func stripJSComments(src string) string {
	var b strings.Builder
	inString := byte(0)
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '\'' || c == '`':
			inString = c
			b.WriteByte(c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// jsParser parses the JavaScript subset the sidebar config uses.
type jsParser struct {
	src string
	at  int
}

// skipSpace moves over whitespace.
func (p *jsParser) skipSpace() {
	for p.at < len(p.src) && (p.src[p.at] == ' ' || p.src[p.at] == '\n' || p.src[p.at] == '\t' || p.src[p.at] == '\r') {
		p.at++
	}
}

// take consumes one expected byte.
func (p *jsParser) take(c byte) bool {
	p.skipSpace()
	if p.at < len(p.src) && p.src[p.at] == c {
		p.at++
		return true
	}
	return false
}

// find moves past the first occurrence of word.
func (p *jsParser) find(word string) bool {
	i := strings.Index(p.src[p.at:], word)
	if i < 0 {
		return false
	}
	p.at += i + len(word)
	return true
}

// value parses one array, object, string, number or identifier.
func (p *jsParser) value() (any, error) {
	p.skipSpace()
	if p.at >= len(p.src) {
		return nil, fmt.Errorf("starlight: the sidebar config ended early")
	}
	switch p.src[p.at] {
	case '[':
		p.at++
		var out []any
		for {
			p.skipSpace()
			if p.take(']') {
				return out, nil
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			p.skipSpace()
			_ = p.take(',')
		}
	case '{':
		p.at++
		out := map[string]any{}
		for {
			p.skipSpace()
			if p.take('}') {
				return out, nil
			}
			key, err := p.key()
			if err != nil {
				return nil, err
			}
			if !p.take(':') {
				return nil, fmt.Errorf("starlight: the sidebar key %q has no value", key)
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			out[key] = v
			p.skipSpace()
			_ = p.take(',')
		}
	case '\'', '"', '`':
		return p.string()
	default:
		return p.ident()
	}
}

// key parses an object key.
func (p *jsParser) key() (string, error) {
	p.skipSpace()
	if p.at >= len(p.src) {
		return "", fmt.Errorf("starlight: the sidebar config ended early")
	}
	if p.src[p.at] == '\'' || p.src[p.at] == '"' || p.src[p.at] == '`' {
		return p.string()
	}
	start := p.at
	for p.at < len(p.src) && (isIdentByte(p.src[p.at]) || p.src[p.at] == '-') {
		p.at++
	}
	if start == p.at {
		return "", fmt.Errorf("starlight: cannot read a sidebar key at offset %d", p.at)
	}
	return p.src[start:p.at], nil
}

// string parses a quoted string.
func (p *jsParser) string() (string, error) {
	quote := p.src[p.at]
	p.at++
	var b strings.Builder
	for p.at < len(p.src) {
		c := p.src[p.at]
		if c == '\\' && p.at+1 < len(p.src) {
			p.at++
			b.WriteByte(p.src[p.at])
			p.at++
			continue
		}
		p.at++
		if c == quote {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
	return "", fmt.Errorf("starlight: an unterminated string in the sidebar config")
}

// ident parses an identifier, number or boolean.
func (p *jsParser) ident() (any, error) {
	start := p.at
	for p.at < len(p.src) && (isIdentByte(p.src[p.at]) || p.src[p.at] == '.' || p.src[p.at] == '-') {
		p.at++
	}
	text := p.src[start:p.at]
	if n, err := strconv.Atoi(text); err == nil {
		return n, nil
	}
	switch text {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null", "undefined":
		return nil, nil
	}
	return text, nil
}

// isIdentByte reports whether c can stand in an identifier.
func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
