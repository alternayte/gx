package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ_AUT_12_EventAttributeRejected(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":        moduleWithGx(t),
		"ui/btn/Btn.gx": "package btn\n\nprops {\n  Fn string\n}\n\n<button onclick={p.Fn}>x</button>\n",
	})
	diags := checkDir(t, dir)
	if !hasCode(diags, "GX2007") {
		t.Fatalf("dynamic onclick: diagnostics = %v, want GX2007", diags)
	}
}

func TestSI_02_DynamicURLRejected(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  URL string\n}\n\n<a href={p.URL}>x</a>\n",
	})
	diags := checkDir(t, dir)
	if !hasCode(diags, "GX2011") {
		t.Fatalf("dynamic href: diagnostics = %v, want GX2011", diags)
	}
}

func TestREQ_AUT_12_StyleAttribute(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  CSS gx.Style\n}\n\n<div style={p.CSS}></div>\n",
	})
	files := generateFiles(t, dir)
	card := string(files[filepath.Join(dir, "ui/card/Card_gx.go")])
	if !strings.Contains(card, "Kind: gx.AttrStyle") {
		t.Fatalf("gx.Style is not the style attribute kind:\n%s", card)
	}

	bad := writeTree(t, map[string]string{
		"go.mod":          moduleWithGx(t),
		"ui/card/Card.gx": "package card\n\nprops {\n  CSS string\n}\n\n<div style={p.CSS}></div>\n",
	})
	diags := checkDir(t, bad)
	if !hasCode(diags, "GX2000") {
		t.Fatalf("dynamic string style: diagnostics = %v, want GX2000", diags)
	}
}
