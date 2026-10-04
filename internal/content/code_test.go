package content_test

import (
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/content"
)

// TestREQ_CNT_04_CodeOptions covers the code block options: line marks,
// ins/del/word marks, title, frame and wrap (REQ-CNT-04).
func TestREQ_CNT_04_CodeOptions(t *testing.T) {
	code := "func main() {\n\tx := 1\n\t_ = x\n}\n"

	// A plain block is a figure with line spans.
	plain := content.RenderCode("go", "", code)
	for _, want := range []string{`<figure class="gx-code" data-lang="go">`, `<pre class="gx-code-pre">`, `<span class="line">`} {
		if !strings.Contains(plain, want) {
			t.Fatalf("plain lacks %q:\n%s", want, plain)
		}
	}

	// A title and frame.
	titled := content.RenderCode("go", `go title="main.go" frame="code"`, code)
	if !strings.Contains(titled, `<span class="gx-code-title">main.go</span>`) {
		t.Fatalf("title missing:\n%s", titled)
	}

	// Line marks, ins, del and word marks.
	marked := content.RenderCode("go", "go {2-3} ins={4} del={1} word={2}", code)
	for _, want := range []string{
		`<span class="line mark">`,
		`<span class="line ins">`,
		`<span class="line del">`,
		`<span class="line mark word">`,
	} {
		if !strings.Contains(marked, want) {
			t.Fatalf("marks lack %q:\n%s", want, marked)
		}
	}

	// Wrap.
	wrapped := content.RenderCode("go", "go wrap", code)
	if !strings.Contains(wrapped, `<pre class="gx-code-pre wrap">`) {
		t.Fatalf("wrap missing:\n%s", wrapped)
	}

	// A shell fence gets the terminal frame by default.
	terminal := content.RenderCode("sh", `sh title="Terminal"`, "echo hi\n")
	for _, want := range []string{`class="gx-code terminal"`, `<i class="gx-dot"></i>`, `Terminal`} {
		if !strings.Contains(terminal, want) {
			t.Fatalf("terminal lacks %q:\n%s", want, terminal)
		}
	}
}

// TestREQ_CNT_04_Golden pins one full block: a plaintext fence with marks.
func TestREQ_CNT_04_Golden(t *testing.T) {
	got := content.RenderCode("text", `text title="t" frame="code" {2} ins={3} del={1} word={3} wrap`, "a\nb\nc\n")
	want := `<figure class="gx-code" data-lang="text"><figcaption class="gx-code-bar"><span class="gx-code-title">t</span><button class="gx-copy" type="button" data-gx-copy>Copy</button></figcaption><pre class="gx-code-pre wrap"><code><span class="line del"><span class="tok-text">a</span></span>` + "\n" +
		`<span class="line mark"><span class="tok-text">b</span></span>` + "\n" +
		`<span class="line ins word"><span class="tok-text">c</span></span></code></pre></figure>`
	if got != want {
		t.Fatalf("golden:\ngot  %s\nwant %s", got, want)
	}
}

// TestREQ_CNT_04_CodeCSS covers the light and dark token variables
// (REQ-CNT-04).
func TestREQ_CNT_04_CodeCSS(t *testing.T) {
	css := content.CodeCSS()
	for _, want := range []string{":root {", ".dark {", "prefers-color-scheme: dark", "--gx-code-bg:", ".tok-keyword { color: var(--gx-code-keyword);"} {
		if !strings.Contains(css, want) {
			t.Fatalf("CSS lacks %q", want)
		}
	}
}

// TestREQ_CNT_04_RenderHighlight covers highlighting through the Markdown
// renderer (REQ-CNT-04).
func TestREQ_CNT_04_RenderHighlight(t *testing.T) {
	md := "```go title=\"x.go\" {1}\npackage main\n```\n"
	got, err := content.Render([]byte(md), content.Options{Highlight: true})
	if err != nil {
		t.Fatal(err)
	}
	html := string(got)
	for _, want := range []string{`<figure class="gx-code" data-lang="go">`, "x.go", `<span class="line mark">`, "tok-keyword"} {
		if !strings.Contains(html, want) {
			t.Fatalf("render lacks %q:\n%s", want, html)
		}
	}
}
