// Package content renders Markdown bodies for content pages (REQ-CNT-02).
// It wraps the compiler's Markdown renderer, so package gx keeps no Markdown
// dependency (SDD §12.4). Content comes from repository files (SI-12).
package content

import (
	"github.com/alternayte/gx"
	gxcontent "github.com/alternayte/gx/internal/content"
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
