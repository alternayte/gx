# Gx

A Go framework for server-rendered web apps. Gx types the full loop:
template, route, link, action, form, client signal and island. It runs on
`net/http` and Datastar, with no node in any default workflow.

**Status: pre-release.** The 0.1.0 feature set is under construction. The
`gx` command, the language server and the example apps work; `gx init` and
the release gate are still to come. The specification is local to the
maintainer's checkout and is not in this repository.

## Layout

- `gx.go`, `route.go`, `action.go`, `form.go`, `patch.go`: package `gx`,
  the public runtime API.
- `internal/compiler`: the `.gx` compiler, checks, code generation and the
  content pipeline.
- `cmd/gx`, `gxcli`: the CLI.
- `runtime/js`: the browser runtime (TypeScript, built with Bun).
- `registry`: the component registry source and the published items.
- `examples/shop`: the reference app. `examples/deedbox-docs`: the docs
  site, a port of the Deedbox documentation (MIT).
- `docs/`: docs content, the grammar and every diagnostic code.
- `tests/e2e`: the browser suite for the repo only.

## Try it

    go run ./cmd/gx help

Write a component in `ui/card/Card.gx`:

```go
package card

props {
  Title string
}

<article class="rounded-xl border p-4">
  <h3>{p.Title}</h3>
</article>
```

    go run ./cmd/gx generate .
    go run ./cmd/gx dev -main .

The `examples/shop` and `examples/deedbox-docs` modules show routes,
actions, forms, signals, the docs shell, search and static export.

## Build and test

    go build ./...
    just verify

`just verify` is the gate: build, vet, the browser runtime check, every Go
test, the `checks/` scripts and the browser suite. It needs Go, Bun,
Chrome, and network access on a cold cache for the pinned Tailwind and
Pagefind downloads. `just check` is an alias.

## Licence

MIT.
