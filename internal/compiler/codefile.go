package compiler

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/highlight"
)

// checkCodeFiles resolves every gx.CodeFile call and reports a missing file
// or line range (GX8004, REQ-CNT-05). The resolved literals are cached for
// codegen.
func (l *loader) checkCodeFiles(res *typesResult, dirs []string) []Diagnostic {
	var out []Diagnostic
	for _, dir := range dirs {
		p := l.load(dir)
		root := ""
		if p.Module != nil {
			root = p.Module.Dir
		}
		for _, f := range p.Files {
			walkNodes(f.Body, func(n Node) {
				switch t := n.(type) {
				case *Expr:
					out = l.checkCodeFileSite(res, f, t, t.Data, t.At, root, out)
				case *Element:
					for i := range t.Attrs {
						a := &t.Attrs[i]
						if a.Kind != AttrExpr {
							continue
						}
						out = l.checkCodeFileSite(res, f, a, a.Value, a.ValueAt, root, out)
					}
				}
			})
		}
	}
	return out
}

// checkCodeFileSite resolves one site whose type is gx.Code.
func (l *loader) checkCodeFileSite(res *typesResult, f *File, site any, raw string, at Pos, root string, out []Diagnostic) []Diagnostic {
	t := res.types[site]
	if t == nil || t.String() != "github.com/alternayte/gx.Code" {
		return out
	}
	path, lines, ok := codeFileCall(raw)
	if !ok {
		out = append(out, Diagnostic{
			Code: CodeCodeFile,
			File: f.File,
			Line: at.Line,
			Col:  at.Col,
			Msg:  "gx.CodeFile needs a constant path and line range",
			Fix:  `write gx.CodeFile("content/docs/start.md", "1-20")`,
		})
		return out
	}
	lit, err := resolveCodeFile(root, path, lines)
	if err != nil {
		out = append(out, Diagnostic{
			Code: CodeCodeFile,
			File: f.File,
			Line: at.Line,
			Col:  at.Col,
			Msg:  err.Error(),
		})
		return out
	}
	if res.codeFiles == nil {
		res.codeFiles = map[any]string{}
	}
	res.codeFiles[site] = lit
	return out
}

// codeFileCall reports whether raw is a gx.CodeFile(path, lines) call with
// constant string arguments.
func codeFileCall(raw string) (path, lines string, ok bool) {
	expr, err := goparser.ParseExpr(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	call, isCall := expr.(*ast.CallExpr)
	if !isCall || len(call.Args) != 2 {
		return "", "", false
	}
	name := ""
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		name = fun.Sel.Name
	case *ast.Ident:
		name = fun.Name
	default:
		return "", "", false
	}
	if name != "CodeFile" {
		return "", "", false
	}
	path, ok1 := stringLit(call.Args[0])
	lines, ok2 := stringLit(call.Args[1])
	if !ok1 || !ok2 {
		return "", "", false
	}
	return path, lines, true
}

// stringLit unquotes a string literal expression.
func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// resolveCodeFile reads path relative to the module root, selects the lines
// and returns the Go literal of the gx.Code value (REQ-CNT-05).
func resolveCodeFile(root, path, lines string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("gx.CodeFile %q needs a module root", path)
	}
	full := filepath.FromSlash(path)
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, full)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("gx.CodeFile cannot read %s", path)
	}
	src, missing := selectCodeLines(string(data), lines)
	if len(missing) > 0 {
		return "", fmt.Errorf("gx.CodeFile %s has no lines %s", path, joinInts(missing))
	}
	lang := highlight.LangForFile(path)
	return "gx.Code{File: " + strconv.Quote(path) +
		", Lang: " + strconv.Quote(lang) +
		", Source: " + strconv.Quote(src) + "}", nil
}

// selectCodeLines returns the text of the selected 1-based lines and the
// selected lines that src does not hold. An empty selection returns all of
// src.
func selectCodeLines(src, lines string) (string, []int) {
	sel := parseCodeLines(lines)
	if len(sel) == 0 {
		src = strings.TrimSuffix(src, "\n")
		if src != "" {
			src += "\n"
		}
		return src, nil
	}
	src = strings.TrimSuffix(src, "\n")
	all := strings.Split(src, "\n")
	var keep []string
	var missing []int
	for _, n := range sel {
		if n < 1 || n > len(all) {
			missing = append(missing, n)
			continue
		}
		keep = append(keep, all[n-1])
	}
	text := strings.Join(keep, "\n")
	if text != "" {
		text += "\n"
	}
	return text, missing
}

// parseCodeLines parses "1,3-5" into 1-based line numbers.
func parseCodeLines(s string) []int {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			start, err1 := strconv.Atoi(strings.TrimSpace(lo))
			end, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 == nil && err2 == nil && start > 0 && end >= start {
				for n := start; n <= end; n++ {
					out = append(out, n)
				}
			}
			continue
		}
		if n, err := strconv.Atoi(part); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// joinInts renders line numbers for a diagnostic.
func joinInts(ns []int) string {
	parts := make([]string, 0, len(ns))
	for _, n := range ns {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}
