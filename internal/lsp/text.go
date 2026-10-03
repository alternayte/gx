package lsp

import (
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// position is an LSP position: zero-based line and UTF-16 character offset.
type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// rng is an LSP range.
type rng struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

// location is an LSP location.
type location struct {
	URI   string `json:"uri"`
	Range rng    `json:"range"`
}

// document is one open text buffer.
type document struct {
	Path    string
	Text    string
	Version int
}

// line returns the text of a zero-based line without its newline.
func (d *document) line(n int) string {
	if n < 0 {
		return ""
	}
	start := 0
	for i := 0; i < n; i++ {
		idx := strings.IndexByte(d.Text[start:], '\n')
		if idx < 0 {
			return ""
		}
		start += idx + 1
	}
	end := strings.IndexByte(d.Text[start:], '\n')
	if end < 0 {
		return d.Text[start:]
	}
	return d.Text[start : start+end]
}

// offsetAt converts an LSP position to a byte offset.
func (d *document) offsetAt(pos position) int {
	offset := 0
	for i := 0; i < pos.Line; i++ {
		idx := strings.IndexByte(d.Text[offset:], '\n')
		if idx < 0 {
			return len(d.Text)
		}
		offset += idx + 1
	}
	line := d.line(pos.Line)
	return offset + byteOffsetForUTF16(line, pos.Character)
}

// positionAt converts a byte offset to an LSP position.
func (d *document) positionAt(offset int) position {
	if offset > len(d.Text) {
		offset = len(d.Text)
	}
	if offset < 0 {
		offset = 0
	}
	line := strings.Count(d.Text[:offset], "\n")
	lineStart := strings.LastIndexByte(d.Text[:offset], '\n') + 1
	char := utf16Len(d.Text[lineStart:offset])
	return position{Line: line, Character: char}
}

// byteOffsetForUTF16 returns the byte offset in s after units UTF-16 code
// units. A position past the end clamps to the line.
func byteOffsetForUTF16(s string, units int) int {
	if units <= 0 {
		return 0
	}
	count := 0
	for i, r := range s {
		if count >= units {
			return i
		}
		if r > 0xFFFF {
			count += 2
		} else {
			count++
		}
	}
	return len(s)
}

// utf16Len returns the length of s in UTF-16 code units.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// pathToURI converts a file path to a file URI.
func pathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String()
}

// uriToPath converts a file URI to a file path.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	return filepath.FromSlash(u.Path)
}
