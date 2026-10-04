package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// contentCollection is one gx.Collection declaration found in Go code
// (REQ-CNT-02).
type contentCollection struct {
	dir   string // absolute directory
	comps map[string]contentComp
	meta  map[string]string // Meta frontmatter fields, nil when unknown
}

// contentComp is one component of a collection's Components list.
type contentComp struct {
	name  string
	props map[string]Prop // lower-first name -> prop
}

// collectCollections finds every gx.Collection(...).Components(...) call and
// the component set of each (REQ-CNT-03).
func collectCollections(pkgs []*packages.Package) []contentCollection {
	var out []contentCollection
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Components" {
					return true
				}
				inner, ok := sel.X.(*ast.CallExpr)
				if !ok || !isGxFuncExpr(pkg, inner.Fun, "Collection") {
					return true
				}
				if len(inner.Args) == 0 {
					return true
				}
				lit, ok := inner.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				dir, err := strconv.Unquote(lit.Value)
				if err != nil || dir == "" {
					return true
				}
				base := dir
				if !filepath.IsAbs(base) {
					root := ""
					if pkg.Module != nil {
						root = pkg.Module.Dir
					} else if len(pkg.CompiledGoFiles) > 0 {
						root = filepath.Dir(pkg.CompiledGoFiles[0])
					}
					base = filepath.Join(root, filepath.FromSlash(dir))
				}
				coll := contentCollection{dir: filepath.Clean(base), comps: map[string]contentComp{}}
				metaType := pkg.TypesInfo.TypeOf(inner)
				if ptr, ok := metaType.(*types.Pointer); ok {
					metaType = ptr.Elem()
				}
				if named, ok := metaType.(*types.Named); ok {
					if args := named.TypeArgs(); args != nil && args.Len() == 1 {
						coll.meta = yamlFields(args.At(0))
					}
				}
				for _, arg := range call.Args {
					if comp, ok := contentComponent(pkg, arg); ok {
						coll.comps[comp.name] = comp
					}
				}
				out = append(out, coll)
				return true
			})
		}
	}
	return out
}

// isGxFuncExpr is isGxFunc with generic instantiation unwrapped.
func isGxFuncExpr(pkg *packages.Package, fun ast.Expr, name string) bool {
	switch f := fun.(type) {
	case *ast.IndexExpr:
		return isGxFuncExpr(pkg, f.X, name)
	case *ast.IndexListExpr:
		return isGxFuncExpr(pkg, f.X, name)
	default:
		return isGxFunc(pkg, fun, name)
	}
}

// contentComponent reads one Components argument: the name and the props of
// its function signature.
func contentComponent(pkg *packages.Package, expr ast.Expr) (contentComp, bool) {
	name := ""
	switch e := expr.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	default:
		return contentComp{}, false
	}
	if name == "" || !isUpperByte(name[0]) {
		return contentComp{}, false
	}
	comp := contentComp{name: name, props: map[string]Prop{}}
	t := pkg.TypesInfo.TypeOf(expr)
	sig, _ := t.Underlying().(*types.Signature)
	if sig == nil || sig.Params().Len() == 0 {
		return comp, true
	}
	param := sig.Params().At(0).Type()
	if named, ok := param.(*types.Named); ok {
		param = named.Underlying()
	}
	st, ok := param.(*types.Struct)
	if !ok {
		return comp, true
	}
	qual := func(p *types.Package) string { return p.Name() }
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		comp.props[lowerFirst(f.Name())] = Prop{Name: f.Name(), Type: types.TypeString(f.Type(), qual)}
	}
	return comp, true
}

// checkContent reports the component findings of every Markdown file in
// every collection: an undeclared component is GX8002, an unknown prop is
// GX2003 (REQ-CNT-03). read is nil for the on-disk view.
func checkContent(root string, colls []contentCollection, read func(string) ([]byte, error)) []Diagnostic {
	var out []Diagnostic
	for _, coll := range colls {
		for _, path := range markdownFiles(coll.dir) {
			var src []byte
			var err error
			if read != nil {
				src, err = read(path)
			} else {
				src, err = os.ReadFile(path)
			}
			if err != nil {
				continue
			}
			out = checkFrontmatter(path, src, coll.meta, out)
			for _, tag := range scanMarkdownTags(string(src)) {
				comp, ok := coll.comps[tag.name]
				if !ok {
					names := make([]string, 0, len(coll.comps))
					for name := range coll.comps {
						names = append(names, name)
					}
					sort.Strings(names)
					msg := "component " + Quoted(tag.name) + " is not declared in this collection"
					if len(names) > 0 {
						msg += "; declared: " + strings.Join(names, ", ")
					}
					out = append(out, Diagnostic{
						Code: CodeContentComponent,
						File: path,
						Line: tag.pos.Line,
						Col:  tag.pos.Col,
						Msg:  msg,
						Fix:  "add it to the gx.Collection Components list",
					})
					continue
				}
				for _, attr := range tag.attrs {
					if _, ok := comp.props[attr.name]; ok {
						continue
					}
					out = append(out, Diagnostic{
						Code: CodeUnknownAttr,
						File: path,
						Line: attr.pos.Line,
						Col:  attr.pos.Col,
						Msg:  "unknown attribute " + Quoted(attr.name) + " on <" + tag.name + ">",
					})
				}
			}
		}
	}
	return out
}

// markdownFiles lists the .md files under dir.
func markdownFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// mdTag is one component tag of a Markdown file.
type mdTag struct {
	name  string
	pos   Pos
	attrs []mdAttr
}

// mdAttr is one attribute of a component tag.
type mdAttr struct {
	name string
	pos  Pos
}

// scanMarkdownTags finds every component tag in Markdown text. Frontmatter,
// fenced code blocks, inline code spans and HTML comments are skipped.
func scanMarkdownTags(src string) []mdTag {
	var tags []mdTag
	lines := strings.Split(src, "\n")
	inFence := false
	fence := ""
	inComment := false
	frontmatter := false
	for i, line := range lines {
		if i == 0 && strings.TrimSpace(line) == "---" {
			frontmatter = true
			continue
		}
		if frontmatter {
			if strings.TrimSpace(line) == "---" {
				frontmatter = false
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if inFence {
			if strings.HasPrefix(trimmed, fence) {
				inFence = false
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = true
			fence = trimmed[:3]
			continue
		}
		scanMarkdownLine(line, i+1, &tags, &inComment)
	}
	return tags
}

// scanMarkdownLine scans one line for component tags.
func scanMarkdownLine(line string, lineNo int, tags *[]mdTag, inComment *bool) {
	i := 0
	for i < len(line) {
		if *inComment {
			idx := strings.Index(line[i:], "-->")
			if idx < 0 {
				return
			}
			i += idx + 3
			*inComment = false
			continue
		}
		if strings.HasPrefix(line[i:], "<!--") {
			*inComment = true
			i += 4
			continue
		}
		if line[i] == '`' {
			idx := strings.IndexByte(line[i+1:], '`')
			if idx < 0 {
				return
			}
			i += idx + 2
			continue
		}
		if line[i] != '<' {
			i++
			continue
		}
		if i+1 < len(line) && line[i+1] == '/' {
			idx := strings.IndexByte(line[i:], '>')
			if idx < 0 {
				return
			}
			i += idx + 1
			continue
		}
		if i+1 >= len(line) || !isMdTagStart(line[i+1]) {
			i++
			continue
		}
		j := i + 1
		for j < len(line) && isMdTagName(line[j]) {
			j++
		}
		name := line[i+1 : j]
		base := name
		if k := strings.LastIndexByte(base, '.'); k >= 0 {
			base = base[k+1:]
		}
		if base == "" || !isUpperByte(base[0]) {
			i = j
			continue
		}
		tag := mdTag{name: base, pos: Pos{Line: lineNo, Col: i + 1}}
		k := j
		closed := false
		for k < len(line) {
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k < len(line) && (line[k] == '>' || line[k] == '/') {
				closed = true
				break
			}
			if k >= len(line) || line[k] == '{' {
				closed = true
				break
			}
			start := k
			for k < len(line) && !strings.ContainsRune(" \t=>/'", rune(line[k])) {
				k++
			}
			attr := line[start:k]
			if attr == "" {
				k++
				continue
			}
			tag.attrs = append(tag.attrs, mdAttr{name: attr, pos: Pos{Line: lineNo, Col: start + 1}})
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k < len(line) && line[k] == '=' {
				k++
				for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
					k++
				}
				k = skipMarkdownValue(line, k)
			}
		}
		if closed {
			*tags = append(*tags, tag)
		}
		i = k
	}
}

// skipMarkdownValue skips one attribute value: a balanced brace expression,
// a quoted string or a bare word.
func skipMarkdownValue(line string, i int) int {
	if i >= len(line) {
		return i
	}
	switch line[i] {
	case '{':
		depth := 0
		for i < len(line) {
			switch line[i] {
			case '"', '\'':
				q := line[i]
				i++
				for i < len(line) && line[i] != q {
					if line[i] == '\\' {
						i++
					}
					i++
				}
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
			i++
		}
		return i
	case '"', '\'':
		q := line[i]
		i++
		for i < len(line) && line[i] != q {
			if line[i] == '\\' {
				i++
			}
			i++
		}
		if i < len(line) {
			i++
		}
		return i
	default:
		for i < len(line) && !strings.ContainsRune(" \t>", rune(line[i])) {
			i++
		}
		return i
	}
}

func isMdTagStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isMdTagName(c byte) bool {
	return isMdTagStart(c) || (c >= '0' && c <= '9') || c == '.' || c == '-'
}

func isUpperByte(c byte) bool { return c >= 'A' && c <= 'Z' }
