package gxcli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/alternayte/gx/internal/execname"
)

// TestREQ_AI_04_StdioCommand covers `gx mcp` as a process: a standard MCP
// client starts the command, speaks over stdin and stdout, lists the tools
// and calls two of them. No node is on the path of the server (REQ-AI-04).
func TestREQ_AI_04_StdioCommand(t *testing.T) {
	dir := initApp(t)
	bin := execname.Name(filepath.Join(t.TempDir(), "gx"))
	build := exec.Command("go", "build", "-o", bin, "./cmd/gx")
	build.Dir = thisRepo(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build gx: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.Command(bin, "mcp", "--registry", filepath.Join(thisRepo(t), "registry"), dir)
	cmd.Env = withoutNode(t, os.Environ())
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect to gx mcp: %v", err)
	}
	defer session.Close()

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	if got := strings.Join(names, " "); got != "a11y_audit check describe registry_add registry_search render_fixture routes screenshot_route" {
		t.Fatalf("tools = %s", got)
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "check"})
	if err != nil || res.IsError {
		t.Fatalf("check: %v %+v", err, res)
	}
	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "registry_search", Arguments: map[string]any{"query": "dialog"}})
	if err != nil || res.IsError {
		t.Fatalf("registry_search: %v %+v", err, res)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, `"alert-dialog"`) || !strings.Contains(text, `"dialog"`) {
		t.Fatalf("registry_search dialog = %s", text)
	}
}

// nodeTools are the commands of the node world. No default workflow may
// need one (NFR-07, G3).
var nodeTools = []string{"node", "npm", "npx", "bun", "bunx", "pnpm", "yarn", "deno"}

// withoutNode returns env with a PATH that holds no node tool. A directory
// with a node tool leaves the PATH; the Go tool, git and Chrome keep
// working through links in a directory of their own.
func withoutNode(t *testing.T, env []string) []string {
	t.Helper()
	hasNode := func(dir string) bool {
		for _, tool := range nodeTools {
			for _, name := range []string{tool, tool + ".exe", tool + ".cmd"} {
				if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
					return true
				}
			}
		}
		return false
	}
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir != "" && !hasNode(dir) {
			kept = append(kept, dir)
		}
	}
	// A tool the workflows need can share its directory with node, as in
	// a package manager prefix. It gets a link in a clean directory.
	shim := t.TempDir()
	for _, tool := range []string{"go", "gofmt", "git", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		full, err := exec.LookPath(tool)
		if err != nil || !hasNode(filepath.Dir(full)) {
			continue
		}
		if err := os.Symlink(full, filepath.Join(shim, filepath.Base(full))); err != nil {
			t.Fatalf("%s shares its directory with node and cannot be linked: %v", tool, err)
		}
	}
	path := strings.Join(append([]string{shim}, kept...), string(os.PathListSeparator))
	var out []string
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); !strings.EqualFold(name, "PATH") {
			out = append(out, kv)
		}
	}
	out = append(out, "PATH="+path)

	// Prove it: no node tool resolves on the new PATH.
	for _, tool := range nodeTools {
		for _, dir := range filepath.SplitList(path) {
			if hasNode(dir) {
				t.Fatalf("%s is still on the PATH in %s", tool, dir)
			}
		}
	}
	return out
}
