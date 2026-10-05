// Package stelint checks prose against the rules of ASD-STE100 that a
// program can decide (NFR-10): short sentences, short paragraphs, no
// contraction, no future tense, the active voice in a procedure step, and
// no word from the list of words that Simplified Technical English does
// not approve.
//
// The standard asks for the active voice in a procedure and "as much as
// possible" in a description. A program cannot decide the second part, so
// the passive rule reads only the steps of a numbered list.
package stelint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The limits of ASD-STE100: 25 words for a sentence that describes, and 6
// sentences for a paragraph.
const (
	MaxWords     = 25
	MaxSentences = 6
)

// Finding is one place where the text breaks a rule.
type Finding struct {
	File string
	Line int
	Rule string
	Text string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Text)
}

// Word is one word or phrase that the text must not use, with the words to
// use in its place.
type Word struct {
	Bad string
	Use string
}

// ParseWords reads a word list: one "bad = use this" line for each entry. A
// line that starts with # is a comment.
func ParseWords(text string) []Word {
	var out []Word
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bad, use, _ := strings.Cut(line, "=")
		out = append(out, Word{Bad: strings.ToLower(strings.TrimSpace(bad)), Use: strings.TrimSpace(use)})
	}
	return out
}

var (
	codeSpan    = regexp.MustCompile("`[^`]*`")
	linkText    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTag     = regexp.MustCompile(`<[^>]+>`)
	emphasis    = regexp.MustCompile(`\*\*|__|\*`)
	stepMarker  = regexp.MustCompile(`^\s*\d+\.\s+`)
	listMarker  = regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s+`)
	contraction = regexp.MustCompile(`(?i)\b\w+(?:n't|'re|'ll|'ve|'d)\b|\b(?:it|that|there|here|what|let)'s\b`)
	future      = regexp.MustCompile(`(?i)\b(?:will|shall|won't)\b`)
	// passive finds a form of "be" before a past participle: "is written",
	// "are generated", "was not found".
	passive = regexp.MustCompile(`(?i)\b(?:is|are|was|were|be|been|being)\s+(?:not\s+|also\s+|then\s+|only\s+|never\s+|always\s+)?(\w+ed|written|given|shown|built|sent|made|kept|held|found|run|read|set|put|taken|known|done|seen|drawn|chosen|hidden|broken|thrown|lost)\b`)
	wordRe  = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9'_./-]*`)
)

// states are adjectives that end like a past participle. "The prop is
// required" and "the checkbox is checked" state a property and name no
// action, so they are not the passive voice.
var states = map[string]bool{
	"required": true, "typed": true, "needed": true, "allowed": true, "supported": true,
	"based": true, "named": true, "nested": true, "embedded": true, "related": true,
	"limited": true, "pinned": true, "finished": true, "scoped": true, "keyed": true,
	"installed": true, "mounted": true, "committed": true, "formatted": true, "sorted": true,
	"exported": true, "deprecated": true,
	// The states of a control.
	"checked": true, "closed": true, "disabled": true, "selected": true, "pressed": true,
	"focused": true, "expanded": true, "collapsed": true, "hidden": true,
}

// terms are fixed names that the text can use as they are. "Don't" is the
// name of a section of each registry USAGE.md file.
var terms = map[string]bool{"Don't": true}

// Lint checks one Markdown file. It skips the frontmatter keys other than
// the title and the description, code blocks, code spans and HTML tags.
func Lint(file, text string, words []Word) []Finding {
	var out []Finding
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	report := func(line int, rule, text string) {
		out = append(out, Finding{File: file, Line: line, Rule: rule, Text: text})
	}
	checkText := func(line int, text string, sentences, step bool) int {
		plain := clean(text)
		lower := strings.ToLower(plain)
		for _, w := range words {
			if at := findWord(lower, w.Bad); at >= 0 {
				report(line, "word", fmt.Sprintf("%q is not approved; use %s", w.Bad, w.Use))
			}
		}
		if m := contraction.FindString(plain); m != "" {
			report(line, "contraction", fmt.Sprintf("%q; write the two words", m))
		}
		if m := future.FindString(plain); m != "" {
			report(line, "tense", fmt.Sprintf("%q; use the present tense", m))
		}
		for _, m := range passive.FindAllStringSubmatch(plain, -1) {
			if step && !states[strings.ToLower(m[1])] {
				report(line, "passive", fmt.Sprintf("%q; name who does the action", m[0]))
			}
		}
		count := 0
		for _, sentence := range splitSentences(plain) {
			count++
			if n := len(wordRe.FindAllString(sentence, -1)); sentences && n > MaxWords {
				report(line, "length", fmt.Sprintf("a sentence of %d words; the limit is %d: %s", n, MaxWords, short(sentence)))
			}
		}
		return count
	}

	inFront, inCode := false, false
	fence := ""
	var para []string
	paraLine := 0
	paraStep := false
	flush := func() {
		if len(para) == 0 {
			return
		}
		if n := checkText(paraLine, strings.Join(para, " "), true, paraStep); n > MaxSentences {
			report(paraLine, "paragraph", fmt.Sprintf("a paragraph of %d sentences; the limit is %d", n, MaxSentences))
		}
		para = nil
	}
	for i, line := range lines {
		n := i + 1
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFront = true
			continue
		}
		if inFront {
			if trimmed == "---" {
				inFront = false
				continue
			}
			key, value, _ := strings.Cut(trimmed, ":")
			if key == "title" || key == "description" {
				checkText(n, strings.Trim(strings.TrimSpace(value), `"`), true, false)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inCode {
				flush()
				inCode, fence = true, marker
			} else if marker == fence {
				inCode = false
			}
			continue
		}
		if inCode {
			continue
		}
		switch {
		case trimmed == "":
			flush()
		case strings.HasPrefix(trimmed, "#"):
			flush()
			if heading := strings.TrimLeft(trimmed, "# "); !terms[heading] {
				checkText(n, heading, false, false)
			}
		case strings.HasPrefix(trimmed, "|"):
			flush()
			if strings.Trim(trimmed, "|-: ") == "" {
				continue
			}
			for _, cell := range strings.Split(strings.Trim(trimmed, "|"), "|") {
				checkText(n, cell, true, false)
			}
		case listMarker.MatchString(line):
			flush()
			para, paraLine = []string{listMarker.ReplaceAllString(line, "")}, n
			paraStep = stepMarker.MatchString(line)
		case strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") && !strings.Contains(trimmed, " ") && len(para) == 0:
			// A component tag alone on a line.
		default:
			if len(para) == 0 {
				paraLine, paraStep = n, false
			}
			para = append(para, trimmed)
		}
	}
	flush()
	return out
}

// clean removes what is not prose: code spans, link targets, tags and
// emphasis marks.
func clean(text string) string {
	text = codeSpan.ReplaceAllString(text, "CODE")
	text = linkText.ReplaceAllString(text, "$1")
	text = htmlTag.ReplaceAllString(text, " ")
	text = emphasis.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// findWord finds a word or phrase at word boundaries.
func findWord(lower, bad string) int {
	from := 0
	for {
		at := strings.Index(lower[from:], bad)
		if at < 0 {
			return -1
		}
		at += from
		before := at == 0 || !isWordByte(lower[at-1])
		after := at+len(bad) == len(lower) || !isWordByte(lower[at+len(bad)])
		if before && after {
			return at
		}
		from = at + 1
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c == '\'' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// splitSentences splits prose at a full stop, a question mark, an
// exclamation mark or a colon that ends a sentence.
func splitSentences(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '.' && c != '?' && c != '!' && c != ':' && c != ';' {
			continue
		}
		if i+1 < len(text) && text[i+1] != ' ' {
			continue // inside a number, a file name or an address
		}
		if c == '.' && i > 0 && text[i-1] == '.' {
			continue
		}
		if s := strings.TrimSpace(text[start : i+1]); len(wordRe.FindAllString(s, -1)) > 0 {
			out = append(out, s)
		}
		start = i + 1
	}
	if s := strings.TrimSpace(text[start:]); len(wordRe.FindAllString(s, -1)) > 0 {
		out = append(out, s)
	}
	return out
}

func short(s string) string {
	if len(s) > 70 {
		return s[:70] + "..."
	}
	return s
}

// LintPaths checks each Markdown file of the paths. A path is a file or a
// directory. It returns the findings in file order and the number of files.
func LintPaths(paths []string, words []Word) ([]Finding, int, error) {
	var out []Finding
	files := 0
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			out = append(out, Lint(filepath.ToSlash(path), string(raw), words)...)
			return nil
		})
		if err != nil {
			return nil, 0, err
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, files, nil
}
