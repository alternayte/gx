package gxcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/execname"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/gxstyles"
	"github.com/alternayte/gx/internal/mcpserver"
)

// PluginAPI is the version of the plugin API of this Gx (REQ-PLG-05). A
// plugin says which version it was built for. A change of the API that
// breaks a plugin gets a new number.
const PluginAPI = 1

// Plugin is a typed extension of the gx command of one project
// (REQ-PLG-01). A plugin is a Go value. The cmd/gx/main.go of the project
// gives it to Main with WithPlugins, so the Go compiler checks it and no
// code loads at run time (REQ-PLG-02).
//
// A plugin adds to Gx through the hook interfaces that it also implements:
// Commands is one of them. A plugin cannot change the grammar of a .gx
// file: no hook takes part in the parse (REQ-PLG-05).
type Plugin interface {
	// Name is the name of the plugin in messages, for example "chartzoom".
	Name() string
	// API is the version of the plugin API that the plugin was built for:
	// return gxcli.PluginAPI of the Gx version that you tested with, as a
	// number.
	API() int
}

// Command is one command that a plugin adds to gx.
type Command struct {
	// Name is the word after gx, for example "deploy".
	Name string
	// Summary is the line of the command in gx help.
	Summary string
	// Run gets the arguments after the name and returns the exit code.
	Run func(args []string) int
}

// Commands is the hook of a plugin that adds commands to gx.
type Commands interface {
	Commands() []Command
}

// Directive is one directive that a plugin adds to .gx files (REQ-PLG-03).
// A directive is an attribute with the namespace of the plugin:
// chartzoom:zoom="2x". It runs at compile time: the compiler puts the
// attributes that Transform returns in the place of the directive. A
// directive cannot change the grammar, and it takes a static value.
type Directive struct {
	// Name is the namespace and the name, for example "chartzoom:zoom".
	Name string
	// Transform returns the HTML attributes of one use. An error is a
	// diagnostic at the place of the use.
	Transform func(use DirectiveUse) ([]DirectiveAttr, error)
	// Behavior is an ES module that a page with the directive loads, or
	// nil. The module finds its elements by the attributes of Transform.
	Behavior []byte
}

// DirectiveUse is one use of a directive in a .gx file: the tag of the
// element and the static value. HasValue is false for the directive with no
// value.
type DirectiveUse = compiler.DirectiveUse

// DirectiveAttr is one HTML attribute that a directive writes.
type DirectiveAttr = compiler.DirectiveAttr

// Directives is the hook of a plugin that adds directives to .gx files.
type Directives interface {
	Directives() []Directive
}

// objectSchema reports whether data is a JSON Schema with the type object.
// The MCP server takes no other schema for the input of a tool.
func objectSchema(data json.RawMessage) bool {
	var schema struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(data, &schema) == nil && schema.Type == "object"
}

// pluginWord is a name that a plugin gives: an adapter, a registry or a
// theme.
var pluginWord = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// toolWord is the name of an MCP tool.
var toolWord = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// directiveName is a directive name with a namespace.
var directiveName = regexp.MustCompile(`^[a-z][a-z0-9-]*:[a-z][a-z0-9-]*$`)

// builtinNamespaces are the namespaces of the directives of Gx and of HTML.
// A plugin cannot take one.
var builtinNamespaces = map[string]bool{"on": true, "bind": true, "class": true, "attr": true, "xml": true, "xmlns": true, "xlink": true, "gx": true}

// AdapterInfo is a hypermedia adapter that a plugin adds (REQ-PLG-01). The
// adapter itself is a gx.Adapter value of a package of the plugin, which
// the app gives to gx.Config. AdapterInfo is what the gx command must know:
// the name for the adapter key of gx.toml, and what the adapter can express,
// so the compiler reports a .gx file that the adapter cannot run (GX4006).
type AdapterInfo struct {
	// Name is the value of the adapter key in gx.toml.
	Name string
	// Signals reports whether the adapter has client signals and client
	// expressions.
	Signals bool
	// Modifiers are the event modifiers that an adapter with no signals
	// can express, for example "debounce".
	Modifiers []string
}

// Adapter is the hook of a plugin that adds a hypermedia adapter.
type Adapter interface {
	Adapter() AdapterInfo
}

// BuildInfo is what a build step knows of the build.
type BuildInfo struct {
	// Root is the directory of the app.
	Root string
}

// BuildStep is one step that a plugin adds to gx build. It runs after the
// generated code, the stylesheet and the island bundle are written, and
// before the Go build.
type BuildStep struct {
	Name string
	Run  func(ctx context.Context, b BuildInfo) error
}

// BuildSteps is the hook of a plugin that adds steps to gx build.
type BuildSteps interface {
	BuildSteps() []BuildStep
}

// Registry is a component registry that a plugin adds. gx add @acme/button
// reads the registry with the name "acme". A registry of the same name in
// gx.toml goes before the registry of a plugin.
type Registry struct {
	Name string
	// URL is an http or https address, or a directory.
	URL string
	// Headers are "name: value" strings for each request to the registry.
	Headers []string
}

// Registries is the hook of a plugin that adds component registries.
type Registries interface {
	Registries() []Registry
}

// MCPTool is one tool that a plugin adds to the dev MCP server of gx mcp.
type MCPTool struct {
	Name        string
	Description string
	// InputSchema is the JSON Schema of the arguments: an object schema.
	InputSchema json.RawMessage
	// Call gets the JSON object of the arguments and returns the text of
	// the answer.
	Call func(ctx context.Context, args json.RawMessage) (string, error)
}

// MCPTools is the hook of a plugin that adds tools to the dev MCP server.
type MCPTools interface {
	MCPTools() []MCPTool
}

// ThemePreset is one theme that a plugin adds. gx add theme:<name> writes
// its CSS as the theme of the app.
type ThemePreset struct {
	Name string
	// CSS is the content of app/theme.css for the theme.
	CSS []byte
}

// ThemePresets is the hook of a plugin that adds themes.
type ThemePresets interface {
	ThemePresets() []ThemePreset
}

// Option changes one run of Main.
type Option func(*config)

// config is what the options of one run of Main give.
type config struct {
	plugins []Plugin
}

// WithPlugins gives Main the plugins of the project. Write it in the
// cmd/gx/main.go of the project:
//
//	os.Exit(gxcli.Main(os.Args[1:], gxcli.WithPlugins(chartzoom.Plugin())))
func WithPlugins(plugins ...Plugin) Option {
	return func(c *config) { c.plugins = append(c.plugins, plugins...) }
}

// builtinCommands are the commands of Gx. A plugin cannot take one of these
// names.
var builtinCommands = map[string]bool{
	"init": true, "new": true, "agents": true, "fmt": true, "check": true, "generate": true, "build": true,
	"dev": true, "routes": true, "describe": true, "lsp": true, "mcp": true, "lint": true, "icons": true,
	"pin": true, "wc": true, "vendor": true, "export": true, "import": true, "registry": true, "add": true,
	"diff": true, "update": true, "help": true, "-h": true, "--help": true,
}

// loaded is the plugins of one run after the load checks.
type loaded struct {
	plugins    []Plugin
	commands   map[string]pluginCommand
	directives []compiler.PluginDirective
	adapters   []compiler.PluginAdapter
	steps      []pluginStep
	registries map[string]Registry
	tools      []mcpserver.ExtraTool
	themes     map[string]ThemePreset
}

type pluginStep struct {
	BuildStep
	plugin string
}

type pluginCommand struct {
	Command
	plugin string
}

// load checks the plugins of a run (REQ-PLG-05). It returns an error text
// for a plugin with no name, a name that two plugins have, a plugin of a
// different plugin API, or a command that takes the name of a different
// command.
func (c *config) load() (*loaded, string) {
	l := &loaded{plugins: c.plugins, commands: map[string]pluginCommand{}, registries: map[string]Registry{}, themes: map[string]ThemePreset{}}
	toolNames := map[string]bool{}
	for _, name := range mcpserver.ToolNames {
		toolNames[name] = true
	}
	names := map[string]bool{}
	for _, p := range c.plugins {
		name := p.Name()
		if name == "" {
			return nil, "a plugin has no name. Each plugin returns a name from Name()."
		}
		if names[name] {
			return nil, "two plugins have the name " + strconv.Quote(name) + ". Give each plugin one time to gxcli.WithPlugins."
		}
		names[name] = true
		if api := p.API(); api != PluginAPI {
			return nil, fmt.Sprintf("the plugin %q is built for plugin API %d, and this gx has plugin API %d. Use a version of the plugin for this gx, or the gx version that the plugin names.", name, api, PluginAPI)
		}
		if hook, ok := p.(Commands); ok {
			for _, cmd := range hook.Commands() {
				switch {
				case cmd.Name == "" || cmd.Run == nil:
					return nil, "the plugin " + strconv.Quote(name) + " has a command with no name or no Run function."
				case builtinCommands[cmd.Name]:
					return nil, "the plugin " + strconv.Quote(name) + " has the command " + strconv.Quote(cmd.Name) + ", which is a command of Gx. Give the command a different name."
				}
				if other, ok := l.commands[cmd.Name]; ok {
					return nil, "the plugins " + strconv.Quote(other.plugin) + " and " + strconv.Quote(name) + " have the command " + strconv.Quote(cmd.Name) + "."
				}
				l.commands[cmd.Name] = pluginCommand{Command: cmd, plugin: name}
			}
		}
		if hook, ok := p.(Directives); ok {
			for _, d := range hook.Directives() {
				ns, _, _ := strings.Cut(d.Name, ":")
				switch {
				case !directiveName.MatchString(d.Name):
					// A directive with no namespace can take the name
					// of an HTML attribute or of a directive of Gx
					// (REQ-PLG-03).
					return nil, "the plugin " + strconv.Quote(name) + " has the directive " + strconv.Quote(d.Name) + ". A plugin directive needs a namespace: a name such as " + strconv.Quote(name+":"+strings.TrimLeft(d.Name, ":")) + ", in lower case."
				case builtinNamespaces[ns]:
					return nil, "the plugin " + strconv.Quote(name) + " has the directive " + strconv.Quote(d.Name) + ", and " + strconv.Quote(ns) + " is a namespace of Gx or of HTML. Use the name of the plugin as the namespace."
				case d.Transform == nil:
					return nil, "the directive " + strconv.Quote(d.Name) + " of the plugin " + strconv.Quote(name) + " has no Transform function."
				}
				for _, other := range l.directives {
					if other.Name == d.Name {
						return nil, "the plugins " + strconv.Quote(other.Plugin) + " and " + strconv.Quote(name) + " have the directive " + strconv.Quote(d.Name) + "."
					}
				}
				l.directives = append(l.directives, compiler.PluginDirective{Name: d.Name, Plugin: name, Transform: d.Transform, Behavior: d.Behavior})
			}
		}
		quoted := strconv.Quote(name)
		if hook, ok := p.(Adapter); ok {
			info := hook.Adapter()
			if !pluginWord.MatchString(info.Name) || info.Name == gxconfig.AdapterDatastar || info.Name == gxconfig.AdapterHtmx {
				return nil, "the plugin " + quoted + " has the adapter " + strconv.Quote(info.Name) + ". An adapter of a plugin has its own name in lower case."
			}
			for _, other := range l.adapters {
				if other.Name == info.Name {
					return nil, "two plugins have the adapter " + strconv.Quote(info.Name) + "."
				}
			}
			l.adapters = append(l.adapters, compiler.PluginAdapter{Name: info.Name, Signals: info.Signals, Modifiers: info.Modifiers})
		}
		if hook, ok := p.(BuildSteps); ok {
			for _, step := range hook.BuildSteps() {
				if step.Name == "" || step.Run == nil {
					return nil, "the plugin " + quoted + " has a build step with no name or no Run function."
				}
				l.steps = append(l.steps, pluginStep{BuildStep: step, plugin: name})
			}
		}
		if hook, ok := p.(Registries); ok {
			for _, reg := range hook.Registries() {
				if !pluginWord.MatchString(reg.Name) || reg.URL == "" {
					return nil, "the plugin " + quoted + " has the registry " + strconv.Quote(reg.Name) + ". A registry has a name in lower case, as in @acme/button, and a URL."
				}
				if _, ok := l.registries[reg.Name]; ok {
					return nil, "two plugins have the registry " + strconv.Quote(reg.Name) + "."
				}
				l.registries[reg.Name] = reg
			}
		}
		if hook, ok := p.(MCPTools); ok {
			for _, tool := range hook.MCPTools() {
				switch {
				case !toolWord.MatchString(tool.Name) || tool.Call == nil || tool.Description == "":
					return nil, "the plugin " + quoted + " has the MCP tool " + strconv.Quote(tool.Name) + ". A tool has a name of lower-case letters, digits and underscores, a description and a Call function."
				case toolNames[tool.Name]:
					return nil, "the plugin " + quoted + " has the MCP tool " + strconv.Quote(tool.Name) + ", which a different tool of gx mcp has. Give the tool a different name."
				case !objectSchema(tool.InputSchema):
					return nil, "the MCP tool " + strconv.Quote(tool.Name) + " of the plugin " + quoted + " has an input schema that is not a JSON Schema of an object. Write {\"type\":\"object\", ...}."
				}
				toolNames[tool.Name] = true
				l.tools = append(l.tools, mcpserver.ExtraTool{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema, Call: tool.Call})
			}
		}
		if hook, ok := p.(ThemePresets); ok {
			for _, theme := range hook.ThemePresets() {
				if !pluginWord.MatchString(theme.Name) || len(theme.CSS) == 0 {
					return nil, "the plugin " + quoted + " has the theme " + strconv.Quote(theme.Name) + ". A theme has a name in lower case and its CSS."
				}
				if _, ok := l.themes[theme.Name]; ok {
					return nil, "two plugins have the theme " + strconv.Quote(theme.Name) + "."
				}
				l.themes[theme.Name] = theme
			}
		}
	}
	return l, ""
}

// usage adds the commands of the plugins to gx help.
func (l *loaded) usage(w io.Writer) {
	if len(l.commands) == 0 {
		return
	}
	names := make([]string, 0, len(l.commands))
	for name := range l.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprint(w, "\nCommands of the plugins of this project:\n")
	for _, name := range names {
		cmd := l.commands[name]
		fmt.Fprintf(w, "  %-9s %s (%s)\n", name, cmd.Summary, cmd.plugin)
	}
}

// active is the plugins of the run of Main. Main sets it one time before a
// command runs; a command reads it.
var active = &loaded{commands: map[string]pluginCommand{}, registries: map[string]Registry{}, themes: map[string]ThemePreset{}}

// addTheme writes the theme preset of a plugin as the theme of the app
// (REQ-PLG-01). A theme that the app changed is not overwritten: the preset
// then goes into a file next to it.
func addTheme(root, name string) int {
	theme, ok := active.themes[name]
	if !ok {
		names := make([]string, 0, len(active.themes))
		for n := range active.themes {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(names) == 0 {
			fmt.Fprintf(os.Stderr, "gx add: no plugin of this project has a theme. The theme %q is not known.\n", name)
		} else {
			fmt.Fprintf(os.Stderr, "gx add: unknown theme %q; the themes of the plugins are %s\n", name, strings.Join(names, ", "))
		}
		return 1
	}
	path := gxstyles.ThemePath(root)
	current, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(current, theme.CSS):
		fmt.Printf("the theme %s is the theme of the app\n", name)
		return 0
	case err == nil:
		// The app has a theme of its own. It is not overwritten.
		path = filepath.Join(filepath.Dir(path), "theme."+name+".css")
	case !os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "gx add: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gx add: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, theme.CSS, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gx add: %v\n", err)
		return 1
	}
	if filepath.Base(path) != "theme.css" {
		fmt.Printf("wrote the theme %s to %s; app/theme.css has changes of the app, so copy the tokens that you want\n", name, path)
	} else {
		fmt.Printf("wrote the theme %s to %s\n", name, path)
	}
	return 0
}

// start loads the plugins of a run. It prints the problem and returns false
// for a plugin that cannot load.
func start(opts []Option) bool {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	l, problem := cfg.load()
	if problem != "" {
		fmt.Fprintln(os.Stderr, "gx: "+problem)
		return false
	}
	active = l
	// The compiler of this process knows the directives of the plugins.
	compiler.SetDirectives(l.directives)
	compiler.SetPluginAdapters(l.adapters)
	return true
}

// projectCLIEnv marks a process that the global gx started as the command of
// a project, so the command of the project does not start one more.
const projectCLIEnv = "GX_PROJECT_CLI"

// gxModule is the module of Gx. Its own cmd/gx is the global command.
const gxModule = "github.com/alternayte/gx"

// Delegate runs the gx command of the project of the working directory, when
// the project has one: the cmd/gx/main.go that gx init writes (REQ-PLG-02).
// That command has the plugins of the project compiled in, and the Gx version
// of the go.mod of the project. The global gx calls Delegate first, and runs
// its own Main when ok is false: the directory is in no project with a
// cmd/gx.
func Delegate(args []string) (code int, ok bool) {
	if os.Getenv(projectCLIEnv) != "" {
		return 0, false
	}
	dir, err := os.Getwd()
	if err != nil {
		return 0, false
	}
	root := moduleRootOf(dir)
	if root == "" {
		return 0, false
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "gx", "main.go")); err != nil {
		return 0, false
	}
	if mod, err := os.ReadFile(filepath.Join(root, "go.mod")); err != nil || modulePath(mod) == gxModule {
		// The cmd/gx of Gx is this command.
		return 0, false
	}
	// go run gives exit code 1 for each failure of the program, so the
	// command of the project is built and then run. The binary has one
	// place for each project, below the cache directory of the user. The
	// Go build cache makes the second build quick.
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	sum := sha256.Sum256([]byte(root))
	bin := execname.Name(filepath.Join(cache, "gx", "project-cli", hex.EncodeToString(sum[:8]), "gx"))
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gx: %v\n", err)
		return 1, true
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/gx")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "gx: the command of the project in %s does not build:\n%s", filepath.Join(root, "cmd", "gx"), out)
		return 1, true
	}
	// A path argument of the user is relative to the working directory, so
	// the command runs there.
	return runProjectCLI(bin, args, append(os.Environ(), projectCLIEnv+"=1")), true
}

// modulePath returns the module path of a go.mod file, or "".
func modulePath(mod []byte) string {
	for _, line := range strings.Split(string(mod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}
