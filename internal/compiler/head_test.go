package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ_RTE_11_HeadCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/page/Page.gx": "package page\n\nprops {\n  Title string\n  Meta  []gx.Meta\n}\n\n<gx.Head title={p.Title} meta={p.Meta} />\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "ui/page/Page_gx.go")])
	if !strings.Contains(src, "gx.Head(gx.HeadProps{Title: p.Title, Meta: p.Meta, Links: nil})") {
		t.Fatalf("Page_gx.go does not call gx.Head:\n%s", src)
	}
}
