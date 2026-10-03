# Gx

## What this is

Gx is a Go framework for server-rendered web apps: one typed loop from `.gx` template to route, action, form and client signal, on Datastar.
The spec is `docs/SDD.md`; it and the build state in `docs/build/` stay local and never enter the public repo.

## Run

No code yet. The first build session starts at M0 and writes the module and the justfile (SDD §15.1).

## Test

No tests yet. The SDD names `just verify` as the gate; M0 writes the justfile.

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
