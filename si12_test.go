package gx_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSI_12_NoMarkdownInGx covers the architecture rule: package gx has no
// runtime Markdown renderer; content Markdown compiles at build time
// (SI-12).
func TestSI_12_NoMarkdownInGx(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/alternayte/gx").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := string(out)
	for _, dep := range []string{"goldmark", "chroma", "yaml.v3"} {
		if strings.Contains(deps, dep) {
			t.Fatalf("package gx depends on %s:\n%s", dep, deps)
		}
	}
	// The build-time renderer still uses goldmark.
	compilerDeps, err := exec.Command("go", "list", "-deps", "github.com/alternayte/gx/internal/content").Output()
	if err != nil {
		t.Fatalf("go list internal/content: %v", err)
	}
	if !strings.Contains(string(compilerDeps), "goldmark") {
		t.Fatal("the content renderer does not use goldmark")
	}
}
