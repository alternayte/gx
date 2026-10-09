// Package content renders Markdown bodies for content pages (REQ-CNT-02)
// and code frames for the docs kit (REQ-CNT-05). It wraps the internal
// renderers, so package gx keeps no Markdown or chroma dependency
// (SDD §12.4). Content comes from repository files (SI-12).
package content

import (
	"regexp"
	"strconv"

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

// slotMarker finds the place of one slot in rendered Markdown. The
// compiler writes the two forms: a comment for a component with its lines
// to itself, an element for a component inside a line.
var slotMarker = regexp.MustCompile(`<!--gx-slot:(\d+)-->|<gx-slot n="(\d+)"></gx-slot>`)

// BodySlots renders a Markdown body that holds components (REQ-CNT-03).
// The Markdown has one marker for each slot, and the result has slots[n]
// in the place of marker n. The generated body function of a collection
// calls it; the prose around the components of one parent is one document,
// so a component in a list item stays in the list item.
func BodySlots(markdown []byte, slots ...gx.Node) (gx.Node, error) {
	html, err := gxcontent.Render(markdown, gxcontent.Options{GFM: true, Footnotes: true, Anchors: true, Highlight: true})
	if err != nil {
		return nil, err
	}
	// A component that is the only content of a paragraph is a block.
	text := slotParagraph.ReplaceAllString(string(html), "$1")
	var parts []gx.Node
	last := 0
	for _, m := range slotMarker.FindAllStringSubmatchIndex(text, -1) {
		parts = append(parts, gx.Raw(gx.SafeHTML(text[last:m[0]]))) //gx:trusted content is a repository file (SI-12)
		lo, hi := m[2], m[3]
		if lo < 0 {
			lo, hi = m[4], m[5]
		}
		digits := text[lo:hi]
		if n, err := strconv.Atoi(digits); err == nil && n < len(slots) {
			parts = append(parts, slots[n])
		}
		last = m[1]
	}
	parts = append(parts, gx.Raw(gx.SafeHTML(text[last:]))) //gx:trusted content is a repository file (SI-12)
	return gx.Frag(parts...), nil
}

// slotParagraph finds a paragraph whose only content is one inline slot
// marker.
var slotParagraph = regexp.MustCompile(`<p>(<gx-slot n="\d+"></gx-slot>)</p>`)

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
