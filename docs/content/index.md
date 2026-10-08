---
title: "Gx"
description: "Gx is a Go framework for server-rendered web apps on Datastar."
---

Gx types the full loop of a web page: the template, the route, the link, the action, the form and the signal in the browser. A renamed field stops the build. It does not give a 404 at run time.

<docs.CardGrid>
  <docs.LinkCard title="Quick start" href="/start/quick-start/" description="Make an app, run it and change a page in five minutes." />
  <docs.LinkCard title="Tutorial" href="/tutorial/a-page/" description="Build a small shop: pages, an action, a form and a registry component." />
  <docs.LinkCard title="Guides" href="/guides/components/" description="One page for each feature." />
  <docs.LinkCard title="Reference" href="/reference/cli/" description="The command, the gx package, the grammar and each diagnostic." />
  <docs.LinkCard title="Components" href="/components/" description="The registry, with live examples and the code of each one." />
  <docs.LinkCard title="Comparisons" href="/compare/comparisons/" description="What templ, gomponents, Next.js and Rails do better, and what Gx does better." />
</docs.CardGrid>

## What you get

- **Typed templates.** A `.gx` file is HTML with Go expressions. Props have names and types.
- **Typed routes and links.** A route is a Go struct. A link is a value of that struct.
- **Typed actions and forms.** An action answers with typed patches. A form has one rule set for the browser and the server.
- **No node.** The `gx` command and the Go toolchain run each default workflow.
- **One binary.** `gx build` puts the pages, the scripts and the stylesheet in one file.
- **Components that you own.** `gx add` copies a component into the app, and `gx update` merges a later release.
- **Islands.** A TypeScript file is a component with typed props from Go, for the part of a page that needs client code.
- **Widgets.** A component of your app is a custom element on a page of a different site. Your server renders it. Read [Widgets](/guides/widgets/).
- **Tools for agents.** An action with `.Tool()` is a tool that the agent of a user calls, in the browser and over MCP. Read [Actions as tools for agents](/guides/app-tools/).
- **Plugins.** A typed Go value adds directives, commands and build steps to the `gx` command of a project. Read [Plugins](/guides/plugins/).
