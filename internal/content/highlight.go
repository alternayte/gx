package content

import (
	"strings"

	"github.com/alternayte/gx/internal/highlight"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// codeRenderer replaces the default fenced code block renderer with the
// highlighted frame (REQ-CNT-04).
type codeRenderer struct{}

func (r *codeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFenced)
}

func (r *codeRenderer) renderFenced(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.FencedCodeBlock)
	lang := string(n.Language(source))
	info := ""
	if n.Info != nil {
		info = string(n.Info.Value(source))
	}
	var code strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		code.Write(line.Value(source))
	}
	_, _ = w.WriteString(RenderCode(lang, info, code.String()))
	return ast.WalkSkipChildren, nil
}

// CodeOptions are the per-block options of a highlighted code block
// (REQ-CNT-04).
type CodeOptions = highlight.Options

// RenderCode renders one highlighted code block (REQ-CNT-04).
func RenderCode(lang, info, code string) string {
	return highlight.RenderCode(lang, info, code)
}

// ParseCodeInfo parses the fence info string (REQ-CNT-04).
func ParseCodeInfo(lang, info string) CodeOptions {
	return highlight.ParseInfo(lang, info)
}

// CodeCSS returns the light and dark stylesheet of the highlighted code
// blocks (REQ-CNT-04).
func CodeCSS() string { return highlight.CodeCSS() }
