package fuzz

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// node is one element or one text of a tree. A text has no tag.
type node struct {
	tag      string
	attrs    []string
	text     string
	children []*node
}

// voidElements have no end tag.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true, "input": true,
	"link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// contextOf maps a root element to the element that the parser needs
// around it: a table row parses only in a table.
var contextOf = map[string]atom.Atom{
	"tr": atom.Tbody, "td": atom.Tr, "th": atom.Tr,
	"thead": atom.Table, "tbody": atom.Table, "tfoot": atom.Table, "caption": atom.Table, "colgroup": atom.Table,
	"col": atom.Colgroup, "li": atom.Ul, "dt": atom.Dl, "dd": atom.Dl,
	"option": atom.Select, "optgroup": atom.Select,
}

func attrsOf(list []html.Attribute) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		key := a.Key
		if a.Namespace != "" {
			key = a.Namespace + ":" + key
		}
		out = append(out, strings.ToLower(key)+"="+a.Val)
	}
	sort.Strings(out)
	return out
}

// written returns the tree as the render wrote it: each start tag opens an
// element and each end tag closes one. It is an error when an end tag has
// no start tag or an element has no end tag.
func written(src string) (*node, error) {
	root := &node{}
	stack := []*node{root}
	z := html.NewTokenizer(strings.NewReader(src))
	for {
		switch z.Next() {
		case html.ErrorToken:
			if z.Err() != io.EOF {
				return nil, z.Err()
			}
			if len(stack) > 1 {
				return nil, fmt.Errorf("<%s> has no end tag", stack[len(stack)-1].tag)
			}
			return root, nil
		case html.TextToken:
			top := stack[len(stack)-1]
			top.children = append(top.children, &node{text: string(z.Text())})
		case html.StartTagToken, html.SelfClosingTagToken:
			// The raw tag shows an attribute that the source has two
			// times; the parser keeps the first one only.
			raw := string(z.Raw())
			tok := z.Token()
			n := &node{tag: strings.ToLower(tok.Data), attrs: attrsOf(tok.Attr)}
			if key := twice(raw); key != "" {
				return nil, fmt.Errorf("the attributes of <%s> differ after the parse: %s is there two times", n.tag, key)
			}
			top := stack[len(stack)-1]
			top.children = append(top.children, n)
			if tok.Type == html.StartTagToken && !voidElements[n.tag] {
				stack = append(stack, n)
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			tag := strings.ToLower(string(name))
			if top := stack[len(stack)-1]; len(stack) == 1 || top.tag != tag {
				return nil, fmt.Errorf("</%s> closes no open element; the open element is <%s>", tag, top.tag)
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// twice returns the name of an attribute that a raw start tag holds two
// times, or the empty string.
func twice(raw string) string {
	seen := map[string]bool{}
	i := 1
	// The tag name.
	for i < len(raw) && !strings.ContainsRune(" \t\n\f\r/>", rune(raw[i])) {
		i++
	}
	for i < len(raw) {
		for i < len(raw) && strings.ContainsRune(" \t\n\f\r/", rune(raw[i])) {
			i++
		}
		if i >= len(raw) || raw[i] == '>' {
			return ""
		}
		start := i
		for i < len(raw) && !strings.ContainsRune(" \t\n\f\r/>=", rune(raw[i])) {
			i++
		}
		key := strings.ToLower(raw[start:i])
		if seen[key] {
			return key
		}
		seen[key] = true
		for i < len(raw) && strings.ContainsRune(" \t\n\f\r", rune(raw[i])) {
			i++
		}
		if i >= len(raw) || raw[i] != '=' {
			continue
		}
		i++
		for i < len(raw) && strings.ContainsRune(" \t\n\f\r", rune(raw[i])) {
			i++
		}
		if i < len(raw) && (raw[i] == '"' || raw[i] == '\'') {
			end := strings.IndexByte(raw[i+1:], raw[i])
			if end < 0 {
				return ""
			}
			i += end + 2
			continue
		}
		for i < len(raw) && !strings.ContainsRune(" \t\n\f\r>", rune(raw[i])) {
			i++
		}
	}
	return ""
}

// parsed returns the tree that the HTML parser makes of src.
func parsed(src string, first string) (*node, error) {
	root := &node{}
	var convert func(dst *node, n *html.Node)
	convert = func(dst *node, n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				dst.children = append(dst.children, &node{text: c.Data})
			case html.ElementNode:
				e := &node{tag: strings.ToLower(c.Data), attrs: attrsOf(c.Attr)}
				dst.children = append(dst.children, e)
				convert(e, c)
			case html.DocumentNode:
				convert(dst, c)
			}
		}
	}
	if first == "html" || first == "head" || first == "body" {
		doc, err := html.Parse(strings.NewReader(src))
		if err != nil {
			return nil, err
		}
		convert(root, doc)
		return root, nil
	}
	ctx := atom.Div
	if a, ok := contextOf[first]; ok {
		ctx = a
	}
	nodes, err := html.ParseFragment(strings.NewReader(src), &html.Node{Type: html.ElementNode, DataAtom: ctx, Data: ctx.String()})
	if err != nil {
		return nil, err
	}
	holder := &html.Node{Type: html.DocumentNode}
	for _, n := range nodes {
		holder.AppendChild(n)
	}
	convert(root, holder)
	return root, nil
}

// normal makes two trees comparable: it joins the texts that touch, drops
// a text of white space only, and puts the rows of a `tbody` and the
// columns of a `colgroup` with no attribute into the table. The parser adds
// those two elements when the source has none, and it moves no content for
// that.
func normal(n *node) {
	var out []*node
	for _, c := range n.children {
		if c.tag == "" {
			if len(out) > 0 && out[len(out)-1].tag == "" {
				out[len(out)-1].text += c.text
				continue
			}
			out = append(out, &node{text: c.text})
			continue
		}
		normal(c)
		if n.tag == "table" && (c.tag == "tbody" || c.tag == "colgroup") && len(c.attrs) == 0 {
			out = append(out, c.children...)
			continue
		}
		out = append(out, c)
	}
	kept := out[:0]
	for _, c := range out {
		if c.tag == "" && strings.TrimSpace(c.text) == "" {
			continue
		}
		kept = append(kept, c)
	}
	n.children = kept
}

func (n *node) label() string {
	if n.tag == "" {
		text := strings.TrimSpace(n.text)
		if len(text) > 30 {
			text = text[:30] + "..."
		}
		return fmt.Sprintf("the text %q", text)
	}
	return "<" + n.tag + ">"
}

// diff returns the first difference of two trees, or the empty string.
func diff(w, p *node, path string) string {
	here := path
	if w.tag != "" {
		here = strings.TrimPrefix(path+" > "+w.tag, " > ")
	}
	where := "at the root"
	if here != "" {
		where = "in " + here
	}
	for i := 0; i < len(w.children) || i < len(p.children); i++ {
		switch {
		case i >= len(p.children):
			return fmt.Sprintf("%s: the render wrote %s, and the parser has no node there", where, w.children[i].label())
		case i >= len(w.children):
			return fmt.Sprintf("%s: the parser has %s, which the render did not write there", where, p.children[i].label())
		}
		a, b := w.children[i], p.children[i]
		if a.tag != b.tag || (a.tag == "" && a.text != b.text) {
			return fmt.Sprintf("%s: the render wrote %s, and the parser has %s there", where, a.label(), b.label())
		}
		if strings.Join(a.attrs, "\x00") != strings.Join(b.attrs, "\x00") {
			return fmt.Sprintf("%s: the attributes of %s differ after the parse", where, a.label())
		}
		if d := diff(a, b, here); d != "" {
			return d
		}
	}
	return ""
}

// TreeDefect compares the tree that the render wrote with the tree that
// the HTML parser makes of the same bytes (REQ-AI-11). It returns the first
// difference, or the empty string when the trees are the same.
func TreeDefect(src string) string {
	w, err := written(src)
	if err != nil {
		return err.Error()
	}
	normal(w)
	first := ""
	for _, c := range w.children {
		if c.tag != "" {
			first = c.tag
			break
		}
	}
	p, err := parsed(src, first)
	if err != nil {
		return err.Error()
	}
	normal(p)
	if first == "head" || first == "body" {
		// The parser adds the html element and the part that is missing.
		w = &node{children: []*node{{tag: "html", children: w.children}}}
		for _, c := range p.children {
			if c.tag != "html" {
				continue
			}
			kept := c.children[:0]
			for _, part := range c.children {
				if part.tag == first || len(part.children) > 0 || len(part.attrs) > 0 {
					kept = append(kept, part)
				}
			}
			c.children = kept
		}
	}
	return diff(w, p, "")
}
