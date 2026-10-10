package fuzz

import (
	"strings"
	"testing"
)

// The tree check passes what the parser keeps and names what it moves.
func TestREQ_AI_11_TreeDefect(t *testing.T) {
	same := []string{
		`<div class="a"><p>one &amp; two</p><img src="x" alt=""><br></div>`,
		`<tr><td>1</td><td>2</td></tr>`,
		`<table><tr><td>1</td></tr></table>`,
		`<li>one</li><li>two</li>`,
		`<svg viewBox="0 0 1 1"><path d="M0 0" /><linearGradient id="g"></linearGradient></svg>`,
		`<script>if (a < b && c) { d("</div>") }</script><p>x</p>`,
		`<!DOCTYPE html><html lang="en"><head><title>t</title></head><body><main>x</main></body></html>`,
		`<p>&lt;b&gt;&#34;x&#34;&lt;/b&gt;</p>`,
		``,
	}
	for _, src := range same {
		if d := TreeDefect(src); d != "" {
			t.Errorf("%s\n  has the defect: %s", src, d)
		}
	}
	defects := map[string]string{
		`<p>one<div>two</div></p>`:               "",
		`<a href="/a">x<a href="/b">y</a></a>`:   "",
		`<table><div>x</div></table>`:            "",
		`<div><span>x</div>`:                     "closes no open element",
		`<div>x`:                                 "has no end tag",
		`<div/><p>x</p>`:                         "",
		`<button>a<button>b</button></button>`:   "",
		`<div class="a" class="b">x</div>`:       "attributes",
		`<ul><li>a</li></ul><b>raw <i>x</b></i>`: "",
	}
	for src, want := range defects {
		d := TreeDefect(src)
		if d == "" || !strings.Contains(d, want) {
			t.Errorf("%s\n  defect = %q, want one with %q", src, d, want)
		}
	}
}
