package gx

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// EmailOptions are the options of RenderEmail.
type EmailOptions struct {
	// BaseURL is the address of the site, for example
	// "https://shop.example". Each link and each image address that is a
	// site path gets it, because an email has no site of its own.
	BaseURL string
	// Tokens are the theme tokens as values, by the name of the custom
	// property: "--primary" to "#171717". The generated functions
	// gxstyles.LightTokens and gxstyles.DarkTokens return them. A style
	// that reads a token with var() gets the value.
	Tokens map[string]string
	// Lang is the language of the document. Empty means "en".
	Lang string
	// Title is the title of the document.
	Title string
}

// EmailMessage is one rendered email: the HTML document and the same
// content as plain text. Give the two parts to the mail library of the
// app; Gx sends no email.
type EmailMessage struct {
	HTML string
	Text string
}

// RenderEmail renders n as an email (REQ-REG-16). The HTML is a full
// document: a node with no html element gets the document around it. A
// site path in a link or an image becomes an absolute URL from the base
// URL. A var() of a style becomes the value of its token.
//
// An email client runs no script and reads no stylesheet of the site. These
// constructs in the node are an error: a class attribute, a signal, a
// client expression, an action invocation, an island and a script element
// (REQ-REG-17).
func RenderEmail(n Node, opt EmailOptions) (EmailMessage, error) {
	base, err := url.Parse(opt.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return EmailMessage{}, fmt.Errorf("gx: RenderEmail needs an absolute BaseURL such as https://shop.example; it is %q", opt.BaseURL)
	}
	if n == nil {
		n = Frag()
	}
	needs := scanRuntimeNeeds(n)
	switch {
	case needs.island:
		return EmailMessage{}, emailError("an island")
	case needs.signals:
		return EmailMessage{}, emailError("a signal")
	case needs.adapter, needs.tool:
		return EmailMessage{}, emailError("a client expression or an action invocation")
	}
	var b bytes.Buffer
	if err := RenderNode(&b, n); err != nil {
		return EmailMessage{}, err
	}
	w := emailWriter{base: strings.TrimRight(opt.BaseURL, "/"), tokens: opt.Tokens}
	if err := w.run(b.String()); err != nil {
		return EmailMessage{}, err
	}
	doc := w.html.String()
	if !w.ownDocument {
		lang := opt.Lang
		if lang == "" {
			lang = "en"
		}
		doc = `<!DOCTYPE html><html lang="` + escapeAttr(lang) + `"><head><meta charset="utf-8">` +
			`<meta name="viewport" content="width=device-width, initial-scale=1">` +
			`<title>` + html.EscapeString(opt.Title) + `</title></head>` +
			`<body style="margin:0;padding:0">` + doc + `</body></html>`
	}
	return EmailMessage{HTML: doc, Text: w.plain()}, nil
}

// emailError is the error for a construct that an email cannot hold. The
// compiler gives GX6010 for the same construct in a component of an email
// package.
func emailError(what string) error {
	return errors.New("gx: the node of an email has " + what + "; an email client runs no script and reads no stylesheet of the site (GX6010)")
}

// emailVar is one var() of a style, with its fallback.
var emailVar = regexp.MustCompile(`var\(\s*(--[A-Za-z0-9_-]+)\s*(?:,([^()]*))?\)`)

// emailHidden finds a style that hides its element: the preview text of an
// inbox is not text of the email.
var emailHidden = regexp.MustCompile(`(?:^|;)\s*display\s*:\s*none`)

// emailBlocks are the elements that start a new line of the plain text.
// The value is the count of line breaks after the element.
var emailBlocks = map[string]int{
	"p": 2, "h1": 2, "h2": 2, "h3": 2, "h4": 2, "h5": 2, "h6": 2, "table": 2, "ul": 2, "ol": 2, "blockquote": 2,
	"div": 1, "tr": 1, "li": 1, "section": 1, "article": 1, "header": 1, "footer": 1, "address": 1, "pre": 1,
}

// emailURLAttrs are the attributes that hold an address.
var emailURLAttrs = map[string]bool{"href": true, "src": true, "background": true, "poster": true}

// emailWriter walks rendered HTML one time. It writes the HTML of the
// email and collects the plain text.
type emailWriter struct {
	base   string
	tokens map[string]string

	html        strings.Builder
	text        strings.Builder
	ownDocument bool
	// skip is the element whose content is no text of the email: the
	// head, and an element that the style hides. skipDepth counts the
	// open elements of that name inside it.
	skip      string
	skipDepth int
	// space is true when white space came after the last word.
	space bool
	// links holds the address of each open link.
	links []string
	// linkStart is the length of the text at the start of each open link.
	linkStart []int
}

// run rewrites src. The input is the output of the render, so each
// attribute value has double quotes.
func (w *emailWriter) run(src string) error {
	for i := 0; i < len(src); {
		lt := strings.IndexByte(src[i:], '<')
		if lt < 0 {
			w.chars(src[i:])
			break
		}
		w.chars(src[i : i+lt])
		i += lt
		switch {
		case strings.HasPrefix(src[i:], "<!--"):
			end := strings.Index(src[i:], "-->")
			if end < 0 {
				end = len(src) - i - 3
			}
			w.html.WriteString(src[i : i+end+3])
			i += end + 3
		case strings.HasPrefix(src[i:], "<!"), strings.HasPrefix(src[i:], "</"):
			end := strings.IndexByte(src[i:], '>')
			if end < 0 {
				end = len(src) - i - 1
			}
			tag := src[i : i+end+1]
			w.html.WriteString(tag)
			if strings.HasPrefix(tag, "</") {
				w.end(strings.ToLower(strings.TrimSpace(tag[2 : len(tag)-1])))
			}
			i += end + 1
		default:
			next, err := w.start(src, i)
			if err != nil {
				return err
			}
			i = next
		}
	}
	return nil
}

// nameEnd returns the end of a tag name or an attribute name.
func nameEnd(src string, i int) int {
	for i < len(src) && !strings.ContainsRune(" \t\n\r\f/>=", rune(src[i])) {
		i++
	}
	return i
}

// start writes one start tag from src[i], which is '<', and returns the
// index after it.
func (w *emailWriter) start(src string, i int) (int, error) {
	j := nameEnd(src, i+1)
	name := strings.ToLower(src[i+1 : j])
	if name == "" {
		// A '<' that starts no tag is text.
		w.html.WriteByte('<')
		w.text.WriteByte('<')
		return i + 1, nil
	}
	switch name {
	case "script":
		return 0, emailError("a script element")
	case islandElement:
		// The walk reads what the email holds, so it finds an island in
		// trusted raw HTML too.
		return 0, emailError("an island")
	}
	if name == "html" {
		w.ownDocument = true
	}
	w.html.WriteString(src[i:j])
	href, alt, hidden := "", "", false
	for {
		for j < len(src) && strings.ContainsRune(" \t\n\r\f/", rune(src[j])) {
			j++
		}
		if j >= len(src) || src[j] == '>' {
			break
		}
		k := nameEnd(src, j)
		if k == j {
			// A stray '='.
			j++
			continue
		}
		key := src[j:k]
		lower := strings.ToLower(key)
		j = k
		value, hasValue := "", false
		if j < len(src) && src[j] == '=' {
			hasValue = true
			j++
			if j < len(src) && (src[j] == '"' || src[j] == '\'') {
				end := strings.IndexByte(src[j+1:], src[j])
				if end < 0 {
					end = len(src) - j - 1
				}
				value = src[j+1 : j+1+end]
				j += end + 2
			} else {
				k := j
				for k < len(src) && !strings.ContainsRune(" \t\n\r\f>", rune(src[k])) {
					k++
				}
				value = src[j:k]
				j = k
			}
		}
		switch {
		case lower == "class":
			return 0, emailError("a class attribute on <" + name + ">")
		case lower == "data-signals" || lower == "data-bind" || strings.HasPrefix(lower, "data-signals:") || strings.HasPrefix(lower, "data-bind:"):
			return 0, emailError("a signal on <" + name + ">")
		case lower == "data-gx-module":
			return 0, emailError("an island")
		case adapterMarker(lower) || strings.HasPrefix(lower, "hx-"):
			return 0, emailError("a client expression or an action invocation on <" + name + ">")
		case strings.HasPrefix(lower, "data-gx-"):
			// A marker for the runtime of a page, such as the marker of
			// a typed link.
			continue
		case emailURLAttrs[lower]:
			if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
				value = w.base + value
			}
			if lower == "href" {
				href = html.UnescapeString(value)
			}
		case lower == "style":
			resolved, err := w.style(value, name)
			if err != nil {
				return 0, err
			}
			value = resolved
			hidden = emailHidden.MatchString(value)
		case lower == "alt":
			alt = html.UnescapeString(value)
		}
		w.html.WriteByte(' ')
		w.html.WriteString(key)
		if hasValue {
			w.html.WriteString(`="`)
			w.html.WriteString(strings.ReplaceAll(value, `"`, "&#34;"))
			w.html.WriteByte('"')
		}
	}
	w.html.WriteByte('>')
	if j < len(src) {
		j++
	}
	w.open(name, href, alt, hidden)
	if name == "style" {
		// The text of a style element is not HTML.
		end := strings.Index(strings.ToLower(src[j:]), "</style")
		if end < 0 {
			end = len(src) - j
		}
		w.html.WriteString(src[j : j+end])
		j += end
	}
	return j, nil
}

// style gives each var() of a style value the value of its token.
func (w *emailWriter) style(value, element string) (string, error) {
	var missing string
	out := emailVar.ReplaceAllStringFunc(value, func(m string) string {
		parts := emailVar.FindStringSubmatch(m)
		if v, ok := w.tokens[parts[1]]; ok {
			return v
		}
		if fallback := strings.TrimSpace(parts[2]); fallback != "" {
			return fallback
		}
		if missing == "" {
			missing = parts[1]
		}
		return m
	})
	if missing != "" {
		return "", fmt.Errorf("gx: the style of <%s> reads the token %s, and EmailOptions.Tokens has no value for it; give gxstyles.LightTokens()", element, missing)
	}
	if strings.Contains(out, "var(") {
		return "", fmt.Errorf("gx: the style of <%s> has a var() that RenderEmail cannot give a value: %s", element, html.UnescapeString(value))
	}
	return out, nil
}

// chars adds text between two tags to the HTML and to the plain text.
func (w *emailWriter) chars(s string) {
	w.html.WriteString(s)
	w.words(html.UnescapeString(s))
}

// words adds text to the plain text. White space is one space, and two
// texts with no white space between them stay together.
func (w *emailWriter) words(s string) {
	if w.skip != "" || s == "" {
		return
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		w.space = true
		return
	}
	lead := strings.TrimLeft(s, " \t\n\r\f") != s
	if t := w.text.String(); (w.space || lead) && t != "" && !strings.HasSuffix(t, "\n") && !strings.HasSuffix(t, " ") {
		w.text.WriteByte(' ')
	}
	w.text.WriteString(strings.Join(fields, " "))
	w.space = strings.TrimRight(s, " \t\n\r\f") != s
}

// breakLine ends the line of the plain text.
func (w *emailWriter) breakLine(n int) {
	w.space = false
	t := strings.TrimRight(w.text.String(), " ")
	if t == "" {
		return
	}
	have := len(t) - len(strings.TrimRight(t, "\n"))
	w.text.Reset()
	w.text.WriteString(t)
	for ; have < n; have++ {
		w.text.WriteByte('\n')
	}
}

func (w *emailWriter) open(name, href, alt string, hidden bool) {
	if w.skip != "" {
		if w.skip == name && !voidElements[name] {
			w.skipDepth++
		}
		return
	}
	if hidden && !voidElements[name] {
		w.skip = name
		return
	}
	switch name {
	case "head", "style", "title":
		w.skip = name
	case "br":
		w.text.WriteByte('\n')
		w.space = false
	case "hr":
		w.breakLine(2)
		w.text.WriteString("----------")
		w.breakLine(2)
	case "img":
		w.words(" " + alt + " ")
	case "a":
		w.links = append(w.links, href)
		w.linkStart = append(w.linkStart, w.text.Len())
	case "li":
		w.breakLine(1)
		w.text.WriteString("- ")
	default:
		if n := emailBlocks[name]; n > 0 {
			w.breakLine(1)
		}
	}
}

func (w *emailWriter) end(name string) {
	if w.skip != "" {
		if w.skip == name {
			if w.skipDepth > 0 {
				w.skipDepth--
			} else {
				w.skip = ""
			}
		}
		return
	}
	switch name {
	case "a":
		if len(w.links) == 0 {
			return
		}
		href, start := w.links[len(w.links)-1], w.linkStart[len(w.linkStart)-1]
		w.links, w.linkStart = w.links[:len(w.links)-1], w.linkStart[:len(w.linkStart)-1]
		label := strings.TrimSpace(w.text.String()[min(start, w.text.Len()):])
		if href != "" && label != href {
			if label != "" {
				w.text.WriteByte(' ')
			}
			w.text.WriteString("(" + href + ")")
		}
	case "td", "th":
		w.space = true
	default:
		if n := emailBlocks[name]; n > 0 {
			w.breakLine(n)
		}
	}
}

// plain returns the plain text of the email.
func (w *emailWriter) plain() string {
	lines := strings.Split(w.text.String(), "\n")
	var out []string
	blank := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			blank++
			if blank > 1 || len(out) == 0 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
