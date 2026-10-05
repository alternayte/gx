---
title: "Quick start"
description: "Make an app, run it and change a page in five minutes."
section: Start
order: 1
---

You need Go 1.25 or later. You do not need node.

## Install the command

```sh
go install github.com/alternayte/gx/cmd/gx@latest
```

## Make an app

```sh
gx init acme
cd acme
```

`gx init` asks for the adapter. Press Enter to take Datastar.

The command writes these files.

| File | Content |
| --- | --- |
| `cmd/app/main.go` | The app. It mounts each slice. |
| `cmd/gx/main.go` | The Gx command of this project. `go run ./cmd/gx` uses the Gx version in `go.mod`. |
| `app/Shell.gx`, `app/layout.go` | The layout that wraps each page. |
| `app/theme.css` | The Tailwind theme with the colour tokens. |
| `home/` | An example slice: a page, a component with a signal, and an action. |
| `gx.toml` | The registry and the site data. |
| `AGENTS.md` | The rules and the commands for a coding agent. |

## Run the app

```sh
go run ./cmd/gx dev
```

Open `http://127.0.0.1:3333`. The page shows a counter. **Add one** changes a signal in the browser. **Save** sends the signal to an action on the server.

## Change the page

Change the heading in `home/Home.gx` and save the file.

```gx title="home/Home.gx"
package home

<gx.Head title="Acme" />
<h1 class="text-3xl font-semibold">Hello from acme</h1>
<p class="mt-2 text-muted-foreground">Edit <code>home/Home.gx</code> and save. The page updates in place.</p>
<Counter label="Clicks" />
```

`gx dev` builds the app again and updates the page. The counter keeps its value.

```text title="GET /"
<h1 class="text-3xl font-semibold">Hello from acme</h1>
```

## Check and build

```sh
go run ./cmd/gx check
go run ./cmd/gx build
./bin/app
```

`gx check` finds type errors, unknown props and stale generated code. `gx build` writes one binary to `bin/app`. The binary holds the pages, the scripts and the stylesheet.

## Next steps

- Build a small shop in the [tutorial](/tutorial/a-page/).
- Read the [guides](/guides/components/) for each feature.
- Find a component in the [registry](/components/).
