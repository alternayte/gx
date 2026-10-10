package gx

import (
	"bytes"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"net/http"
	"net/url"
	"strings"
)

// fragmentHashAttr is the attribute that holds the hash of a fragment
// (REQ-ACT-15).
const fragmentHashAttr = "data-gx-h"

// fragmentsHeader is the request header that holds the hashes of the
// fragments that the browser has: "id=hash,id=hash".
const fragmentsHeader = "Gx-Fragments"

// hashPlace holds the place of a hash in the buffer until the fragment ends.
const hashPlace = "00000000"

// openFragmentState is a fragment whose content the render writes now.
type openFragmentState struct {
	el TemplateEl
	// begin is the offset of the element in the buffer, and place is
	// the offset of its hash.
	begin, place int
	id           string
	node         Node
	// holes holds the parts of the element that its hash leaves out:
	// the hash and the content of each fragment inside it. A change
	// inside an inner fragment then changes only the hash of that
	// fragment, so the server sends only that fragment.
	holes [][2]int
	// parent is the index in the record of the fragment around this
	// one, or -1.
	parent int
	rec    int
}

// fragmentRecord is one fragment of a render: what c.Update compares.
type fragmentRecord struct {
	id     string
	hash   string
	node   Node
	parent int
	// gone is true for a fragment that the render does not have
	// (gx.NoFragment).
	gone bool
}

// goneFragment records a fragment that this render does not have.
func (st *renderState) goneFragment(id string) {
	if st.record == nil || id == "" {
		return
	}
	parent := -1
	if len(st.frags) > 0 {
		parent = st.frags[len(st.frags)-1].rec
	}
	*st.record = append(*st.record, fragmentRecord{id: id, parent: parent, gone: true})
}

// openFragment writes the hash attribute of a fragment element, with a place
// for the hash, and the element becomes the open fragment.
func (st *renderState) openFragment(b *bytes.Buffer, begin int, el TemplateEl, id string, node Node) {
	b.WriteString(" " + fragmentHashAttr + `="`)
	f := &openFragmentState{el: el, begin: begin, place: b.Len(), id: id, node: node, parent: -1, rec: -1}
	b.WriteString(hashPlace)
	b.WriteByte('"')
	if len(st.frags) > 0 {
		f.parent = st.frags[len(st.frags)-1].rec
	}
	if st.record != nil {
		f.rec = len(*st.record)
		*st.record = append(*st.record, fragmentRecord{id: id, node: node, parent: f.parent})
	}
	st.frags = append(st.frags, f)
}

// closeFragment puts the hash of the open fragment into its place. The
// buffer holds the element up to its end.
func (st *renderState) closeFragment(b *bytes.Buffer) {
	f := st.frags[len(st.frags)-1]
	st.frags = st.frags[:len(st.frags)-1]
	data, end := b.Bytes(), b.Len()
	h := fnv.New32a()
	// The place of its own hash is not part of the hash. The holes come
	// after it, in the order of the buffer.
	h.Write(data[f.begin:f.place])
	at := f.place + len(hashPlace)
	for _, hole := range f.holes {
		h.Write(data[at:hole[0]])
		at = hole[1]
	}
	h.Write(data[at:end])
	var sum [4]byte
	hex.Encode(data[f.place:f.place+len(hashPlace)], h.Sum(sum[:0]))
	if f.rec >= 0 {
		(*st.record)[f.rec].hash = string(data[f.place : f.place+len(hashPlace)])
	}
	if len(st.frags) > 0 {
		// The fragment around this one leaves out the hash and the
		// content of this one. It keeps the open tag up to the hash,
		// so a new or a removed inner fragment changes its hash.
		parent := st.frags[len(st.frags)-1]
		parent.holes = append(parent.holes, [2]int{f.place, end})
	}
}

// requestFragments returns the hashes of the fragments that the browser of
// the request has, by id.
func requestFragments(r *http.Request) map[string]string {
	if r == nil {
		return nil
	}
	have := map[string]string{}
	for _, part := range strings.Split(r.Header.Get(fragmentsHeader), ",") {
		if id, hash, ok := strings.Cut(strings.TrimSpace(part), "="); ok {
			have[id] = hash
		}
	}
	return have
}

// Update sends the fragments of the node that differ from the fragments of
// the browser (REQ-ACT-16). n is the node of the invoking component from new
// props. The request holds the hash of each fragment that the browser has;
// the server keeps no render state for a viewer (DR-12). A node with no
// fragment goes as its root elements, as c.Patch sends it.
func (c *Ctx) Update(n Node) error {
	if c.res == nil {
		return errors.New("gx: Update is only valid in an action or a form")
	}
	var records []fragmentRecord
	st := newRenderState(c.R)
	st.hashes, st.record = true, &records
	// The hashes of the browser are from the render of its page. A typed
	// link has the active mark for the address of that page, and not for
	// the address of the action, so the comparison renders for the page.
	if uri := pageURI(c.R); uri != "" {
		st.requestURI = uri
	}
	b := getBuffer()
	collectHead(n, st, 1)
	renderNode(b, n, st)
	putBuffer(b)
	if len(records) == 0 {
		return c.Patch(n)
	}
	have := requestFragments(c.R)
	sent := make([]bool, len(records))
	for i, rec := range records {
		if rec.parent >= 0 && sent[rec.parent] {
			// The fragment around this one goes, with this one in it.
			sent[i] = true
			continue
		}
		if rec.gone {
			// The browser has a fragment that the new props do not
			// give: the patch removes it.
			if _, ok := have[rec.id]; ok {
				sent[i] = true
				c.res.Patches = append(c.res.Patches, ElementPatch{Mode: ModeRemove, Target: idSelector(rec.id)})
			}
			continue
		}
		if rec.id == "" || have[rec.id] == rec.hash {
			continue
		}
		sent[i] = true
		c.res.Patches = append(c.res.Patches, ElementPatch{Mode: ModeMorph, Target: idSelector(rec.id), Node: rec.node})
	}
	return nil
}

// pageURI returns the address of the page that sent an action request: the
// path and the query of its Referer, when the page is of this host. It
// returns "" when the request does not name the page.
func pageURI(r *http.Request) string {
	if r == nil {
		return ""
	}
	u, err := url.Parse(r.Referer())
	if err != nil || u.Host == "" || u.Host != r.Host {
		return ""
	}
	return u.RequestURI()
}
