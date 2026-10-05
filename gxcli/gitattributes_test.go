package gxcli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_DEV_12_GitAttributes covers the .gitattributes of a new app: the
// file equals the golden scaffold, and git itself reads a .gx file as HTML
// and a _gx.go file as generated (REQ-DEV-12).
func TestREQ_DEV_12_GitAttributes(t *testing.T) {
	dir := initApp(t)
	data, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot(t, "dev12_gitattributes.golden.txt", string(data))

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	git("init", "-q")
	attrs := git("check-attr", "linguist-language", "linguist-generated", "--",
		"home/Home.gx", "home/Home_gx.go", "home/route/route_gx.go", "home/home.go", "ui/card/Card.gx")
	for _, want := range []string{
		"home/Home.gx: linguist-language: HTML",
		"ui/card/Card.gx: linguist-language: HTML",
		"home/Home_gx.go: linguist-generated: true",
		"home/route/route_gx.go: linguist-generated: true",
		"home/home.go: linguist-generated: unspecified",
		"home/Home_gx.go: linguist-language: unspecified",
	} {
		if !strings.Contains(attrs, want) {
			t.Fatalf("git check-attr lacks %q:\n%s", want, attrs)
		}
	}
}
