# acme

<!-- gx:agents:start - gx agents --update rewrites this section. Write your own rules outside it. -->
## Gx

This app uses Gx: typed server-rendered pages in Go, on Datastar.

### Commands

- `go run ./cmd/gx dev` runs the app with rebuild and reload.
- `go run ./cmd/gx check` checks every `.gx` file and fails on stale generated code. Run it before each commit.
- `go run ./cmd/gx check --json` prints the diagnostics for a tool. Each one has a code `GXnnnn`.
- `go run ./cmd/gx generate` writes the `_gx.go` files. Commit them.
- `go run ./cmd/gx fmt <file>` formats a `.gx` file.
- `go run ./cmd/gx lint` runs `go vet` and the Gx analyzers.
- `go run ./cmd/gx build` builds one binary.
- `go run ./cmd/gx routes` lists the routes.
- `go run ./cmd/gx describe --json` prints the full app model: components, routes, actions and forms.
- `go run ./cmd/gx new page|action|form|component|slice <name>` adds typed code.
- `go run ./cmd/gx add <item>` installs a registry component under `ui/`.

### Conventions

- One component per `.gx` file. The file name is the component name.
- A slice is one feature package. It owns its routes, views and handlers.
- Route types live in the `route` subpackage of their slice. That package holds only route types.
- A link is a route value: `href={route.Show{ID: 1}}`.
- An action is a route value: `on:click={route.Add{}}`.
- Put every page, action and form in the `gx.Collect` call of its slice. Put every slice in the `Group` call in `cmd/app/main.go`.
- Props have names. A prop with no default is required.
- A loader does the IO and returns props. A component renders from its props only.
- Client state is a signal in the `signals` block. `$Name` works only in a client expression.
- A class is a static string. `gx.Cx` merges classes and `gx.Enum` holds variants.
- A component has a `<Name>.fixtures.go` file. The dev gallery at `/_gx/gallery` shows each fixture.

### Never

- Never edit a `_gx.go` file. Edit the `.gx` file and run `gx generate`.
- Never put a dynamic string in `href`, `src`, `action` or `formaction`. Use a route value or `gx.URL`.
- Never build a class string at runtime.
- Never convert a non-constant string to `gx.SafeHTML` without `//gx:trusted <reason>` on the same line.
- Never put a `gx.Secret` value in a signal or a client expression.
- Never trust a signal value. Give the action input `Rules()`.
- Never do IO in a `.gx` file.
- Never register a route or a component in `init()`.
- Never add node or npm for a default workflow.
<!-- gx:agents:end -->

## Project notes

Write the rules of this project here. `gx agents --update` keeps this part.
