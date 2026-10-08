---
title: "Plugins"
description: "A plugin is a typed Go value that the gx command of a project compiles in: directives, an adapter, commands, build steps, registries, MCP tools and themes."
section: Guides
order: 17
---

A plugin adds to the `gx` command of one project. A plugin is a Go value. The `cmd/gx/main.go` of the project gives it to the command, so the Go compiler checks it. No code loads at run time, and no `init` function registers a plugin.

Plugins arrive with release 0.3.0.

A plugin is for the build of an app: the compiler, the commands and the dev tools. The running app needs no plugin system. Middleware, components and Go packages add to it.

## Add a plugin to a project

`gx init` writes `cmd/gx/main.go`. Give each plugin to `gxcli.WithPlugins`.

```go title="cmd/gx/main.go"
// Command gx is the Gx command line tool of this project.
package main

import (
	"os"

	"acme/tools/stamp"

	"github.com/alternayte/gx/gxcli"
)

func main() {
	os.Exit(gxcli.Main(os.Args[1:], gxcli.WithPlugins(stamp.Plugin())))
}
```

Run a command with `gx <command>`. In a project with a `cmd/gx`, the `gx` command that you installed builds that command and runs it. `go run ./cmd/gx <command>` does the same with no installed command. Each developer then has the plugins and the Gx version of the project.

## Write a plugin

A plugin has a name and the version of the plugin API that it was built for. It adds to Gx through the hook interfaces that it also implements.

```go title="tools/stamp/stamp.go"
// Package stamp is a plugin of the acme project.
package stamp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alternayte/gx/gxcli"
)

type plugin struct{}

// Plugin returns the plugin.
func Plugin() gxcli.Plugin { return plugin{} }

func (plugin) Name() string { return "stamp" }

// API is the plugin API that this plugin was built for.
func (plugin) API() int { return 1 }

// Directives adds stamp:label. The compiler puts a data attribute in its
// place.
func (plugin) Directives() []gxcli.Directive {
	return []gxcli.Directive{{
		Name: "stamp:label",
		Transform: func(use gxcli.DirectiveUse) ([]gxcli.DirectiveAttr, error) {
			if !use.HasValue || use.Value == "" {
				return nil, errors.New("the label needs a value, for example stamp:label=\"draft\"")
			}
			return []gxcli.DirectiveAttr{{Name: "data-stamp", Value: use.Value}}, nil
		},
	}}
}

// Commands adds gx stamp.
func (plugin) Commands() []gxcli.Command {
	return []gxcli.Command{{
		Name:    "stamp",
		Summary: "print the name of the project",
		Run: func(args []string) int {
			fmt.Println("acme")
			return 0
		},
	}}
}

// BuildSteps adds one step to gx build.
func (plugin) BuildSteps() []gxcli.BuildStep {
	return []gxcli.BuildStep{{
		Name: "stamp",
		Run: func(ctx context.Context, b gxcli.BuildInfo) error {
			return os.WriteFile(filepath.Join(b.Root, "stamp.txt"), []byte("built by acme\n"), 0o644)
		},
	}}
}
```

```gx title="home/Badge.gx"
package home

<span class="rounded border px-2" stamp:label="draft">Draft</span>
```

## The hooks

| Hook | Method | What it adds |
| --- | --- | --- |
| `gxcli.Directives` | `Directives()` | Directives for `.gx` files, with the namespace of the plugin. |
| `gxcli.Adapter` | `Adapter()` | The name of a hypermedia adapter for `gx.toml`, and what the adapter can express. |
| `gxcli.Commands` | `Commands()` | Commands of `gx`. `gx help` lists them. |
| `gxcli.BuildSteps` | `BuildSteps()` | Steps of `gx build`, after the generated files and before the Go build. |
| `gxcli.Registries` | `Registries()` | Component registries: `gx add @name/item`. |
| `gxcli.MCPTools` | `MCPTools()` | Tools of the dev MCP server of `gx mcp`. |
| `gxcli.ThemePresets` | `ThemePresets()` | Themes: `gx add theme:<name>` writes one as `app/theme.css`. |

The repository of Gx has an example plugin with each hook: `examples/chartzoom`.

## Directives

A directive of a plugin is an attribute with a namespace: `stamp:label="draft"`. The namespace keeps the directive apart from an HTML attribute and from the directives of Gx. A plugin with a directive that has no namespace does not load.

A directive runs at compile time. The compiler calls `Transform` for each use and puts the returned attributes in the place of the directive. The value is static: a directive with an expression is a diagnostic. An error of `Transform` is a diagnostic at the place of the use.

A directive writes plain attributes. It cannot write an event attribute, an address or a directive.

A directive can have a `Behavior`: an ES module. A page with the directive loads the module, and the module finds its elements by the attributes of `Transform`. The module is in the bundle of the app, as an island is.

A plugin cannot change the grammar of a `.gx` file. No hook takes part in the parse.

## The version of the plugin API

`gxcli.PluginAPI` is the version of the plugin API of a Gx version. A plugin returns the version that it was built for from `API()`, as a number in its code. A Gx with a different version stops each command and names the plugin and the two versions. A plugin thus fails with a clear error and not with a wrong build.

## The load checks

The command checks each plugin before it runs. It stops for these problems:

- A plugin has no name, or two plugins have one name.
- A command, a directive, an adapter or an MCP tool takes a name of Gx.
- Two plugins give one name to a command, a directive, an adapter, a registry, an MCP tool or a theme.

A registry in `gx.toml` goes before a registry of a plugin with the same name.
