// Package docscheck reads the pages of the docs site for the tests that
// keep them true: the diagnostic pages (REQ-DOC-03), the code samples
// (REQ-DOC-04) and the page inventory (REQ-DOC-02).
package docscheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Page is one Markdown page of the docs site.
type Page struct {
	// Slug is the site path with no slashes at its ends, for example
	// "errors/GX2001".
	Slug string
	// File is the path of the Markdown file.
	File string
	// Meta holds the frontmatter keys with a plain value.
	Meta map[string]string
	// Body is the Markdown after the frontmatter.
	Body string
}

// Block is one fenced code block.
type Block struct {
	Lang string
	// Title is the title option of the block: the file a sample belongs
	// to, or the command of an output block.
	Title string
	Code  string
	// Line is the line of the opening fence in the body.
	Line int
}

// ContentDir returns the content directory of the docs site.
func ContentDir(repo string) string { return filepath.Join(repo, "docs", "content") }

// Pages reads every Markdown page under dir.
func Pages(dir string) ([]Page, error) {
	var out []Page
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		page := Page{Slug: strings.TrimSuffix(filepath.ToSlash(rel), ".md"), File: path, Meta: map[string]string{}}
		text := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if strings.HasPrefix(text, "---\n") {
			if end := strings.Index(text[4:], "\n---\n"); end >= 0 {
				for _, line := range strings.Split(text[4:4+end], "\n") {
					key, value, ok := strings.Cut(line, ":")
					if !ok || strings.HasPrefix(line, " ") {
						continue
					}
					value = strings.TrimSpace(value)
					if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
						value = strings.ReplaceAll(value[1:len(value)-1], `\"`, `"`)
					}
					page.Meta[strings.TrimSpace(key)] = value
				}
				text = text[4+end+5:]
			}
		}
		page.Body = text
		out = append(out, page)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, err
}

var titleOption = regexp.MustCompile(`title="([^"]*)"`)

// Blocks returns the fenced code blocks of a page, in order.
func (p Page) Blocks() []Block {
	var out []Block
	lines := strings.Split(p.Body, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}
		fence := trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, "`"))]
		info := strings.TrimSpace(trimmed[len(fence):])
		block := Block{Line: i + 1}
		block.Lang, _, _ = strings.Cut(info, " ")
		if m := titleOption.FindStringSubmatch(info); m != nil {
			block.Title = m[1]
		}
		indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		var code []string
		for i++; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == fence {
				break
			}
			line := lines[i]
			if len(line) >= indent && strings.TrimSpace(line[:indent]) == "" {
				line = line[indent:]
			}
			code = append(code, line)
		}
		block.Code = strings.Join(code, "\n")
		if block.Code != "" {
			block.Code += "\n"
		}
		out = append(out, block)
	}
	return out
}

// Section returns the text under a "## name" heading, up to the next
// heading of the same level.
func (p Page) Section(name string) string {
	lines := strings.Split(p.Body, "\n")
	var out []string
	in, fenced := false, false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(line[3:]) == name
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
