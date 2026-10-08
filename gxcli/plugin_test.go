package gxcli_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/islands"
)

// testPlugin is a plugin with a name and the plugin API that it was built
// for.
type testPlugin struct {
	name string
	api  int
}

func (p testPlugin) Name() string { return p.name }
func (p testPlugin) API() int     { return p.api }

// commandPlugin is a plugin with one command.
type commandPlugin struct {
	testPlugin
	command string
	ran     *[]string
}

func (p commandPlugin) Commands() []gxcli.Command {
	return []gxcli.Command{{
		Name:    p.command,
		Summary: "say hello from the plugin",
		Run: func(args []string) int {
			*p.ran = append(*p.ran, strings.Join(args, " "))
			return 7
		},
	}}
}

// TestREQ_PLG_05_VersionMismatch checks the version of the plugin API: a
// plugin that was built for a different version stops each command with an
// error that names the plugin and the two versions.
func TestREQ_PLG_05_VersionMismatch(t *testing.T) {
	var ran []string
	old := commandPlugin{testPlugin{"chartzoom", gxcli.PluginAPI + 1}, "hello", &ran}
	for _, args := range [][]string{{"hello"}, {"help"}, {"fmt", "--check", t.TempDir()}} {
		code := 0
		text := captureStderr(t, func() { code = gxcli.Main(args, gxcli.WithPlugins(old)) })
		if code != 1 || !strings.Contains(text, "chartzoom") ||
			!strings.Contains(text, "plugin API "+strconv.Itoa(gxcli.PluginAPI+1)) || !strings.Contains(text, "plugin API "+strconv.Itoa(gxcli.PluginAPI)) {
			t.Errorf("gx %s with a plugin of a different API = %d: %s", strings.Join(args, " "), code, text)
		}
	}
	if len(ran) != 0 {
		t.Errorf("the command of the plugin ran: %v", ran)
	}
	// A plugin of this version loads.
	good := commandPlugin{testPlugin{"chartzoom", gxcli.PluginAPI}, "hello", &ran}
	if code := gxcli.Main([]string{"hello", "a", "b"}, gxcli.WithPlugins(good)); code != 7 || len(ran) != 1 || ran[0] != "a b" {
		t.Errorf("the command of a plugin of this API = %d, ran %v", code, ran)
	}
	// A plugin with no name, and two plugins with one name, stop the
	// command.
	for name, plugins := range map[string][]gxcli.Plugin{
		"no name":  {testPlugin{"", gxcli.PluginAPI}},
		"one name": {testPlugin{"a", gxcli.PluginAPI}, testPlugin{"a", gxcli.PluginAPI}},
	} {
		code := 0
		text := captureStderr(t, func() { code = gxcli.Main([]string{"help"}, gxcli.WithPlugins(plugins...)) })
		if code != 1 || text == "" {
			t.Errorf("%s: code %d, text %q", name, code, text)
		}
	}
}

// TestREQ_PLG_01_CommandsHook checks the Commands hook: a command of a
// plugin runs with its arguments, gx help lists it, and a command with the
// name of a command of Gx stops the load.
func TestREQ_PLG_01_CommandsHook(t *testing.T) {
	var ran []string
	p := commandPlugin{testPlugin{"chartzoom", gxcli.PluginAPI}, "hello", &ran}
	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"help"}, gxcli.WithPlugins(p)) })
	if code != 0 || !strings.Contains(out, "hello") || !strings.Contains(out, "say hello from the plugin") || !strings.Contains(out, "chartzoom") {
		t.Errorf("gx help = %d:\n%s", code, out)
	}
	// With no plugin the name is no command.
	if code := gxcli.Main([]string{"hello"}); code != 2 {
		t.Errorf("gx hello with no plugin = %d", code)
	}
	shadow := commandPlugin{testPlugin{"bad", gxcli.PluginAPI}, "build", &ran}
	text := captureStderr(t, func() { code = gxcli.Main([]string{"build"}, gxcli.WithPlugins(shadow)) })
	if code != 1 || !strings.Contains(text, "build") || !strings.Contains(text, "bad") {
		t.Errorf("a plugin command with the name of a Gx command = %d: %s", code, text)
	}
}

// TestREQ_PLG_02_GlobalDelegates checks that a plugin loads through the
// cmd/gx/main.go of the project, and that the global gx runs that command: a
// command of the plugin is available from the global gx in the project, and
// not outside it.
func TestREQ_PLG_02_GlobalDelegates(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := scratchModule(t, map[string]string{
		"hello/hello.go": "package hello\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"github.com/alternayte/gx/gxcli\"\n)\n\n" +
			"type plugin struct{}\n\nfunc Plugin() gxcli.Plugin { return plugin{} }\n\nfunc (plugin) Name() string { return \"hello\" }\nfunc (plugin) API() int { return gxcli.PluginAPI }\n\n" +
			"func (plugin) Commands() []gxcli.Command {\n\treturn []gxcli.Command{{Name: \"hello\", Summary: \"say hello\", Run: func(args []string) int {\n\t\tfmt.Println(\"hello from the plugin: \" + strings.Join(args, \" \"))\n\t\treturn 3\n\t}}}\n}\n",
		"cmd/gx/main.go": "package main\n\nimport (\n\t\"os\"\n\n\t\"app/hello\"\n\n\t\"github.com/alternayte/gx/gxcli\"\n)\n\nfunc main() {\n\tos.Exit(gxcli.Main(os.Args[1:], gxcli.WithPlugins(hello.Plugin())))\n}\n",
		"sub/keep.txt":   "a directory below the root\n",
	})
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	global := filepath.Join(t.TempDir(), "gx")
	if runtime.GOOS == "windows" {
		global += ".exe"
	}
	if out, err := exec.Command("go", "build", "-C", repo, "-o", global, "./cmd/gx").CombinedOutput(); err != nil {
		t.Fatalf("build the global gx: %v\n%s", err, out)
	}
	run := func(in string, args ...string) (string, int) {
		t.Helper()
		cmd := exec.Command(global, args...)
		cmd.Dir = in
		out, err := cmd.CombinedOutput()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return string(out), code
	}
	// In the project, also below its root: the command of the plugin runs
	// with its arguments and its exit code.
	for _, in := range []string{dir, filepath.Join(dir, "sub")} {
		out, code := run(in, "hello", "a", "b")
		if code != 3 || !strings.Contains(out, "hello from the plugin: a b") {
			t.Fatalf("gx hello in %s = %d:\n%s", in, code, out)
		}
	}
	if out, code := run(dir, "help"); code != 0 || !strings.Contains(out, "say hello") {
		t.Errorf("gx help in the project = %d:\n%s", code, out)
	}
	// Outside a project the global gx has no plugin.
	if out, code := run(t.TempDir(), "hello"); code != 2 || !strings.Contains(out, "unknown command") {
		t.Errorf("gx hello outside the project = %d:\n%s", code, out)
	}
}

// directivePlugin is a plugin with directives.
type directivePlugin struct {
	testPlugin
	directives []gxcli.Directive
}

func (p directivePlugin) Directives() []gxcli.Directive { return p.directives }

func zoomDirective(name string) gxcli.Directive {
	return gxcli.Directive{
		Name: name,
		Transform: func(use gxcli.DirectiveUse) ([]gxcli.DirectiveAttr, error) {
			if !use.HasValue {
				return []gxcli.DirectiveAttr{{Name: "data-chartzoom", Value: "1x"}}, nil
			}
			if !strings.HasSuffix(use.Value, "x") {
				return nil, errors.New("the zoom is a number and x, for example 2x")
			}
			return []gxcli.DirectiveAttr{{Name: "data-chartzoom", Value: use.Value}, {Name: "data-chartzoom-tag", Value: use.Tag}}, nil
		},
		Behavior: []byte("for (const el of document.querySelectorAll('[data-chartzoom]')) el.setAttribute('data-chartzoom-live', '')\n"),
	}
}

// TestREQ_PLG_03_Directives checks the directives of a plugin: a directive
// needs a namespace or the plugin does not load, a use becomes the
// attributes of its Transform at compile time, and the behaviour module of
// the directive is in the bundle of the app.
func TestREQ_PLG_03_Directives(t *testing.T) {
	// A directive with no namespace, or with a namespace of Gx, stops the
	// load of the plugin.
	for _, name := range []string{"zoom", ":zoom", "chartzoom:", "Chartzoom:zoom", "on:zoom", "class:zoom", "bind:zoom", "attr:zoom"} {
		p := directivePlugin{testPlugin{"chartzoom", gxcli.PluginAPI}, []gxcli.Directive{zoomDirective(name)}}
		code := 0
		text := captureStderr(t, func() { code = gxcli.Main([]string{"help"}, gxcli.WithPlugins(p)) })
		if code != 1 || !strings.Contains(text, "namespace") {
			t.Errorf("the directive %q: code %d, %s", name, code, text)
		}
	}

	plugin := gxcli.WithPlugins(directivePlugin{testPlugin{"chartzoom", gxcli.PluginAPI}, []gxcli.Directive{zoomDirective("chartzoom:zoom")}})
	page := func(markup string) map[string]string {
		return map[string]string{"ui/Chart.gx": "package ui\n\nprops {\n  Level string = \"3x\"\n}\n\n" + markup + "\n"}
	}
	dir := scratchModule(t, page(`<figure chartzoom:zoom="2x"><div chartzoom:zoom>x</div></figure>`))
	if code := gxcli.Main([]string{"generate", dir}, plugin); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	src, err := os.ReadFile(filepath.Join(dir, "ui", "Chart_gx.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`gx.Attr{Key: "data-chartzoom", Value: "2x", Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-chartzoom-tag", Value: "figure", Kind: gx.AttrText}`,
		`gx.Attr{Key: "data-chartzoom", Value: "1x", Kind: gx.AttrText}`,
		`gx.ElementModule("gx-directive/chartzoom/zoom")`,
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("the generated code has no %s:\n%s", want, src)
		}
	}
	if strings.Contains(string(src), "chartzoom:zoom") {
		t.Errorf("the directive is in the generated code:\n%s", src)
	}
	// The bundle of the app holds the behaviour of the directive.
	bundle, err := islands.Build(dir, islands.Options{})
	if err != nil {
		t.Fatal(err)
	}
	file := bundle.Entries["gx-directive/chartzoom/zoom"]
	if file == "" || !strings.Contains(string(bundle.Files[file]), "data-chartzoom-live") {
		t.Errorf("the bundle has no behaviour of the directive: entries %v", bundle.Entries)
	}

	// Uses that the directive cannot take are diagnostics at the use.
	for name, tc := range map[string]struct{ markup, want string }{
		"an expression":         {`<figure chartzoom:zoom={p.Level}></figure>`, "takes a static value"},
		"an error of Transform": {`<figure chartzoom:zoom="big"></figure>`, "the zoom is a number and x"},
		"an unknown name":       {`<figure chartzoom:pan="2x"></figure>`, `unknown directive "chartzoom:pan"`},
	} {
		dir := scratchModule(t, page(tc.markup))
		code := 0
		text := captureStderr(t, func() { code = gxcli.Main([]string{"generate", dir}, plugin) })
		if code == 0 || !strings.Contains(text, "GX2003") || !strings.Contains(text, tc.want) || !strings.Contains(text, "Chart.gx:7:9") {
			t.Errorf("%s: code %d, %s", name, code, text)
		}
	}
	// A directive that writes an event attribute or an address is refused.
	hostile := gxcli.WithPlugins(directivePlugin{testPlugin{"evil", gxcli.PluginAPI}, []gxcli.Directive{{
		Name: "evil:run",
		Transform: func(gxcli.DirectiveUse) ([]gxcli.DirectiveAttr, error) {
			return []gxcli.DirectiveAttr{{Name: "onclick", Value: "alert(1)"}, {Name: "href", Value: "javascript:alert(1)"}}, nil
		},
	}}})
	dir = scratchModule(t, page(`<a evil:run>x</a>`))
	code := 0
	text := captureStderr(t, func() { code = gxcli.Main([]string{"generate", dir}, hostile) })
	if code == 0 || !strings.Contains(text, "onclick") || !strings.Contains(text, "href") {
		t.Errorf("a directive that writes an event attribute: code %d, %s", code, text)
	}
	// The next command with no plugin has no directive of the plugin.
	dir = scratchModule(t, page(`<figure chartzoom:zoom={p.Level}></figure>`))
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Errorf("gx generate with no plugin = %d", code)
	}
}
