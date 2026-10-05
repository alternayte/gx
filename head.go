package gx

import (
	"net/http"
	"strings"
)

// Meta is one meta tag in the head (REQ-RTE-11).
type Meta struct {
	Name     string `json:"name,omitempty"`
	Property string `json:"property,omitempty"`
	Content  string `json:"content,omitempty"`
}

// Link is one link tag in the head.
type Link struct {
	Rel  string `json:"rel,omitempty"`
	Href string `json:"href,omitempty"`
}

// HeadProps is the props of the gx.Head component.
type HeadProps struct {
	Title string `json:"title,omitempty"`
	Meta  []Meta `json:"meta,omitempty"`
	Links []Link `json:"links,omitempty"`
	// Lang, HtmlClass and BodyClass set attributes of the document shell.
	// The deepest value wins; Lang defaults to "en".
	Lang      string `json:"lang,omitempty"`
	HtmlClass string `json:"htmlClass,omitempty"`
	BodyClass string `json:"bodyClass,omitempty"`
}

// Head marks the head of a page or layout. The deepest title wins and the
// other tags merge.
func Head(p HeadProps) Node { return headNode(p) }

type headNode HeadProps

func (headNode) node() {}

// renderState carries the merged head through one render.
type renderState struct {
	head        HeadProps
	hasHead     bool
	titleDepth  int
	langDepth   int
	htmlDepth   int
	bodyDepth   int
	headWritten bool
	requestURI  string
	// request selects the adapter that writes the action invocations of
	// the render (REQ-PLG-04).
	request    *http.Request
	adapter    Adapter
	adapterSet bool
	// nonce is the CSP nonce every script element gets (SI-11).
	nonce string
	// keepSignals is true for the reload of a dev build: the first values
	// of the signals then do not replace the values in the browser
	// (REQ-DEV-03).
	keepSignals bool
}

// HeadOf returns the merged head of a node tree (REQ-RTE-11, REQ-RTE-12).
func HeadOf(n Node) HeadProps {
	st := &renderState{}
	collectHead(n, st, 1)
	return st.head
}

// collectHead merges every head node of the tree, deepest title first.
func collectHead(n Node, st *renderState, depth int) {
	switch t := n.(type) {
	case headNode:
		mergeHead(st, HeadProps(t), depth)
	case fragNode:
		for _, c := range t {
			collectHead(c, st, depth)
		}
	case *elNode:
		for _, c := range t.children {
			collectHead(c, st, depth+1)
		}
	}
}

func mergeHead(st *renderState, p HeadProps, depth int) {
	if !st.hasHead {
		st.head, st.hasHead, st.titleDepth = p, true, depth
		st.langDepth, st.htmlDepth, st.bodyDepth = depth, depth, depth
		return
	}
	if p.Lang != "" && depth >= st.langDepth {
		st.head.Lang, st.langDepth = p.Lang, depth
	}
	if p.HtmlClass != "" && depth >= st.htmlDepth {
		st.head.HtmlClass, st.htmlDepth = p.HtmlClass, depth
	}
	if p.BodyClass != "" && depth >= st.bodyDepth {
		st.head.BodyClass, st.bodyDepth = p.BodyClass, depth
	}
	if p.Title != "" && depth >= st.titleDepth {
		st.head.Title, st.titleDepth = p.Title, depth
	}
	for _, m := range p.Meta {
		st.head.Meta = mergeMeta(st.head.Meta, m)
	}
	for _, l := range p.Links {
		st.head.Links = mergeLink(st.head.Links, l)
	}
}

func mergeMeta(list []Meta, m Meta) []Meta {
	key := m.Name
	if key == "" {
		key = "#" + m.Property
	}
	for i, cur := range list {
		curKey := cur.Name
		if curKey == "" {
			curKey = "#" + cur.Property
		}
		if curKey == key {
			list[i] = m
			return list
		}
	}
	return append(list, m)
}

func mergeLink(list []Link, l Link) []Link {
	for i, cur := range list {
		if cur.Rel == l.Rel {
			list[i] = l
			return list
		}
	}
	return append(list, l)
}

func renderHead(b *strings.Builder, st *renderState) {
	if st.head.Title != "" {
		renderNode(b, El("title", nil, Text(st.head.Title)), st)
	}
	for _, m := range st.head.Meta {
		var attrs Attrs
		if m.Name != "" {
			attrs = append(attrs, Attr{Key: "name", Value: m.Name})
		}
		if m.Property != "" {
			attrs = append(attrs, Attr{Key: "property", Value: m.Property})
		}
		attrs = append(attrs, Attr{Key: "content", Value: m.Content})
		renderNode(b, El("meta", attrs), st)
	}
	for _, l := range st.head.Links {
		renderNode(b, El("link", Attrs{
			{Key: "rel", Value: l.Rel},
			{Key: "href", Value: l.Href, Kind: AttrURL},
		}), st)
	}
}
