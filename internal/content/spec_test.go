package content_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/content"
)

// specExample is one CommonMark or GFM extension example.
type specExample struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	Example  int    `json:"example"`
	Section  string `json:"section"`
}

// TestREQ_CNT_01_CommonMarkSpec runs the CommonMark 0.31.2 spec suite
// (REQ-CNT-01).
func TestREQ_CNT_01_CommonMarkSpec(t *testing.T) {
	runSpec(t, "testdata/commonmark.json", func(specExample) content.Options { return content.Options{XHTML: true} })
}

// TestREQ_CNT_01_GFMSpec runs the GFM extension suite: tables, task lists,
// strikethrough, autolinks and footnotes (REQ-CNT-01).
func TestREQ_CNT_01_GFMSpec(t *testing.T) {
	runSpec(t, "testdata/gfm.json", func(ex specExample) content.Options {
		if ex.Section == "footnote" {
			return content.Options{Footnotes: true}
		}
		return content.Options{GFM: true}
	})
}

func runSpec(t *testing.T, path string, options func(specExample) content.Options) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var examples []specExample
	if err := json.Unmarshal(data, &examples); err != nil {
		t.Fatal(err)
	}
	failures := 0
	for _, ex := range examples {
		out, err := content.Render([]byte(ex.Markdown), options(ex))
		if err != nil {
			t.Fatalf("example %d: %v", ex.Example, err)
		}
		if string(out) == ex.HTML {
			continue
		}
		failures++
		if failures <= 5 {
			t.Errorf("example %d (%s): markdown %q got %q want %q",
				ex.Example, ex.Section, ex.Markdown, string(out), ex.HTML)
		}
	}
	if failures > 0 {
		t.Fatalf("%d of %d examples failed", failures, len(examples))
	}
}

// TestREQ_CNT_01_DisallowedRawHTML covers the GFM disallowed raw HTML
// filter: the opening angle bracket of a disallowed tag is escaped
// (REQ-CNT-01).
func TestREQ_CNT_01_DisallowedRawHTML(t *testing.T) {
	md := "<strong> <title> <style> <em>\n\n<blockquote>\n  <xmp> is disallowed.  <XMP> is also disallowed.\n</blockquote>\n"
	want := "<p><strong> &lt;title> &lt;style> <em></p>\n<blockquote>\n  &lt;xmp> is disallowed.  &lt;XMP> is also disallowed.\n</blockquote>\n"
	got, err := content.Render([]byte(md), content.Options{GFM: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// TestREQ_CNT_01_Headings covers stable heading ids and anchor links
// (REQ-CNT-01).
func TestREQ_CNT_01_Headings(t *testing.T) {
	md := "# Hello World\n\n## Hello World\n"
	got, err := content.Render([]byte(md), content.Options{Anchors: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<h1 id="hello-world">Hello World<a class="anchor" href="#hello-world" aria-hidden="true">#</a></h1>`,
		`<h2 id="hello-world-1">Hello World<a class="anchor" href="#hello-world-1" aria-hidden="true">#</a></h2>`,
	} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("output lacks %q:\n%s", want, got)
		}
	}
	// The same source gives the same ids.
	again, err := content.Render([]byte(md), content.Options{Anchors: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(got) {
		t.Fatal("heading ids are not stable")
	}
	// Without anchors there are no ids.
	plain, err := content.Render([]byte(md), content.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), `id=`) {
		t.Fatalf("plain render has ids: %s", plain)
	}
}

// TestREQ_CNT_01_Footnotes covers footnote rendering (REQ-CNT-01).
func TestREQ_CNT_01_Footnotes(t *testing.T) {
	md := "Text with a note[^1].\n\n[^1]: The note.\n"
	got, err := content.Render([]byte(md), content.Options{Footnotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `id="fn:1"`) || !strings.Contains(string(got), "The note.") {
		t.Fatalf("footnote output = %s", got)
	}
}
