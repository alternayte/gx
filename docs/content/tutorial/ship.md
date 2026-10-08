---
title: "6. Check, build and ship"
description: "Run the checks, build one binary and know what a static export can hold."
section: Tutorial
order: 6
sample: tutorial
---

## Check the app

```sh
gx check
gx lint
```

`gx check` reads each `.gx` file, the routes, the actions and the forms. It also fails when a generated file is stale. Run it before each commit.

`gx lint` runs `go vet` and the Gx analyzers.

## See the app as data

```sh
gx routes
gx describe --json
```

`gx routes` lists each route with its page, its layouts and its fields. `gx describe --json` prints the full model of the app: components, props, signals, fragments, routes, actions and forms with their rules. A coding agent reads this output.

## Build one binary

```sh
gx build
./bin/app
```

`gx build` generates the code, builds the stylesheet with the pinned Tailwind binary, and builds the app. The binary holds each page, each script and the stylesheet. Copy the one file to a server and run it.

The app listens on `127.0.0.1:8080`. Change the address in `cmd/app/main.go`.

## A static export

`gx export` renders each page to a file. It is for a site with no server, such as a docs site.

The shop has an action and a form. They need a server, so the export stops and lists them.

```text title="gx export"
gx export: these features need a server. A static host cannot run them.
  action           POST /save
  action           POST /shop/{id}/cart
  form             POST /shop/order
  form on a page   /shop/order
  live validation  /shop/order
Mark an action that another server answers with .External(url). Remove the other features from the exported pages.
```

The export writes nothing in this case. Read [static export](/guides/static-export/) for a site that it can hold.

## What you built

- Pages with typed routes, typed links and typed props.
- A signal in the browser and an action that patches one fragment.
- A form with one rule set for the browser and the server.
- A registry component that you own.
- One binary with no node in the build.
