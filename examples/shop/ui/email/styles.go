// Package email holds the parts of an HTML email: inline styles and table
// layout, which each email client reads. A component of this package has no
// class, no signal and no script (GX6010). The styles read the theme tokens
// with var(); gx.RenderEmail gives each one its value.
package email

import (
	"strconv"

	"github.com/alternayte/gx"
)

// Align is the horizontal position of the content of a part.
type Align string

// The positions of the content.
const (
	Left   Align = "left"
	Center Align = "center"
	Right  Align = "right"
)

// attr returns the value of the align attribute; a zero value is Left.
func (a Align) attr() string {
	switch a {
	case Center, Right:
		return string(a)
	}
	return string(Left)
}

// font is the font of each text. An email client has no font of the site.
const font = "font-family:Helvetica,Arial,sans-serif;"

// width returns the width of the content in pixels; a zero value is 600.
func (p DocumentProps) width() string {
	if p.Width <= 0 {
		return "600"
	}
	return strconv.Itoa(p.Width)
}

// style returns the style of the content table.
func (p DocumentProps) style() gx.Style {
	return gx.Style("width:100%;max-width:" + p.width() + "px")
}

// style returns the style of the cell of a section.
func (p SectionProps) style() gx.Style {
	if p.Card {
		return "padding:24px;background-color:var(--card);border:1px solid var(--border);border-radius:var(--radius)"
	}
	return "padding:12px 0"
}

// style returns the style of a column. An empty width shares the row.
func (p ColumnProps) style() gx.Style {
	s := "padding:0 8px 0 0;vertical-align:top;" + font + "font-size:16px;line-height:24px;color:var(--foreground)"
	if p.Width > 0 {
		s += ";width:" + strconv.Itoa(p.Width) + "px"
	}
	return gx.Style(s)
}

// style returns the style of a paragraph.
func (p TextProps) style() gx.Style {
	s := "margin:0 0 16px;" + font + "font-size:16px;line-height:24px;text-align:" + p.Align.attr()
	if p.Muted {
		return gx.Style(s + ";font-size:14px;line-height:20px;color:var(--muted-foreground)")
	}
	return gx.Style(s + ";color:var(--foreground)")
}

// level returns the level of a heading: 1, 2 or 3. A zero value is 1.
func (p HeadingProps) level() int {
	if p.Level < 1 {
		return 1
	}
	if p.Level > 3 {
		return 3
	}
	return p.Level
}

// style returns the style of a heading.
func (p HeadingProps) style() gx.Style {
	size := map[int]string{1: "font-size:28px;line-height:36px", 2: "font-size:22px;line-height:28px", 3: "font-size:18px;line-height:24px"}[p.level()]
	return gx.Style("margin:0 0 16px;" + font + size + ";font-weight:700;color:var(--foreground);text-align:" + p.Align.attr())
}

// size returns a pixel count as an attribute value.
func size(n int) string { return strconv.Itoa(n) }

// style returns the style of an image. The image is not wider than its
// column.
func (p ImageProps) style() gx.Style {
	return "display:block;border:0;outline:none;max-width:100%;height:auto"
}
