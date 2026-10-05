package gxcli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// TestREQ_AI_05_AgentsFile covers AGENTS.md: `gx init` writes the golden
// file with conventions, commands and the never-list, and `gx agents
// --update` rewrites only the managed section (REQ-AI-05).
func TestREQ_AI_05_AgentsFile(t *testing.T) {
	dir := initApp(t)
	path := filepath.Join(dir, "AGENTS.md")
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	update := func() string {
		t.Helper()
		out, code := captureStdout(t, func() int { return gxcli.Main([]string{"agents", "--update", dir}) })
		if code != 0 {
			t.Fatalf("gx agents --update exit = %d\n%s", code, out)
		}
		return out
	}
	golden := read()
	snapshot(t, "ai05_agents.golden.md", golden)
	for _, want := range []string{"### Commands", "### Conventions", "### Never", "go run ./cmd/gx check", "Never edit a `_gx.go` file"} {
		if !strings.Contains(golden, want) {
			t.Fatalf("AGENTS.md lacks %q", want)
		}
	}

	// A current file does not change.
	if out := update(); !strings.Contains(out, "current") || read() != golden {
		t.Fatalf("an update of a current file changed it: %s", out)
	}

	// The user edits outside the section, and the section goes stale.
	start := strings.Index(golden, "<!-- gx:agents:start")
	end := strings.Index(golden, "<!-- gx:agents:end -->")
	if start < 0 || end < start {
		t.Fatalf("AGENTS.md has no managed section:\n%s", golden)
	}
	before := "# Acme shop\n\nOur team rule: every slice has an owner.\n\n"
	after := "\n## Project notes\n\n- Deploys run on Friday.\n- <!-- a comment of ours -->\n"
	block := golden[start : end+len("<!-- gx:agents:end -->")]
	markerLine := block[:strings.Index(block, "\n")+1]
	stale := before + markerLine + "## Gx\n\nAn old rule that Gx removed.\n<!-- gx:agents:end -->\n" + after
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := update(); !strings.Contains(out, "updated") {
		t.Fatalf("an update of a stale file reported: %s", out)
	}
	got := read()
	if !strings.HasPrefix(got, before) || !strings.HasSuffix(got, after) {
		t.Fatalf("the update changed text outside the section:\n%s", got)
	}
	if strings.Contains(got, "An old rule that Gx removed.") {
		t.Fatalf("the update kept the stale section:\n%s", got)
	}
	if got[len(before):len(got)-len(after)] != block+"\n" {
		t.Fatalf("the section is not the current one:\n%s", got)
	}

	// A file with no section keeps its text and gets the section at its end.
	own := "# Our rules\n\nNo section here.\n"
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	update()
	got = read()
	if !strings.HasPrefix(got, own) || !strings.Contains(got, "### Never") || !strings.HasSuffix(got, "<!-- gx:agents:end -->\n") {
		t.Fatalf("a file with no section:\n%s", got)
	}

	// A missing file is written new.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	update()
	if read() != golden {
		t.Fatalf("a new AGENTS.md differs from the scaffold:\n%s", read())
	}

	// Without --update the command changes nothing.
	stderr := captureStderr(t, func() {
		if code := gxcli.Main([]string{"agents", dir}); code == 0 {
			t.Error("gx agents with no flag passed")
		}
	})
	if !strings.Contains(stderr, "--update") {
		t.Fatalf("usage = %q", stderr)
	}
}
