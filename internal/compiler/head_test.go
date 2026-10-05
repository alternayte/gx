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
	if !strings.Contains(src, "gx.Head(gx.HeadProps{Title: p.Title, Meta: p.Meta, Links: nil, Lang: \"\", HtmlClass: \"\", BodyClass: \"\"})") {
		t.Fatalf("Page_gx.go does not call gx.Head:\n%s", src)
	}
}

func TestREQ_RTE_13_ActiveLinkCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"products/routes.go": routesGoNoRoutes,
		"products/Nav.gx":    "package products\n\nprops {\n  ID int64\n}\n\n<a href={Show{ID: p.ID}} active=\"section\">p</a>\n",
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "products/Nav_gx.go")])
	if !strings.Contains(src, `Kind: gx.AttrURL, Active: "section"`) {
		t.Fatalf("Nav_gx.go does not mark the link active:\n%s", src)
	}
}
