// Package content renders Markdown bodies for content pages (REQ-CNT-02)
// and code frames for the docs kit (REQ-CNT-05). It wraps the internal
// renderers, so package gx keeps no Markdown or chroma dependency
// (SDD §12.4). Content comes from repository files (SI-12).
package content

import (
	"github.com/alternayte/gx"
	gxcontent "github.com/alternayte/gx/internal/content"
	"github.com/alternayte/gx/internal/highlight"
	"gopkg.in/yaml.v3"
)

// Install installs the YAML frontmatter decoder and the body renderer of
// content pages (REQ-CNT-02). The app's main calls it once:
//
//	content.Install()
func Install() {
	gx.SetFrontmatterDecoder(func(data []byte, v any) error { return yaml.Unmarshal(data, v) })
}

// Body renders the Markdown body of a content entry: CommonMark plus GFM,
// footnotes and heading anchors.
func Body(markdown []byte) (gx.Node, error) {
	html, err := gxcontent.Render(markdown, gxcontent.Options{GFM: true, Footnotes: true, Anchors: true, Highlight: true})
	if err != nil {
		return nil, err
	}
	return gx.Raw(gx.SafeHTML(string(html))), nil //gx:trusted content is a repository file (SI-12)
}

// CodeOptions are the frame options of a code block (REQ-CNT-05).
type CodeOptions struct {
	// Title sits in the frame bar.
	Title string
	// Wrap wraps long lines.
	Wrap bool
	// Marks, Ins, Del and Words mark 1-based lines of the shown source.
	Marks []int
	Ins   []int
	Del   []int
	Words []int
}

// Code renders a resolved code block (REQ-CNT-05): the compiler fills
// gx.Code at build time from a repository file, and chroma highlights it
// here.
func Code(c gx.Code, opt CodeOptions) gx.Node {
	lang := c.Lang
	if lang == "" {
		lang = highlight.LangForFile(c.File)
	}
	info := lang
	if opt.Title != "" {
		info += ` title="` + opt.Title + `"`
	}
	for _, m := range opt.Marks {
		info += " {" + itoa(m) + "}"
	}
	for _, n := range opt.Ins {
		info += " ins={" + itoa(n) + "}"
	}
	for _, n := range opt.Del {
		info += " del={" + itoa(n) + "}"
	}
	for _, n := range opt.Words {
		info += " word={" + itoa(n) + "}"
	}
	if opt.Wrap {
		info += " wrap"
	}
	html := highlight.RenderCode(lang, info, c.Source)
	return gx.Raw(gx.SafeHTML(html)) //gx:trusted highlighted source is a repository file (SI-12)
}

// CodeCSS returns the light and dark stylesheet of code frames
// (REQ-CNT-04).
func CodeCSS() string { return highlight.CodeCSS() }

// itoa renders a positive integer for a fence info string.
func itoa(n int) string {
	if n <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Heading is one heading of a content page (REQ-CNT-06).
type Heading = gxcontent.Heading

// Headings returns the rendered headings of a Markdown body, so the docs
// shell can build its table of contents (REQ-CNT-06).
func Headings(markdown []byte) []Heading { return gxcontent.Headings(markdown) }
