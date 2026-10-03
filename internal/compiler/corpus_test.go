package compiler_test

import (
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// corpus is 50 pasted HTML fragments in Tailwind UI style (REQ-AUT-03).
var corpus = []string{
	`<button class="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm text-white">Save</button>`,
	"<div class=\"grid grid-cols-3 gap-4\">\n  <div>1</div>\n  <div>2</div>\n  <div>3</div>\n</div>",
	`<img src="/x.jpg" alt="" class="size-10 rounded-full">`,
	`<input type="email" name="email" required>`,
	`<label for="email" class="text-sm font-medium">Email</label>` + "\n" + `<input id="email" type="email">`,
	`<ul class="space-y-2"><li>One</li><li>Two</li></ul>`,
	`<table class="w-full text-sm"><thead><tr><th class="text-left">Name</th></tr></thead><tbody><tr><td>Ada</td></tr></tbody></table>`,
	`<hr class="my-4">`,
	`<br>`,
	`<meta charset="utf-8">`,
	`<a href="/pricing" class="underline">Pricing</a>`,
	`<x-icon name="cart" />`,
	`<button disabled>Send</button>`,
	`<td class=cell>1</td>`,
	`<div class='flex gap-2'>x</div>`,
	`<p class="text-sm">Hello <strong>world</strong>!</p>`,
	"<button class=\"btn\"\n        type=\"button\"\n        aria-label=\"Close\">\n  Close\n</button>",
	`<svg viewBox="0 0 24 24" class="size-6"><path d="M4 4h16v16H4z"/></svg>`,
	`<details class="group"><summary>More</summary><p>Body</p></details>`,
	`<dialog open><p>Hi</p></dialog>`,
	`<div hidden></div>`,
	`<div data-state="open" aria-expanded="true"></div>`,
	`<span class="sr-only">Close</span>`,
	`<div class="relative"><div class="absolute inset-0 bg-black/50"></div></div>`,
	`<p>Tom &amp; Jerry &copy; 2026</p>`,
	`<!-- nav --><nav class="flex">Home</nav>`,
	`<pre><code>go test ./...</code></pre>`,
	`<textarea rows="3">plain</textarea>`,
	`<select><option value="a" selected>A</option></select>`,
	`<div class="flex items-center justify-between"><span>1</span><span>2</span></div>`,
	`<button type="submit" form="f" formaction="/save">Save</button>`,
	`<div style="color: red">x</div>`,
	`<progress value="0.5" max="1"></progress>`,
	`<div id="a" class="a b c d e f"></div>`,
	`<input type="checkbox" checked>`,
	`<div class="[mask-type:luminance]">x</div>`,
	`<div class="w-[calc(100%-2rem)]">x</div>`,
	`<div class="before:content-['*']">x</div>`,
	`<div class='before:content-["*"]'>x</div>`,
	`<input value="a>b">`,
	`<p title="He said &quot;hi&quot;">x</p>`,
	`<div class="hover:bg-primary/90 focus:ring-2"></div>`,
	`<div class="translate-x-1/2 -translate-y-1/2"></div>`,
	`<ul><li class="first:mt-0 last:mb-0">x</li></ul>`,
	`<div class="grid [grid-template-columns:repeat(3,minmax(0,1fr))]">x</div>`,
	`<abbr title="HyperText Markup Language">HTML</abbr>`,
	`<figure><img src="/a.png" alt="A"><figcaption class="text-xs">A</figcaption></figure>`,
	`<input type="search" placeholder="Search&hellip;">`,
	`<div class="divide-y divide-border"><div>1</div><div>2</div></div>`,
	`<h3 class="font-semibold">{p.Title}</h3>`,
}

func TestREQ_AUT_03_HTMLCorpusRoundTrip(t *testing.T) {
	if len(corpus) != 50 {
		t.Fatalf("corpus has %d snippets, want 50", len(corpus))
	}
	for i, src := range corpus {
		f, diags := compiler.ParseFragment([]byte(src))
		if len(diags) > 0 || f == nil {
			t.Fatalf("snippet %d %q: diagnostics = %v", i, src, diags)
		}
		out := compiler.Format(f)
		f2, diags := compiler.ParseFragment(out)
		if len(diags) > 0 || f2 == nil {
			t.Fatalf("snippet %d %q: formatted output does not parse: %v\n%s", i, src, diags, out)
		}
		out2 := compiler.Format(f2)
		if string(out) != string(out2) {
			t.Fatalf("snippet %d %q: format is not canonical:\nfirst:\n%s\nsecond:\n%s", i, src, out, out2)
		}
	}
}
