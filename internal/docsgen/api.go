package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// apiPage is the output path of the API reference of package gx.
const apiPage = "docs/content/reference/api.md"

// apiMarkdown writes the API reference of package gx from its source: each
// exported function, type, method, constant and variable, with its
// signature and the first paragraph of its doc comment (REQ-DOC-02).
func apiMarkdown(root string) (string, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		// The reference shows the production API: no test file, no
		// generated table and no file of the dev build.
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			strings.HasSuffix(name, "_gxdev.go") || name == "cxtable.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, name), nil, parser.ParseComments)
		if err != nil {
			return "", err
		}
		files = append(files, file)
	}
	pkg, err := doc.NewFromFiles(fset, files, "github.com/alternayte/gx")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: \"The gx package\"\n")
	b.WriteString("description: \"Each exported function, type, constant and variable of package gx.\"\n")
	b.WriteString("section: Reference\n")
	b.WriteString("order: 1\n")
	b.WriteString("generated: api\n")
	b.WriteString("---\n\n")
	b.WriteString("Package `github.com/alternayte/gx` is the whole public API of the framework. `just docs-gen` writes this page from the source. The text of each entry is the doc comment of the symbol.\n\n")
	b.WriteString("Generated code calls some of these symbols. An app does not call a symbol whose name starts with `Gx`.\n\n")

	code := func(node any) {
		var buf bytes.Buffer
		_ = (&printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}).Fprint(&buf, fset, node)
		b.WriteString("```go\n" + strings.ReplaceAll(buf.String(), "\t", "    ") + "\n```\n\n")
	}
	text := func(comment string) {
		if para := firstParagraph(comment); para != "" {
			// A word in angle brackets, such as <Name>, is text here and
			// not a component tag of the page.
			b.WriteString(placeholder.ReplaceAllString(para, "`$0`") + "\n\n")
		}
	}
	funcDecl := func(heading string, f *doc.Func) {
		fmt.Fprintf(&b, "%s %s\n\n", heading, f.Name)
		decl := *f.Decl
		decl.Body, decl.Doc = nil, nil
		code(&decl)
		text(f.Doc)
	}

	if len(pkg.Funcs) > 0 {
		b.WriteString("## Functions\n\n")
		for _, f := range pkg.Funcs {
			funcDecl("### func", f)
		}
	}
	if len(pkg.Types) > 0 {
		b.WriteString("## Types\n\n")
		for _, t := range pkg.Types {
			fmt.Fprintf(&b, "### type %s\n\n", t.Name)
			decl := *t.Decl
			decl.Doc = nil
			code(&decl)
			text(t.Doc)
			for _, group := range [][]*doc.Value{t.Consts, t.Vars} {
				for _, v := range group {
					decl := *v.Decl
					decl.Doc = nil
					code(&decl)
					text(v.Doc)
				}
			}
			for _, f := range t.Funcs {
				funcDecl("#### func", f)
			}
			for _, m := range t.Methods {
				fmt.Fprintf(&b, "#### func (%s) %s\n\n", strings.TrimPrefix(m.Recv, "*"), m.Name)
				decl := *m.Decl
				decl.Body, decl.Doc = nil, nil
				code(&decl)
				text(m.Doc)
			}
		}
	}
	values := append(append([]*doc.Value{}, pkg.Consts...), pkg.Vars...)
	if len(values) > 0 {
		sort.SliceStable(values, func(i, j int) bool { return values[i].Names[0] < values[j].Names[0] })
		b.WriteString("## Constants and variables\n\n")
		for _, v := range values {
			fmt.Fprintf(&b, "### %s\n\n", strings.Join(v.Names, ", "))
			decl := *v.Decl
			decl.Doc = nil
			code(&decl)
			text(v.Doc)
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

// placeholder matches a word in angle brackets that is not in a code span.
var placeholder = regexp.MustCompile(`<[A-Za-z][A-Za-z0-9.:-]*>`)

// firstParagraph returns the first paragraph of a doc comment as one line.
// A requirement id in parentheses is for the builders of Gx, not for the
// reader, so it stays out.
func firstParagraph(comment string) string {
	para, _, _ := strings.Cut(strings.TrimSpace(comment), "\n\n")
	para = strings.Join(strings.Fields(para), " ")
	for {
		open := strings.Index(para, " (")
		if open < 0 {
			break
		}
		end := strings.Index(para[open:], ")")
		if end < 0 {
			break
		}
		inner := para[open+2 : open+end]
		if !isRequirementList(inner) {
			// Keep it, and look after it.
			rest := firstParagraph(para[open+end+1:])
			if rest == "" {
				return para
			}
			return para[:open+end+1] + " " + rest
		}
		para = para[:open] + para[open+end+1:]
	}
	return para
}

// isRequirementList reports text such as "REQ-RTE-01" or "NFR-04, SI-03".
func isRequirementList(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		if part == "and" || part == "to" {
			continue
		}
		if !(strings.HasPrefix(part, "REQ-") || strings.HasPrefix(part, "NFR-") || strings.HasPrefix(part, "SI-") || strings.HasPrefix(part, "DR-") || strings.HasPrefix(part, "D-")) {
			return false
		}
	}
	return true
}
