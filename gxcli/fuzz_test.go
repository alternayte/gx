package gxcli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/scaffold"
)

// TestREQ_AI_11_FuzzCommand covers `gx fuzz` as a command: for a component
// that panics on an empty string, the exit code is 1, the output has the
// fixture entry and the seed, and --json prints the same failure
// (REQ-AI-11).
func TestREQ_AI_11_FuzzCommand(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := filepath.Join(t.TempDir(), "acme")
	if _, err := scaffold.Init(scaffold.Options{
		Dir: dir, Module: "example.com/acme", Adapter: "datastar", Version: "0.1.0", Replace: thisRepo(t),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := scaffold.New(dir, "component", "ui/crash/Crash"); err != nil {
		t.Fatal(err)
	}
	gx := "package crash\n\nprops {\n  // Name is the name of the person.\n  Name string\n}\n\n<span>{p.Name[:1]}</span>\n"
	if err := os.WriteFile(filepath.Join(dir, "ui", "crash", "Crash.gx"), []byte(gx), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "ui", "crash", "Crash.fixtures.go")); err != nil {
		t.Fatal(err)
	}

	out, code := captureStdout(t, func() int {
		return gxcli.Main([]string{"fuzz", "--seed", "7", "--sets", "2", "--app", dir, "Crash"})
	})
	if code != 1 {
		t.Fatalf("gx fuzz exit = %d, want 1\n%s", code, out)
	}
	for _, want := range []string{
		"example.com/acme/ui/crash.Crash, set 0: the render panics:",
		"fixture for Crash.fixtures.go:",
		`"Fuzz7Set0": {},`,
		"gx fuzz: 1 components, 2 prop sets, 1 failures, seed 7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output lacks %q:\n%s", want, out)
		}
	}

	out, code = captureStdout(t, func() int {
		return gxcli.Main([]string{"fuzz", "--seed", "7", "--sets", "1", "--json", "--app", dir, "crash.Crash"})
	})
	var res struct {
		Seed     uint64
		Failures []struct {
			Component, Kind, Name, Fixture string
			Index                          int
		}
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("--json: %v\n%s", err, out)
	}
	if code != 1 || res.Seed != 7 || len(res.Failures) != 1 || res.Failures[0].Kind != "panic" || res.Failures[0].Name != "Fuzz7Set0" || res.Failures[0].Fixture != "{}" {
		t.Fatalf("--json: exit %d, %+v", code, res)
	}

	// A name of no component is an error of the run, not a pass.
	if _, code := captureStdout(t, func() int {
		return gxcli.Main([]string{"fuzz", "--sets", "1", "--app", dir, "Nothing"})
	}); code != 1 {
		t.Errorf("gx fuzz Nothing: exit %d, want 1", code)
	}
}
