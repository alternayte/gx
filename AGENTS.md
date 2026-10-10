# Gx

## What this is

Gx is a Go framework for server-rendered web apps: one typed loop from `.gx` template to route, action, form and client signal, on Datastar.
The spec is `docs/SDD.md`; it and the build state in `docs/build/` stay local and never enter the public repo.

## Run

- `go run ./cmd/gx help` runs the CLI; `gx fmt` formats `.gx` files in place.
- `go build ./...` compiles every package.

## Test

- `just verify` is the gate: build, vet, every test, then the `checks/` scripts.
- `just check` is the same gate under the global name.
- A test name carries the SDD ID it covers, for example `TestREQ_AUT_17_FmtCommand`.
- `just trace` requires a covering test for every PASS ID; `just forbid` fails on stub markers and skipped tests.
- `just evidence` writes `docs/build/evidence.json`; `just evidence-check` fails when it is not for HEAD.

## Stack rules

- `docs/SDD.md` is authoritative. Never edit it.
- Read SDD §17 (build loop), §2.2 (never-list) and §2.4 (decision records) before a change.
- Package `gx` is the whole public API. A new runtime dependency needs a D-entry in `docs/build/decisions.md`.
- Route types live in a `route` subpackage of their slice (DR-01).
- Users never need node. Bun, Playwright and chromedp are for the repo's own e2e, parity and a11y suites only.
- An SDD ID is done only when `just verify` passes at the commit the ledger records.
- Never weaken, skip or delete a test to make it pass.

## Domain words

- slice: one feature package; it owns its routes, views and handlers.
- fragment: a typed, addressable part of a component; patches target it by id.
- signal: a client value scoped to one component instance.
- island: a TypeScript component mounted on the client.
- action: a route type plus a handler that answers with typed patches.
- registry: the component source `gx add` copies into an app.
- document shell: the doctype, html, head and body that the framework writes around a page. Avoid: document wrapper, page skeleton.
- widget: a component that a Gx server renders into a custom element on a page of a different site. Avoid: exported element, exported web component, embed.
- host: the page of a different site that uses a widget; it is not a Gx app. Avoid: consumer, embedder, parent page.
- template value: what generated code returns from a component: its constant static strings and its list of dynamic values. Avoid: render tree, compiled template.
- room: the set of viewers that share one shared signal; its key comes from a loader. Avoid: channel, topic.
- shared signal: a signal whose value each viewer in a room sees; it is not durable. Avoid: synced signal, global signal.
