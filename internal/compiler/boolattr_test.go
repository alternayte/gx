package compiler_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ_AUT_08_BooleanAttributeExpression(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":             moduleWithGx(t),
		"ui/form/Toggle.gx":  "package form\n\nprops {\n  On bool\n}\n\n<input type=\"checkbox\" checked={p.On}>\n",
		"ui/form/Profile.gx": "package form\n\nprops {\n  Name string\n}\n\n<input value={p.Name}>\n",
	})
	files := generateFiles(t, dir)

	toggle := string(files[filepath.Join(dir, "ui/form/Toggle_gx.go")])
	if !strings.Contains(toggle, `gx.Bool("checked", p.On)`) {
		t.Fatalf("boolean attribute with a bool expression is not gx.Bool:\n%s", toggle)
	}

	profile := string(files[filepath.Join(dir, "ui/form/Profile_gx.go")])
	if strings.Contains(profile, "gx.Bool(\"value\"") {
		t.Fatalf("non-boolean attribute was turned into gx.Bool:\n%s", profile)
	}
	if !strings.Contains(profile, `Value: p.Name`) {
		t.Fatalf("non-boolean expression attribute is wrong:\n%s", profile)
	}
}
