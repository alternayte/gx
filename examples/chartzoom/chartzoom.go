// Package chartzoom is the example plugin of Gx: one plugin value with each
// hook of the plugin API (REQ-PLG-01). A project gives it to the gx command
// in its cmd/gx/main.go:
//
//	os.Exit(gxcli.Main(os.Args[1:], gxcli.WithPlugins(chartzoom.Plugin(chartzoom.Options{}))))
//
// Read each method as the pattern for one hook.
package chartzoom

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alternayte/gx/gxcli"
)

// Options are the settings of the plugin for one project.
type Options struct {
	// Registry is the URL or the directory of the component registry of
	// the plugin. Empty adds no registry.
	Registry string
}

// Plugin returns the plugin for a project.
func Plugin(opt Options) gxcli.Plugin { return plugin{opt} }

type plugin struct{ opt Options }

// Name is the name of the plugin in messages. It is also the namespace of
// its directives.
func (plugin) Name() string { return "chartzoom" }

// API is the version of the plugin API that this plugin was built for. The
// number is a literal: a later Gx with a different API then refuses the
// plugin with a clear error.
func (plugin) API() int { return 1 }

// zoom is the form of a zoom level: 2x or 1.5x.
var zoom = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?x$`)

// behavior is the script of the zoom directive.
//
//go:embed zoom.js
var behavior []byte

// Directives adds the chartzoom:zoom directive (the Directives hook). The
// compiler puts a data attribute in its place, and a page with the directive
// loads the behaviour module.
func (plugin) Directives() []gxcli.Directive {
	return []gxcli.Directive{{
		Name: "chartzoom:zoom",
		Transform: func(use gxcli.DirectiveUse) ([]gxcli.DirectiveAttr, error) {
			level := "2x"
			if use.HasValue {
				level = use.Value
			}
			if !zoom.MatchString(level) {
				return nil, errors.New("the zoom level is a number and x, for example 2x")
			}
			return []gxcli.DirectiveAttr{{Name: "data-chartzoom", Value: level}}, nil
		},
		Behavior: behavior,
	}}
}

// Adapter adds the adapter name "static" for gx.toml (the Adapter hook). The
// compiler then knows that the adapter has no signals, and reports a .gx
// file that needs them. The gx.Adapter value of the adapter is runtime code
// of a package of the plugin; the app gives it to gx.Config.
func (plugin) Adapter() gxcli.AdapterInfo {
	return gxcli.AdapterInfo{Name: "static", Signals: false, Modifiers: []string{"once"}}
}

// Commands adds gx zoom-levels (the Commands hook).
func (plugin) Commands() []gxcli.Command {
	return []gxcli.Command{{
		Name:    "zoom-levels",
		Summary: "print the zoom levels that the .gx files of a directory use",
		Run: func(args []string) int {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			levels, err := Levels(root)
			if err != nil {
				fmt.Fprintf(os.Stderr, "gx zoom-levels: %v\n", err)
				return 1
			}
			fmt.Println(strings.Join(levels, " "))
			return 0
		},
	}}
}

var zoomUse = regexp.MustCompile(`chartzoom:zoom="([^"]*)"`)

// Levels returns each zoom level that a .gx file below root uses, in the
// order of the files.
func Levels(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".gx" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range zoomUse.FindAllSubmatch(src, -1) {
			out = append(out, string(m[1]))
		}
		return nil
	})
	return out, err
}

// BuildSteps adds one step to gx build (the BuildSteps hook): it writes the
// zoom levels of the app into a file that the app can embed.
func (plugin) BuildSteps() []gxcli.BuildStep {
	return []gxcli.BuildStep{{
		Name: "zoom-levels",
		Run: func(ctx context.Context, b gxcli.BuildInfo) error {
			levels, err := Levels(b.Root)
			if err != nil {
				return err
			}
			data, err := json.Marshal(levels)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(b.Root, "zoom-levels.json"), append(data, '\n'), 0o644)
		},
	}}
}

// Registries adds the component registry of the plugin (the Registries
// hook): gx add @chartzoom/<item> reads it.
func (p plugin) Registries() []gxcli.Registry {
	if p.opt.Registry == "" {
		return nil
	}
	return []gxcli.Registry{{Name: "chartzoom", URL: p.opt.Registry}}
}

// MCPTools adds one tool to the dev MCP server of gx mcp (the MCPTools
// hook), so a coding agent can ask for the zoom levels of the app.
func (plugin) MCPTools() []gxcli.MCPTool {
	return []gxcli.MCPTool{{
		Name:        "chartzoom_levels",
		Description: "List the zoom levels that the .gx files of a directory use.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"dir":{"type":"string"}},"required":["dir"]}`),
		Call: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Dir string `json:"dir"`
			}
			if err := json.Unmarshal(args, &in); err != nil || in.Dir == "" {
				return "", errors.New("the argument dir is the directory of the app")
			}
			levels, err := Levels(in.Dir)
			if err != nil {
				return "", err
			}
			data, err := json.Marshal(levels)
			return string(data), err
		},
	}}
}

// ThemePresets adds the theme "ocean" (the ThemePresets hook): gx add
// theme:ocean writes it as the theme of the app.
func (plugin) ThemePresets() []gxcli.ThemePreset {
	return []gxcli.ThemePreset{{Name: "ocean", CSS: []byte(oceanTheme)}}
}

const oceanTheme = `@import "tailwindcss";

@source "../.gx/classes.txt";

:root {
  --background: oklch(0.98 0.01 230);
  --foreground: oklch(0.2 0.04 250);
  --primary: oklch(0.5 0.15 240);
  --primary-foreground: oklch(0.98 0.01 230);
  --border: oklch(0.88 0.03 230);
  --radius: 0.5rem;
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-border: var(--border);
}
`
