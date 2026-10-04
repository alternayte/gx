package compiler

import (
	"go/types"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlFields maps the frontmatter field names of a Meta type to the YAML
// kind it accepts (REQ-CNT-02).
func yamlFields(t types.Type) map[string]string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	st, ok := t.(*types.Struct)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Exported() {
			continue
		}
		name := strings.Split(reflect.StructTag(st.Tag(i)).Get("yaml"), ",")[0]
		if name == "" || name == "-" {
			name = strings.ToLower(f.Name())
		}
		out[name] = yamlKindOf(f.Type())
	}
	return out
}

// yamlKindOf returns the YAML tag a Go type accepts.
func yamlKindOf(t types.Type) string {
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	switch u := t.(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsBoolean != 0:
			return "!!bool"
		case u.Info()&types.IsInteger != 0:
			return "!!int"
		case u.Info()&types.IsFloat != 0:
			return "!!float"
		case u.Info()&types.IsString != 0:
			return "!!str"
		}
	case *types.Slice, *types.Array:
		return "!!seq"
	case *types.Map:
		return "!!map"
	}
	return ""
}

var yamlErrLine = regexp.MustCompile(`line (\d+):`)

// frontmatterOf returns the frontmatter bytes and the file line of its first
// content line, or nil when the file has no frontmatter.
func frontmatterOf(data []byte) ([]byte, int) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, 0
	}
	nl := strings.IndexByte(s, '\n')
	if nl < 0 {
		return nil, 0
	}
	rest := s[nl+1:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, 0
	}
	return []byte(rest[:idx]), 2 // content line 1 is file line 2
}

// checkFrontmatter reports malformed frontmatter of one content file:
// a YAML syntax error, an unknown field or a wrong value kind (GX8001,
// REQ-CNT-02).
func checkFrontmatter(path string, data []byte, meta map[string]string, diags []Diagnostic) []Diagnostic {
	front, firstLine := frontmatterOf(data)
	if front == nil {
		return diags
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(front, &doc); err != nil {
		line := firstLine
		if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
			if n, convErr := strconv.Atoi(m[1]); convErr == nil {
				line = firstLine + n - 1
			}
		}
		return append(diags, Diagnostic{
			Code: CodeContentFrontmatter,
			File: path,
			Line: line,
			Col:  1,
			Msg:  "frontmatter: " + err.Error(),
		})
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode || meta == nil {
		return diags
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		line := firstLine + key.Line - 1
		kind, ok := meta[key.Value]
		if !ok {
			diags = append(diags, Diagnostic{
				Code: CodeContentFrontmatter,
				File: path,
				Line: line,
				Col:  1,
				Msg:  "frontmatter: unknown field " + Quoted(key.Value),
				Fix:  "remove it or add the field to the Meta type",
			})
			continue
		}
		if kind != "" && value.Tag != "" && value.Tag != kind {
			diags = append(diags, Diagnostic{
				Code: CodeContentFrontmatter,
				File: path,
				Line: firstLine + value.Line - 1,
				Col:  1,
				Msg:  "frontmatter: " + Quoted(key.Value) + " needs " + kind + ", got " + value.Tag,
			})
		}
	}
	return diags
}
