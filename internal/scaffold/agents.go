package scaffold

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alternayte/gx/internal/gxconfig"
)

// The managed section of AGENTS.md sits between these two lines. `gx agents
// --update` replaces what is between them and nothing else (REQ-AI-05).
const (
	agentsStart = "<!-- gx:agents:start - gx agents --update rewrites this section. Write your own rules outside it. -->"
	agentsEnd   = "<!-- gx:agents:end -->"
)

// agentsManaged is the managed section: conventions, commands and the
// never-list for app code.
const agentsManaged = `## Gx

This app uses Gx: typed server-rendered pages in Go, on «adapter».

### Commands

- ` + "`go run ./cmd/gx dev`" + ` runs the app with rebuild and reload.
- ` + "`go run ./cmd/gx check`" + ` checks every ` + "`.gx`" + ` file and fails on stale generated code. Run it before each commit.
- ` + "`go run ./cmd/gx check --json`" + ` prints the diagnostics for a tool. Each one has a code ` + "`GXnnnn`" + `.
- ` + "`go run ./cmd/gx generate`" + ` writes the ` + "`_gx.go`" + ` files. Commit them.
- ` + "`go run ./cmd/gx fmt <file>`" + ` formats a ` + "`.gx`" + ` file.
- ` + "`go run ./cmd/gx lint`" + ` runs ` + "`go vet`" + ` and the Gx analyzers.
- ` + "`go run ./cmd/gx build`" + ` builds one binary.
- ` + "`go run ./cmd/gx routes`" + ` lists the routes.
- ` + "`go run ./cmd/gx describe --json`" + ` prints the full app model: components, routes, actions and forms.
- ` + "`go run ./cmd/gx new page|action|form|component|slice <name>`" + ` adds typed code.
- ` + "`go run ./cmd/gx add <item>`" + ` installs a registry component under ` + "`ui/`" + `.

### Conventions

- One component per ` + "`.gx`" + ` file. The file name is the component name.
- A slice is one feature package. It owns its routes, views and handlers.
- Route types live in the ` + "`route`" + ` subpackage of their slice. That package holds only route types.
- A link is a route value: ` + "`href={route.Show{ID: 1}}`" + `.
- An action is a route value: ` + "`on:click={route.Add{}}`" + `.
- Put every page, action and form in the ` + "`gx.Collect`" + ` call of its slice. Put every slice in the ` + "`Group`" + ` call in ` + "`cmd/app/main.go`" + `.
- Props have names. A prop with no default is required.
- A loader does the IO and returns props. A component renders from its props only.
«state»
- A class is a static string. ` + "`gx.Cx`" + ` merges classes and ` + "`gx.Enum`" + ` holds variants.
- A component has a ` + "`<Name>.fixtures.go`" + ` file. The dev gallery at ` + "`/_gx/gallery`" + ` shows each fixture.

### Never

- Never edit a ` + "`_gx.go`" + ` file. Edit the ` + "`.gx`" + ` file and run ` + "`gx generate`" + `.
- Never put a dynamic string in ` + "`href`, `src`, `action` or `formaction`" + `. Use a route value or ` + "`gx.URL`" + `.
- Never build a class string at runtime.
- Never convert a non-constant string to ` + "`gx.SafeHTML`" + ` without ` + "`//gx:trusted <reason>`" + ` on the same line.
«never»- Never do IO in a ` + "`.gx`" + ` file.
- Never register a route or a component in ` + "`init()`" + `.
- Never add node or npm for a default workflow.
`

// The lines of the managed section that depend on the adapter
// (REQ-ACT-09). htmx has no signals, so its lines name the server state.
var agentsLines = map[string][3]string{
	gxconfig.AdapterDatastar: {
		"Datastar",
		"- Client state is a signal in the `signals` block. `$Name` works only in a client expression.",
		"- Never put a `gx.Secret` value in a signal or a client expression.\n- Never trust a signal value. Give the action input `Rules()`.\n",
	},
	gxconfig.AdapterHtmx: {
		"htmx",
		"- State lives on the server. The htmx adapter has no signals and no client expressions (GX4006). An action patches a fragment.",
		"- Never trust a request value. Give the action input `Rules()`.\n",
	},
}

// agentsBlock returns the managed section with its marker lines, for the
// adapter of the app.
func agentsBlock(adapter string) string {
	lines, ok := agentsLines[adapter]
	if !ok {
		lines = agentsLines[gxconfig.AdapterDatastar]
	}
	managed := strings.NewReplacer("«adapter»", lines[0], "«state»", lines[1], "«never»", lines[2]).Replace(agentsManaged)
	return agentsStart + "\n" + managed + agentsEnd + "\n"
}

// Agents returns the AGENTS.md of a new app with the adapter (REQ-AI-05).
func Agents(name, adapter string) string {
	return "# " + name + "\n\n" + agentsBlock(adapter) + "\n## Project notes\n\nWrite the rules of this project here. `gx agents --update` keeps this part.\n"
}

// UpdateAgents rewrites the managed section of AGENTS.md in dir and leaves
// every other byte alone (REQ-AI-05). It reports whether the file changed.
// A file with no managed section gets one at its end, and a missing file is
// written new.
func UpdateAgents(dir string) (bool, error) {
	path := filepath.Join(dir, "AGENTS.md")
	cfg, err := gxconfig.Load(dir)
	if err != nil {
		return false, err
	}
	adapter := cfg.Adapter
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		abs, absErr := filepath.Abs(dir)
		if absErr != nil {
			return false, absErr
		}
		return true, os.WriteFile(path, []byte(Agents(appName(filepath.Base(abs)), adapter)), 0o644)
	}
	if err != nil {
		return false, err
	}
	old := string(data)
	var next string
	start := strings.Index(old, "<!-- gx:agents:start")
	end := strings.Index(old, agentsEnd)
	switch {
	case start >= 0 && end > start:
		tail := old[end+len(agentsEnd):]
		tail = strings.TrimPrefix(strings.TrimPrefix(tail, "\r"), "\n")
		next = old[:start] + agentsBlock(adapter) + tail
	default:
		next = strings.TrimRight(old, "\n") + "\n\n" + agentsBlock(adapter)
		if strings.TrimSpace(old) == "" {
			next = agentsBlock(adapter)
		}
	}
	if next == old {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(next), 0o644)
}
