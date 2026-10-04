package gx

import (
	"regexp"
	"strings"
)

// LLMSOptions configure the llms.txt export of one content route
// (REQ-CNT-08).
type LLMSOptions[Meta any] struct {
	// Site is the H1 of llms.txt.
	Site string
	// Summary is the blockquote line of llms.txt.
	Summary string
	// Title returns the entry title.
	Title func(Meta) string
	// Description returns the entry description.
	Description func(Meta) string
	// Skip keeps the entry out of the llms files when it returns true.
	Skip func(Meta) bool
}

// LLMSEntry is one page of the llms.txt export (REQ-CNT-08).
type LLMSEntry struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Skip        bool   `json:"skip"`
}

// LLMSManifest is the llms.txt data of one content route (REQ-CNT-08). The
// dev-only export manifest carries it to the exporter.
type LLMSManifest struct {
	Site    string      `json:"site"`
	Summary string      `json:"summary"`
	Entries []LLMSEntry `json:"entries"`
}

// llmsSkipPattern finds a <docs.LLMSkip> block or element in Markdown.
var llmsSkipPattern = regexp.MustCompile(`(?s)<docs\.LLMSkip\s*/>|<docs\.LLMSkip\s*>.*?</docs\.LLMSkip\s*>`)

// stripLLMSkip removes every <docs.LLMSkip> block from a Markdown body
// (REQ-CNT-08).
func stripLLMSkip(body string) string {
	return strings.TrimSpace(llmsSkipPattern.ReplaceAllString(body, ""))
}
