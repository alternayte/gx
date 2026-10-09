package starlight

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Options configure one import.
type Options struct {
	// Src is a local Starlight project root, an http(s) URL of a tarball,
	// or a GitHub repository URL. A GitHub URL downloads the repository
	// archive and finds the Starlight project (REQ-CNT-13).
	Src string
	// Out is the Gx content directory.
	Out string
	// GitHubBase overrides https://codeload.github.com for tests.
	GitHubBase string
	// Client is the HTTP client of a remote import.
	Client *http.Client
	// Log receives progress lines. Defaults to io.Discard.
	Log io.Writer
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

// propRenames maps a Starlight prop to its docs kit prop.
var propRenames = map[string]map[string]string{
	"Tabs":     {"syncKey": "sync"},
	"Aside":    {"type": "kind"},
	"Badge":    {"text": "label"},
	"LinkCard": {"href": "href"},
}

// droppedProps maps a Starlight prop the docs kit does not have.
var droppedProps = map[string]map[string]bool{
	"LinkCard": {"icon": true, "attrs": true},
	"Aside":    {"title": true},
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
	if opt.Log == nil {
		opt.Log = io.Discard
	}
	if opt.Src == "" || opt.Out == "" {
		return report, fmt.Errorf("starlight: src and out are required")
	}
	src := opt.Src
	if isRemote(src) {
		dir, err := fetchProject(opt)
		if err != nil {
			return report, err
		}
		defer os.RemoveAll(dir)
		src = dir
	}
	project, err := findProject(src)
	if err != nil {
		return report, err
	}
	contentDir := project
	for _, candidate := range []string{
		filepath.Join(project, "src", "content", "docs"),
		filepath.Join(project, "content", "docs"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			contentDir = candidate
			break
		}
	}
	sidebar, sidebarNotes, err := loadSidebar(project)
	report.Notes = append(report.Notes, sidebarNotes...)
	if err != nil {
		report.Notes = append(report.Notes, err.Error())
	}
	var files []string
	_ = filepath.WalkDir(contentDir, func(file string, d os.DirEntry, err error) error {
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
			files = append(files, file)
		}
		return nil
	})
	sort.Strings(files)
	for _, file := range files {
		rel, err := filepath.Rel(contentDir, file)
		if err != nil {
			continue
		}
		slug := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		outPath := filepath.Join(opt.Out, filepath.FromSlash(slug)+".md")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return report, err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return report, err
		}
		front, body := splitFrontmatter(data)
		meta, notes := convertFrontmatter(front, sidebar, slug)
		report.Notes = append(report.Notes, notes...)
		body, notes, err = convertBody(body, filepath.Dir(file), map[string]bool{file: true})
		if err != nil {
			return report, err
		}
		for _, note := range notes {
			report.Notes = append(report.Notes, rel+": "+note)
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
		fmt.Fprintf(opt.Log, "imported %s\n", slug)
	}
	return report, nil
}

// isRemote reports whether src is an http(s) URL.
func isRemote(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// findProject returns the directory that holds astro.config or
// src/content/docs, starting at src and looking one level down.
func findProject(src string) (string, error) {
	if hasStarlight(src) {
		return src, nil
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", fmt.Errorf("starlight: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(src, entry.Name())
		if hasStarlight(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("starlight: no Starlight project under %s", src)
}

// hasStarlight reports whether dir holds a Starlight project.
func hasStarlight(dir string) bool {
	for _, name := range []string{"astro.config.mjs", "astro.config.ts", "astro.config.js", "astro.config.mts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "src", "content", "docs")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// fetchProject downloads a remote project archive and returns its root.
func fetchProject(opt Options) (string, error) {
	client := opt.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	base := opt.GitHubBase
	if base == "" {
		base = "https://codeload.github.com"
	}
	urls := []string{opt.Src}
	if owner, repo, sub, ok := parseGitHub(opt.Src); ok {
		urls = nil
		for _, ref := range []string{"main", "master"} {
			urls = append(urls, strings.TrimSuffix(base, "/")+"/"+owner+"/"+repo+"/tar.gz/"+ref)
		}
		_ = sub
	}
	var lastErr error
	for _, url := range urls {
		data, err := download(client, url)
		if err != nil {
			lastErr = err
			continue
		}
		dir, err := os.MkdirTemp("", "gx-starlight-")
		if err != nil {
			return "", err
		}
		if err := extract(data, dir); err != nil {
			os.RemoveAll(dir)
			lastErr = err
			continue
		}
		return dir, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("starlight: cannot download %s", opt.Src)
	}
	return "", lastErr
}

// parseGitHub splits a GitHub URL into owner, repo and an optional tree
// subdirectory.
func parseGitHub(raw string) (owner, repo, sub string, ok bool) {
	rest := raw
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "github.com/"} {
		if strings.HasPrefix(rest, prefix) {
			rest = strings.TrimPrefix(rest, prefix)
			break
		}
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if len(parts) >= 4 && parts[2] == "tree" {
		sub = strings.Join(parts[3:], "/")
	}
	return owner, repo, path.Clean(sub), true
}

// download reads one URL.
func download(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("starlight: download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("starlight: download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// extract unpacks a gzip tarball into dir, stripping its first path
// element.
func extract(data []byte, dir string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("starlight: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("starlight: %w", err)
		}
		name := path.Clean(hdr.Name)
		if i := strings.IndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if name == "" || strings.HasPrefix(name, "..") {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return err
			}
			out, err := os.Create(full)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	return nil
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
			if hero, ok := value.(map[string]any); ok {
				if tagline, ok := hero["tagline"]; ok {
					if _, exists := out["description"]; !exists {
						out["description"] = tagline
					}
				}
				if actions, ok := hero["actions"].([]any); ok && len(actions) > 0 {
					notes = append(notes, slug+": the hero actions are not converted; add them to the splash component")
				}
			}
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
	for _, dir := range sortedKeys(sidebar.auto) {
		if slug == dir || strings.HasPrefix(slug, dir+"/") {
			if _, exists := out["sidebarGroup"]; !exists {
				out["sidebarGroup"] = sidebar.auto[dir]
			}
			if sidebar.collapsedAuto[dir] {
				if _, exists := out["collapsed"]; !exists {
					out["collapsed"] = true
				}
			}
			break
		}
	}
	if len(out) == 0 {
		return nil, notes
	}
	return out, notes
}

// sortedKeys returns the keys of a map, longest first.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// importDefault finds `import Name from 'path'`.
var importDefault = regexp.MustCompile(`(?m)^import\s+([A-Za-z_$][A-Za-z0-9_$]*)\s+from\s+['"]([^'"]+)['"];?[ \t]*\n?`)

// importNamed finds `import { A, B } from 'path'`.
var importNamed = regexp.MustCompile(`(?m)^import\s*\{([^}]*)\}\s*from\s*['"]([^'"]+)['"];?[ \t]*\n?`)

// convertBody converts one MDX body: imports become snippet inlines or
// notes, and component tags become docs kit tags (REQ-CNT-13).
func convertBody(body []byte, dir string, seen map[string]bool) ([]byte, []string, error) {
	text := string(body)
	var notes []string
	snippets := map[string]string{}
	known := map[string]bool{}
	for _, m := range importDefault.FindAllStringSubmatch(text, -1) {
		alias, target := m[1], m[2]
		if strings.HasSuffix(target, ".md") || strings.HasSuffix(target, ".mdx") {
			snippets[alias] = filepath.Join(dir, filepath.FromSlash(target))
			continue
		}
		if strings.Contains(target, "@astrojs/starlight/components") {
			continue
		}
		notes = append(notes, "removed import of "+target)
	}
	for _, m := range importNamed.FindAllStringSubmatch(text, -1) {
		target := m[2]
		if strings.Contains(target, "@astrojs/starlight/components") {
			for _, name := range strings.Split(m[1], ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					known[name] = true
				}
			}
			continue
		}
		notes = append(notes, "removed import from "+target)
		for _, name := range strings.Split(m[1], ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				known[name] = true
			}
		}
	}
	out, notes2, err := rewriteTags(text, dir, snippets, known, seen)
	notes = append(notes, notes2...)
	if err != nil {
		return nil, notes, err
	}
	out = importDefault.ReplaceAllString(out, "")
	out = importNamed.ReplaceAllString(out, "")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "import ") {
			notes = append(notes, "an import spans several lines and was left in place")
			break
		}
	}
	return []byte(out), notes, nil
}

// rewriteTags rewrites component tags outside code fences and inline code.
func rewriteTags(text, dir string, snippets map[string]string, known map[string]bool, seen map[string]bool) (string, []string, error) {
	var b strings.Builder
	var notes []string
	i := 0
	atLineStart := true
	for i < len(text) {
		switch {
		case atLineStart && (strings.HasPrefix(text[i:], "```") || strings.HasPrefix(text[i:], "~~~")):
			fence := text[i : i+3]
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			b.WriteString(text[i : i+end+1])
			i += end + 1
			for i < len(text) {
				lineEnd := strings.IndexByte(text[i:], '\n')
				stop := len(text)
				if lineEnd >= 0 {
					stop = i + lineEnd + 1
				}
				b.WriteString(text[i:stop])
				trimmed := strings.TrimSpace(text[i:stop])
				i = stop
				if strings.HasPrefix(trimmed, strings.Repeat(fence[:1], 3)) {
					break
				}
			}
			atLineStart = false
		case atLineStart && (strings.HasPrefix(text[i:], "    ") || strings.HasPrefix(text[i:], "\t")) && !importedTagLine(text[i:], snippets, known):
			// An indented code block line.
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			b.WriteString(text[i : i+end+1])
			i += end + 1
		case text[i] == '\n':
			b.WriteByte('\n')
			i++
			atLineStart = true
		case text[i] == '`':
			run := 0
			for i+run < len(text) && text[i+run] == '`' {
				run++
			}
			marker := strings.Repeat("`", run)
			end := strings.Index(text[i+run:], marker)
			if end < 0 {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			stop := i + run + end + run
			b.WriteString(text[i:stop])
			i = stop
			atLineStart = false
		case i+1 < len(text) && text[i] == '<' && text[i+1] == '/':
			name, _ := readName(text[i+2:])
			if name == "" || !isUpperName(name) {
				end := strings.IndexByte(text[i:], '>')
				if end < 0 {
					b.WriteString(text[i:])
					i = len(text)
					continue
				}
				b.WriteString(text[i : i+end+1])
				i += end + 1
				atLineStart = false
				continue
			}
			end := strings.IndexByte(text[i:], '>')
			if end < 0 {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			b.WriteString("</docs." + baseName(name) + ">")
			i += end + 1
			atLineStart = false
		case text[i] == '<':
			name, n := readName(text[i+1:])
			if name == "" || !isUpperName(name) {
				end := strings.IndexByte(text[i:], '>')
				if end < 0 {
					b.WriteString(text[i:])
					i = len(text)
					continue
				}
				b.WriteString(text[i : i+end+1])
				i += end + 1
				atLineStart = false
				continue
			}
			end := findTagEnd(text, i+1+n)
			if end < 0 {
				b.WriteString(text[i:])
				i = len(text)
				continue
			}
			raw := text[i : end+1]
			selfClose := strings.HasSuffix(strings.TrimSpace(raw), "/>")
			if file, ok := snippets[name]; ok {
				inlined, err := inlineSnippet(file, seen)
				if err != nil {
					return "", notes, err
				}
				converted, more, err := convertBody([]byte(inlined), filepath.Dir(file), seen)
				if err != nil {
					return "", notes, err
				}
				// The guard is for a snippet that includes itself. A page
				// can use one snippet two times.
				delete(seen, file)
				notes = append(notes, more...)
				b.WriteString(indentSnippet(string(converted), lineIndent(b.String())))
				i = end + 1
				atLineStart = false
				continue
			}
			rewritten, note, drop := rewriteTag(raw, name, selfClose, known)
			if note != "" {
				notes = append(notes, note)
			}
			if !drop {
				b.WriteString(rewritten)
			}
			i = end + 1
			atLineStart = false
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String(), notes, nil
}

// importedTagLine reports whether a line starts, after its indent, with the
// tag of an imported component or snippet. Such a line is content and not
// an indented code block: the item of a list with ten or more items has an
// indent of four spaces.
func importedTagLine(line string, snippets map[string]string, known map[string]bool) bool {
	line = strings.TrimLeft(line, " \t")
	line = strings.TrimPrefix(line, "<")
	line = strings.TrimPrefix(line, "/")
	name, _ := readName(line)
	if name == "" {
		return false
	}
	_, snippet := snippets[name]
	return snippet || known[name]
}

// lineIndent returns the white space of the last line of out, when that
// line holds white space only. An include tag in a list item has the indent
// of the item.
func lineIndent(out string) string {
	line := out[strings.LastIndexByte(out, '\n')+1:]
	if strings.TrimLeft(line, " \t") != "" {
		return ""
	}
	return line
}

// indentSnippet puts indent before each line of an inlined snippet but the
// first, which follows the indent of the include tag. The snippet then
// stays in the list item that holds the tag.
func indentSnippet(snippet, indent string) string {
	if indent == "" {
		return snippet
	}
	lines := strings.Split(snippet, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// inlineSnippet reads one MDX include, with a cycle guard.
func inlineSnippet(file string, seen map[string]bool) (string, error) {
	if seen[file] {
		return "", fmt.Errorf("starlight: the snippet %s includes itself", file)
	}
	seen[file] = true
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("starlight: %w", err)
	}
	_, body := splitFrontmatter(data)
	return string(body), nil
}

// rewriteTag rewrites one component tag. It reports a note, or drop when
// the tag has no docs kit counterpart.
func rewriteTag(raw, name string, selfClose bool, known map[string]bool) (out, note string, drop bool) {
	kit, ok := kitComponents[name]
	if !ok {
		return "", "component <" + name + "> has no docs kit counterpart and was removed", true
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(raw, "<"+name), ">")
	inner = strings.TrimSuffix(strings.TrimSuffix(inner, "/"), ">")
	inner = strings.TrimSuffix(strings.TrimSpace(inner), "/")
	props, notes := rewriteProps(name, inner)
	end := ">"
	if selfClose {
		end = " />"
	}
	out = "<docs." + kit
	if props != "" {
		out += " " + props
	}
	out += end
	return out, strings.Join(notes, "; "), false
}

// rewriteProps renames and drops the props of one tag.
func rewriteProps(name, inner string) (string, []string) {
	var out []string
	var notes []string
	i := 0
	for i < len(inner) {
		for i < len(inner) && isSpace(inner[i]) {
			i++
		}
		if i >= len(inner) {
			break
		}
		start := i
		for i < len(inner) && !isSpace(inner[i]) && inner[i] != '=' {
			i++
		}
		key := inner[start:i]
		for i < len(inner) && isSpace(inner[i]) {
			i++
		}
		value := ""
		if i < len(inner) && inner[i] == '=' {
			i++
			for i < len(inner) && isSpace(inner[i]) {
				i++
			}
			if i < len(inner) && (inner[i] == '"' || inner[i] == '\'') {
				quote := inner[i]
				i++
				valueStart := i
				for i < len(inner) && inner[i] != quote {
					i++
				}
				value = inner[valueStart:i]
				if i < len(inner) {
					i++
				}
			} else if i < len(inner) && inner[i] == '{' {
				depth := 0
				valueStart := i
				for i < len(inner) {
					if inner[i] == '{' {
						depth++
					}
					if inner[i] == '}' {
						depth--
						if depth == 0 {
							i++
							break
						}
					}
					i++
				}
				value = inner[valueStart:i]
			}
		}
		if key == "" {
			i++
			continue
		}
		if droppedProps[name][key] {
			notes = append(notes, "the "+key+" prop of <"+name+"> was dropped")
			continue
		}
		if renamed, ok := propRenames[name][key]; ok {
			key = renamed
			notes = append(notes, "the <"+name+"> prop was renamed to "+renamed)
		}
		if value == "" {
			out = append(out, key)
			continue
		}
		if strings.HasPrefix(value, "{") {
			out = append(out, key+"="+value)
			continue
		}
		out = append(out, key+"=\""+value+"\"")
	}
	return strings.Join(out, " "), notes
}

// findTagEnd returns the index of the ">" that closes a tag, respecting
// quoted values and brace expressions.
func findTagEnd(text string, i int) int {
	for i < len(text) {
		switch text[i] {
		case '"', '\'':
			quote := text[i]
			i++
			for i < len(text) && text[i] != quote {
				i++
			}
		case '{':
			depth := 0
			for i < len(text) {
				if text[i] == '{' {
					depth++
				}
				if text[i] == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				i++
			}
		case '>':
			return i
		}
		i++
	}
	return -1
}

// readName reads a tag name.
func readName(s string) (string, int) {
	i := 0
	for i < len(s) && (isNameByte(s[i]) || s[i] == '.') {
		i++
	}
	return s[:i], i
}

// baseName returns a tag name without its qualifier.
func baseName(name string) string {
	if dot := strings.LastIndexByte(name, '.'); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

// isUpperName reports whether a tag names a component.
func isUpperName(name string) bool {
	base := baseName(name)
	return base != "" && base[0] >= 'A' && base[0] <= 'Z'
}

// isNameByte reports whether c can stand in a tag name.
func isNameByte(c byte) bool {
	return c == '_' || c == '-' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// isSpace reports whether c is whitespace.
func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

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
	// auto maps an autogenerated directory to its group label.
	auto map[string]string
	// collapsedAuto holds the autogenerated directories whose group
	// starts closed.
	collapsedAuto map[string]bool
}

// loadSidebar finds astro.config and parses its starlight sidebar. The
// notes list what the manual sidebar config could not carry over.
func loadSidebar(src string) (sidebarConfig, []string, error) {
	var file string
	for _, name := range []string{"astro.config.mjs", "astro.config.ts", "astro.config.js", "astro.config.mts"} {
		candidate := filepath.Join(src, name)
		if _, err := os.Stat(candidate); err == nil {
			file = candidate
			break
		}
	}
	if file == "" {
		return sidebarConfig{pages: map[string]sidebarPage{}, auto: map[string]string{}, collapsedAuto: map[string]bool{}}, nil, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return sidebarConfig{pages: map[string]sidebarPage{}, auto: map[string]string{}, collapsedAuto: map[string]bool{}}, nil, err
	}
	groups, err := parseSidebar(string(data))
	cfg := sidebarConfig{pages: map[string]sidebarPage{}, auto: map[string]string{}, collapsedAuto: map[string]bool{}}
	if err != nil {
		return cfg, nil, err
	}
	var notes []string
	for _, group := range groups {
		notes = append(notes, group.notes...)
		for _, dir := range group.autoDirs {
			cfg.auto[dir] = group.label
			if group.collapsed {
				cfg.collapsedAuto[dir] = true
			}
		}
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

// sidebarItem is one manual sidebar entry.
type sidebarItem struct {
	slug  string
	label string
	badge string
}

// sidebarGroup is one manual group of the Starlight sidebar.
type sidebarGroup struct {
	label     string
	collapsed bool
	items     []sidebarItem
	autoDirs  []string
	notes     []string
}

// parseSidebar extracts the starlight sidebar array from an astro config
// with a small JavaScript-subset parser (REQ-CNT-13).
func parseSidebar(text string) ([]sidebarGroup, error) {
	p := &jsParser{src: stripJSComments(text)}
	if !p.find("sidebar") {
		return nil, nil
	}
	p.skipSpace()
	if !p.take(':') {
		return nil, nil
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

// sidebarGroups converts parsed sidebar values into groups. Nested groups
// fold into their parent label.
func sidebarGroups(value any) []sidebarGroup {
	return sidebarGroupsIn(value, "")
}

// sidebarGroupsIn converts one level of the sidebar.
func sidebarGroupsIn(value any, prefix string) []sidebarGroup {
	list, _ := value.([]any)
	var out []sidebarGroup
	for _, entry := range list {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		label := stringOf(obj["label"])
		full := label
		if prefix != "" {
			full = prefix
		}
		group := sidebarGroup{label: full}
		if collapsed, ok := obj["collapsed"].(bool); ok && collapsed {
			group.collapsed = true
		}
		if auto, ok := obj["autogenerate"].(map[string]any); ok {
			dir := strings.Trim(stringOf(auto["directory"]), "/")
			group.autoDirs = append(group.autoDirs, dir)
			group.notes = append(group.notes, "the sidebar auto-generates from "+dir+"; folder order applies")
		}
		var nested []sidebarGroup
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
				if slug := stringOf(item["slug"]); slug != "" {
					group.items = append(group.items, sidebarItem{
						slug:  strings.TrimSuffix(strings.Trim(slug, "/"), ".md"),
						label: stringOf(item["label"]),
						badge: stringOf(item["badge"]),
					})
					continue
				}
				if _, ok := item["autogenerate"]; ok {
					if auto, ok := item["autogenerate"].(map[string]any); ok {
						dir := strings.Trim(stringOf(auto["directory"]), "/")
						group.autoDirs = append(group.autoDirs, dir)
						group.notes = append(group.notes, "the sidebar auto-generates from "+dir+"; folder order applies")
					}
					continue
				}
				// A nested group keeps its own label; the app nests it.
				nested = append(nested, sidebarGroupsIn([]any{item}, "")...)
			}
		}
		out = append(out, group)
		out = append(out, nested...)
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
