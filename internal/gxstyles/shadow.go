package gxstyles

import (
	"bytes"
	"errors"
	"regexp"
)

// A Tailwind build is made for a document. Three things of it do not work
// in the shadow root of a widget (REQ-ISL-11):
//
//   - A browser does not read @property in a shadow root. Tailwind gives its
//     internal variables (--tw-shadow and the rest) their start values with
//     @property, and has the same values in a block for old browsers. That
//     block must hold with no condition.
//   - The theme has its tokens on :root, and :root matches nothing in a
//     shadow root. The tokens go on the host element, where a rule of the
//     host page for the element can give a token a different value.
//   - Dark mode follows the class "dark" of an ancestor. In a shadow root
//     the class is on the host element.
//
// ShadowCSS reads a minified build of the pinned Tailwind version.

var (
	rootRule    = regexp.MustCompile(`(^|[{}]):root\{`)
	rootNotRule = regexp.MustCompile(`(^|[{}]):root:not\(([^()]*)\)\{`)
	darkRule    = regexp.MustCompile(`(^|[{}])\.dark\{`)
)

const (
	propertiesStart = "@layer properties{@supports "
	darkVariant     = ":where(.dark,.dark *)"
	darkVariantHost = ":where(.dark,.dark *,:host(.dark) *)"
)

// ShadowCSS changes a minified Tailwind build so that it works in the shadow
// root of a widget.
func ShadowCSS(css []byte) ([]byte, error) {
	out, lifted := liftStartValues(css)
	if !lifted && bytes.Contains(css, []byte("@property ")) {
		return nil, errors.New("gxstyles: the Tailwind build has @property rules and no block of start values; a widget then has no shadows, rings or transforms. The pinned Tailwind version changed its output: report this to Gx")
	}
	out = rootNotRule.ReplaceAll(out, []byte("${1}:host(:not(${2})){"))
	out = rootRule.ReplaceAll(out, []byte("${1}:host{"))
	out = darkRule.ReplaceAll(out, []byte("${1}:host(.dark){"))
	out = bytes.ReplaceAll(out, []byte(darkVariant), []byte(darkVariantHost))
	return out, nil
}

// liftStartValues takes the start values of the Tailwind variables out of
// their @supports condition: "@layer properties{@supports (...){BODY}}"
// becomes "@layer properties{BODY}".
func liftStartValues(css []byte) ([]byte, bool) {
	start := bytes.Index(css, []byte(propertiesStart))
	if start < 0 {
		return css, false
	}
	// The condition has parentheses and no brace. Its block starts at the
	// first brace after it.
	cond := start + len(propertiesStart)
	open := bytes.IndexByte(css[cond:], '{')
	if open < 0 {
		return css, false
	}
	open += cond
	depth, end := 0, -1
	for i := open; i < len(css); i++ {
		switch css[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return css, false
	}
	out := make([]byte, 0, len(css))
	out = append(out, css[:start]...)
	out = append(out, "@layer properties{"...)
	out = append(out, css[open+1:end]...)
	out = append(out, css[end+1:]...)
	return out, true
}

var (
	tailwindImport = regexp.MustCompile(`@import\s+["']tailwindcss["']\s*;`)
	classesSource  = regexp.MustCompile(`@source\s+["'][^"']*\.gx/classes\.txt["']\s*;`)
)

// widgetTheme returns the theme input of one widget: the theme of the app,
// with the class list of the widget as the only source of classes.
func widgetTheme(theme []byte, classList string) ([]byte, error) {
	if !tailwindImport.Match(theme) {
		return nil, errors.New(`gxstyles: app/theme.css has no line @import "tailwindcss"; the stylesheet of a widget needs it to turn off the search for sources`)
	}
	// Without source(none), Tailwind reads each file of the app, and the
	// stylesheet of the widget holds each class of the app.
	out := tailwindImport.ReplaceAll(theme, []byte(`@import "tailwindcss" source(none);`))
	source := []byte(`@source "` + classList + `";`)
	if classesSource.Match(out) {
		return classesSource.ReplaceAllLiteral(out, source), nil
	}
	return append(append(out, '\n'), append(source, '\n')...), nil
}
