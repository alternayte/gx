package lsp_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/compiler"
)

// actionModule builds a module with one action invocation and one
// registration.
func actionModule(t *testing.T) (dir, card, routes string) {
	t.Helper()
	dir = module(t, map[string]string{
		"ui/card/Card.gx":   "package card\n\n<button on:click={Reset{}}>Reset</button>\n",
		"ui/card/routes.go": "package card\n\nimport \"github.com/alternayte/gx\"\n\ntype Reset struct {\n\tgx.Route `POST /card/reset`\n}\n\ntype Wipe struct {\n\tgx.Route `POST /card/wipe`\n}\n\nvar resetAction = gx.Action(func(c *gx.Ctx, in Reset) error { return nil })\n",
	})
	return dir, filepath.Join(dir, "ui/card/Card.gx"), filepath.Join(dir, "ui/card/routes.go")
}

// TestREQ_DEV_11_ActionIndexRename covers the whole-module index from route
// types to registered actions: renaming the registered input type shows
// GX4001 at the invoking .gx site within 2 s, in the LSP and in gx check
// (REQ-DEV-11).
func TestREQ_DEV_11_ActionIndexRename(t *testing.T) {
	dir, card, routes := actionModule(t)
	c := newClient(t, dir)
	c.initialize()
	c.didOpen(card, readBody(t, card))
	d := c.waitDiagnostics(card)
	for _, item := range d.Items {
		if item.Code == "GX4001" {
			t.Fatalf("clean state has GX4001: %+v", d.Items)
		}
	}

	// Rename the input type of the registration: Reset now has no action.
	renamed := strings.Replace(readBody(t, routes), "in Reset", "in Wipe", 1)
	if err := os.WriteFile(routes, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	d = c.waitDiagnostics(card)
	elapsed := time.Since(start)
	found := false
	for _, item := range d.Items {
		if item.Code == "GX4001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no GX4001 after the rename: %+v", d.Items)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("GX4001 after %s, want under 2s", elapsed)
	}

	// gx check keeps the same index.
	diags := compiler.Check(dir)
	found = false
	for _, diag := range diags {
		if diag.Code == compiler.CodeActionMissing {
			found = true
		}
	}
	if !found {
		t.Fatalf("gx check lost the action index: %v", diags)
	}
}
