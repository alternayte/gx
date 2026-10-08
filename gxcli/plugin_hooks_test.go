package gxcli_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/gx/examples/chartzoom"
	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/islands"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The tests of this file run the example plugin of the repo,
// examples/chartzoom, through the commands of gx: one test for each hook of
// the plugin API (REQ-PLG-01).

func zoomPlugin(opt chartzoom.Options) gxcli.Option { return gxcli.WithPlugins(chartzoom.Plugin(opt)) }

const zoomChart = "package ui\n\n<figure chartzoom:zoom=\"3x\"><div chartzoom:zoom>x</div></figure>\n"

// TestREQ_PLG_01_DirectivesHook checks the directive of the example plugin:
// the generated code has its attribute, and the bundle has its behaviour.
func TestREQ_PLG_01_DirectivesHook(t *testing.T) {
	dir := scratchModule(t, map[string]string{"ui/Chart.gx": zoomChart})
	if code := gxcli.Main([]string{"generate", dir}, zoomPlugin(chartzoom.Options{})); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	src, err := os.ReadFile(filepath.Join(dir, "ui", "Chart_gx.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `Key: "data-chartzoom", Value: "3x"`) || !strings.Contains(string(src), `Key: "data-chartzoom", Value: "2x"`) {
		t.Errorf("the generated code has no attribute of the directive:\n%s", src)
	}
	bundle, err := islands.Build(dir, islands.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if file := bundle.Entries["gx-directive/chartzoom/zoom"]; !strings.Contains(string(bundle.Files[file]), "data-chartzoom-live") {
		t.Errorf("the bundle has no behaviour of the directive: %v", bundle.Entries)
	}
	bad := scratchModule(t, map[string]string{"ui/Chart.gx": "package ui\n\n<figure chartzoom:zoom=\"big\"></figure>\n"})
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main([]string{"generate", bad}, zoomPlugin(chartzoom.Options{})) })
	if code == 0 || !strings.Contains(text, "the zoom level is a number and x") {
		t.Errorf("a bad zoom level: code %d, %s", code, text)
	}
}

// TestREQ_PLG_01_AdapterHook checks the adapter of the example plugin: its
// name is an adapter of gx.toml, and the compiler reports a signal under it,
// because the plugin says that the adapter has no signals.
func TestREQ_PLG_01_AdapterHook(t *testing.T) {
	plain := map[string]string{"gx.toml": "adapter = \"static\"\n", "ui/Card.gx": "package ui\n\n<article>Card</article>\n"}
	dir := scratchModule(t, plain)
	if code := gxcli.Main([]string{"generate", dir}, zoomPlugin(chartzoom.Options{})); code != 0 {
		t.Fatalf("gx generate with the adapter of the plugin = %d", code)
	}
	if code := gxcli.Main([]string{"check", dir}, zoomPlugin(chartzoom.Options{})); code != 0 {
		t.Errorf("gx check with the adapter of the plugin = %d", code)
	}
	// The adapter has no signals: a signal is GX4006.
	signals := scratchModule(t, map[string]string{
		"gx.toml":       "adapter = \"static\"\n",
		"ui/Counter.gx": "package ui\n\nsignals {\n  Count int = 0\n}\n\n<button on:click={$Count++}>Add</button>\n",
	})
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main([]string{"generate", signals}, zoomPlugin(chartzoom.Options{})) })
	if code == 0 || !strings.Contains(text, "GX4006") {
		t.Errorf("a signal under an adapter with no signals = %d: %s", code, text)
	}
	// The compiler knows the adapter only from the plugin: the next command
	// with no plugin does not have its rule.
	text = captureStderr(t, func() { code = gxcli.Main([]string{"generate", signals}) })
	if strings.Contains(text, "GX4006") {
		t.Errorf("the rule of the adapter of the plugin holds with no plugin: %s", text)
	}
}

// TestREQ_PLG_01_BuildStepsHook checks the build step of the example plugin:
// gx build runs it after the generated code is written, and a step that
// fails stops the build.
func TestREQ_PLG_01_BuildStepsHook(t *testing.T) {
	files := map[string]string{"ui/Chart.gx": zoomChart, "main.go": "package main\n\nimport _ \"app/ui\"\n\nfunc main() {}\n"}
	dir := scratchModule(t, files)
	bin := execname.Name(filepath.Join(t.TempDir(), "app"))
	if code := gxcli.Main([]string{"build", "-main", ".", "-o", bin, dir}, zoomPlugin(chartzoom.Options{})); code != 0 {
		t.Fatalf("gx build = %d", code)
	}
	data, err := os.ReadFile(filepath.Join(dir, "zoom-levels.json"))
	if err != nil || strings.TrimSpace(string(data)) != `["3x"]` {
		t.Errorf("the file of the build step: %v, %s", err, data)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Errorf("gx build wrote no binary: %v", err)
	}
	// A step sees the generated code, and its error stops the build.
	var sawGenerated bool
	failing := gxcli.WithPlugins(stepPlugin{testPlugin{"stopper", gxcli.PluginAPI}, func(ctx context.Context, b gxcli.BuildInfo) error {
		_, err := os.Stat(filepath.Join(b.Root, "ui", "Chart_gx.go"))
		sawGenerated = err == nil
		return errors.New("the licence file is missing")
	}}, chartzoom.Plugin(chartzoom.Options{}))
	dir = scratchModule(t, files)
	bin = execname.Name(filepath.Join(t.TempDir(), "app"))
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main([]string{"build", "-main", ".", "-o", bin, dir}, failing) })
	if code != 1 || !sawGenerated || !strings.Contains(text, "stopper") || !strings.Contains(text, "the licence file is missing") {
		t.Errorf("a failing step: code %d, saw generated %v, %s", code, sawGenerated, text)
	}
	if _, err := os.Stat(bin); err == nil {
		t.Error("gx build wrote a binary after a step failed")
	}
}

type stepPlugin struct {
	testPlugin
	run func(context.Context, gxcli.BuildInfo) error
}

func (p stepPlugin) BuildSteps() []gxcli.BuildStep {
	return []gxcli.BuildStep{{Name: "licence", Run: p.run}}
}

// TestREQ_PLG_01_RegistriesHook checks the registry of the example plugin:
// gx add reads it by its name, and a registry of that name in gx.toml goes
// first.
func TestREQ_PLG_01_RegistriesHook(t *testing.T) {
	f := registryFixture(t)
	app := t.TempDir()
	out, code := captureStdout(t, func() int {
		return gxcli.Main([]string{"add", "@chartzoom/button", app}, zoomPlugin(chartzoom.Options{Registry: f.out}))
	})
	if code != 0 || !strings.Contains(out, "added button") {
		t.Fatalf("gx add from the registry of the plugin = %d:\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(app, "ui", "button", "Button.gx")); err != nil {
		t.Errorf("the item of the registry of the plugin is not in the app: %v", err)
	}
	// With no plugin the name is no registry.
	text := captureStderr(t, func() { code = gxcli.Main([]string{"add", "@chartzoom/button", t.TempDir()}) })
	if code == 0 || !strings.Contains(text, "@chartzoom") {
		t.Errorf("gx add with no plugin = %d: %s", code, text)
	}
	// The gx.toml of the project goes before the plugin.
	own := t.TempDir()
	writeFile(t, own, "gx.toml", "[registries]\nchartzoom = \""+filepath.Join(t.TempDir(), "nothing")+"\"\n")
	text = captureStderr(t, func() {
		code = gxcli.Main([]string{"add", "@chartzoom/button", own}, zoomPlugin(chartzoom.Options{Registry: f.out}))
	})
	if code == 0 {
		t.Errorf("gx add read the registry of the plugin and not the registry of gx.toml: %s", text)
	}
}

// TestREQ_PLG_01_ThemePresetsHook checks the theme of the example plugin: gx
// add theme:ocean writes it as the theme of the app, and does not overwrite
// a theme that the app changed.
func TestREQ_PLG_01_ThemePresetsHook(t *testing.T) {
	app := t.TempDir()
	plugin := zoomPlugin(chartzoom.Options{})
	if code := gxcli.Main([]string{"add", "theme:ocean", app}, plugin); code != 0 {
		t.Fatalf("gx add theme:ocean = %d", code)
	}
	theme := filepath.Join(app, "app", "theme.css")
	css, err := os.ReadFile(theme)
	if err != nil || !strings.Contains(string(css), "--primary: oklch(0.5 0.15 240)") {
		t.Fatalf("the theme of the app: %v\n%s", err, css)
	}
	// The same theme a second time changes nothing.
	if code := gxcli.Main([]string{"add", "theme:ocean", app}, plugin); code != 0 {
		t.Errorf("gx add theme:ocean a second time = %d", code)
	}
	// The app changed its theme: the preset goes next to it.
	changed := string(css) + "\n:root { --primary: red; }\n"
	if err := os.WriteFile(theme, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := gxcli.Main([]string{"add", "theme:ocean", app}, plugin); code != 0 {
		t.Errorf("gx add theme:ocean on a changed theme = %d", code)
	}
	if after, _ := os.ReadFile(theme); string(after) != changed {
		t.Error("gx add theme:ocean overwrote the theme of the app")
	}
	if next, err := os.ReadFile(filepath.Join(app, "app", "theme.ocean.css")); err != nil || string(next) != string(css) {
		t.Errorf("the preset next to the theme of the app: %v", err)
	}
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main([]string{"add", "theme:forest", app}, plugin) })
	if code != 1 || !strings.Contains(text, "ocean") {
		t.Errorf("an unknown theme = %d: %s", code, text)
	}
	text = captureStderr(t, func() { code = gxcli.Main([]string{"add", "theme:ocean", app}) })
	if code != 1 {
		t.Errorf("gx add theme:ocean with no plugin = %d: %s", code, text)
	}
}

// TestREQ_PLG_01_MCPToolsHook checks the MCP tool of the example plugin. The
// gx command of a project, with the plugin compiled in, serves the tool
// next to the tools of gx mcp. The test also runs the command of the plugin
// through that binary (the Commands hook).
func TestREQ_PLG_01_MCPToolsHook(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := scratchModule(t, map[string]string{
		"ui/Chart.gx": zoomChart,
		"cmd/gx/main.go": "package main\n\nimport (\n\t\"os\"\n\n\t\"github.com/alternayte/gx/examples/chartzoom\"\n\t\"github.com/alternayte/gx/gxcli\"\n)\n\n" +
			"func main() {\n\tos.Exit(gxcli.Main(os.Args[1:], gxcli.WithPlugins(chartzoom.Plugin(chartzoom.Options{}))))\n}\n",
	})
	bin := execname.Name(filepath.Join(t.TempDir(), "gx"))
	build := exec.Command("go", "build", "-o", bin, "./cmd/gx")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the gx of the project: %v\n%s", err, out)
	}
	if out, err := exec.Command(bin, "zoom-levels", dir).CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "3x" {
		t.Errorf("gx zoom-levels: %v, %s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.Command(bin, "mcp", dir)
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "plugin-test", Version: "0.0.0"}, nil)
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
	if got := strings.Join(names, " "); got != "a11y_audit chartzoom_levels check describe registry_add registry_search render_fixture routes screenshot_route" {
		t.Fatalf("tools = %s", got)
	}
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "chartzoom_levels", Arguments: map[string]any{"dir": dir}})
	if err != nil || res.IsError {
		t.Fatalf("chartzoom_levels: %v %+v", err, res)
	}
	var levels []string
	if text, ok := res.Content[0].(*mcp.TextContent); !ok || json.Unmarshal([]byte(text.Text), &levels) != nil || len(levels) != 1 || levels[0] != "3x" {
		t.Errorf("chartzoom_levels = %+v", res.Content)
	}

	// A tool with the name of a tool of gx mcp stops the load.
	code := 0
	text := captureStderr(t, func() {
		code = gxcli.Main([]string{"help"}, gxcli.WithPlugins(toolPlugin{testPlugin{"clash", gxcli.PluginAPI}}))
	})
	if code != 1 || !strings.Contains(text, "check") {
		t.Errorf("a tool with the name check: code %d, %s", code, text)
	}
}

type toolPlugin struct{ testPlugin }

func (toolPlugin) MCPTools() []gxcli.MCPTool {
	return []gxcli.MCPTool{{Name: "check", Description: "A tool with a taken name.", InputSchema: json.RawMessage(`{"type":"object"}`),
		Call: func(context.Context, json.RawMessage) (string, error) { return "", nil }}}
}
