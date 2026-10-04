// Package content renders Markdown at build time: CommonMark plus the GFM
// extensions, stable heading ids and anchor links, and the GFM disallowed
// raw HTML filter (REQ-CNT-01). Only repository files are rendered (SI-12).
package content

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Options configure one render.
type Options struct {
	// GFM enables tables, task lists, strikethrough and autolinks, and the
	// GFM disallowed raw HTML filter (REQ-CNT-01).
	GFM bool
	// Footnotes enables the footnote extension (REQ-CNT-01).
	Footnotes bool
	// Anchors adds a stable id and an anchor link to every heading
	// (REQ-CNT-01).
	Anchors bool
	// XHTML renders void elements in the XHTML form the CommonMark spec
	// uses (<hr />). Content pages use the HTML5 form.
	XHTML bool
}

// Render renders Markdown to HTML.
func Render(src []byte, opts Options) ([]byte, error) {
	var exts []goldmark.Extender
	if opts.GFM {
		exts = append(exts,
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.TaskList,
			extension.Strikethrough,
			extension.Linkify,
		)
	}
	if opts.Footnotes {
		exts = append(exts, extension.Footnote)
	}
	parserOpts := []parser.Option{}
	if opts.Anchors {
		parserOpts = append(parserOpts, parser.WithAutoHeadingID())
	}
	renderOpts := []renderer.Option{
		html.WithUnsafe(),
		renderer.WithNodeRenderers(util.Prioritized(&nodeRenderer{opts: opts}, 500)),
	}
	if opts.XHTML {
		renderOpts = append(renderOpts, html.WithXHTML())
	}
	md := goldmark.New(
		goldmark.WithExtensions(exts...),
		goldmark.WithParserOptions(parserOpts...),
		goldmark.WithRendererOptions(renderOpts...),
	)
	var out bytes.Buffer
	if err := md.Convert(src, &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// nodeRenderer adds the heading anchors and the GFM disallowed raw HTML
// filter on top of the default renderer.
type nodeRenderer struct {
	opts Options
}

func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	if r.opts.Anchors {
		reg.Register(ast.KindHeading, r.renderHeading)
	}
	if r.opts.GFM {
		reg.Register(ast.KindRawHTML, r.renderRawHTML)
		reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
	}
}

// renderHeading is the default heading renderer plus an anchor link.
func (r *nodeRenderer) renderHeading(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	if entering {
		_, _ = w.WriteString("<h")
		_ = w.WriteByte("0123456"[n.Level])
		if n.Attributes() != nil {
			html.RenderAttributes(w, node, html.HeadingAttributeFilter)
		}
		_ = w.WriteByte('>')
		return ast.WalkContinue, nil
	}
	if id, ok := n.AttributeString("id"); ok {
		if b, ok := id.([]byte); ok && len(b) > 0 {
			_, _ = fmt.Fprintf(w, `<a class="anchor" href="#%s" aria-hidden="true">#</a>`, util.EscapeHTML(b))
		}
	}
	_, _ = w.WriteString("</h")
	_ = w.WriteByte("0123456"[n.Level])
	_, _ = w.WriteString(">\n")
	return ast.WalkContinue, nil
}

// disallowedTags finds the GFM disallowed raw HTML tags.
var disallowedTags = regexp.MustCompile(`(?i)<(/?)(title|textarea|style|xmp|iframe|noembed|noframes|script|plaintext)\b`)

// renderRawHTML writes raw HTML with the GFM disallowed tags escaped.
func (r *nodeRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	for i := 0; i < n.Segments.Len(); i++ {
		segment := n.Segments.At(i)
		_, _ = w.Write(escapeDisallowed(segment.Value(source)))
	}
	return ast.WalkSkipChildren, nil
}

// renderHTMLBlock writes an HTML block with the GFM disallowed tags
// escaped.
func (r *nodeRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if entering {
		for i := 0; i < n.Lines().Len(); i++ {
			line := n.Lines().At(i)
			_, _ = w.Write(escapeDisallowed(line.Value(source)))
		}
		return ast.WalkContinue, nil
	}
	if n.HasClosure() {
		_, _ = w.Write(escapeDisallowed(n.ClosureLine.Value(source)))
	}
	return ast.WalkContinue, nil
}

// escapeDisallowed escapes the opening angle bracket of every disallowed
// tag in a raw HTML segment.
func escapeDisallowed(b []byte) []byte {
	s := string(b)
	if !strings.Contains(s, "<") {
		return b
	}
	out := disallowedTags.ReplaceAllStringFunc(s, func(match string) string {
		return "&lt;" + match[1:]
	})
	return []byte(out)
}
