---
title: "Comparisons"
description: "What templ, gomponents, Next.js and Rails with Hotwire do better than Gx, and what Gx does better."
section: Compare
order: 1
---

Gx is new. Release 0.1.0 is the first release. Each tool on this page is older, has more users and has more answers on the web. This page names what each one does better, so that you can choose.

## What Gx does not have

- A stable 1.0 API. A 0.x release can change an API.
- Signals with htmx. Gx has a Datastar adapter and, from release 0.2.0, an htmx adapter. Signals and client expressions need Datastar.
- A client component tree. Gx has TypeScript islands from release 0.2.0. It has no React, Vue or Svelte component tree.
- A data layer. Gx is the web layer. It has no ORM, no auth, no jobs and no mailer.
- A build with no code generation. Each `.gx` file has a generated Go file that you commit.
- A strict Content Security Policy with Datastar. Datastar needs `unsafe-eval`.

## templ

[templ](https://templ.guide) is a language for HTML components in Go, with code generation and a language server.

**What templ does better**

- It is mature and has many users. You find examples, answers and integrations for it.
- A templ file takes any Go statement. Gx allows `if`, `for`, `switch` and `:=` only.
- Its language server uses `gopls` for the Go parts, so each `gopls` feature works. Gx uses its own type index.
- It does one thing. It has no opinion about routes, forms or the browser, so it fits any stack.
- It has no browser runtime of its own.

**What Gx does better**

- Routes, links, actions, forms and signals are typed. In templ, a URL and an htmx target are strings.
- Props have names, and a required prop is checked. A templ component takes arguments by position.
- A component registry with three-way updates.
- One dev command with page morph, the error overlay and the stylesheet build.

**Choose templ when** you want only a template language, or when you need a tool that is in production in many companies today.

## gomponents

[gomponents](https://www.gomponents.com) builds HTML with plain Go functions.

**What gomponents does better**

- It is only Go. There is no new file type, no code generation and no command to install.
- Each Go tool works with no extra step: `gopls`, the debugger, refactoring and coverage.
- A component is a function. There is nothing more to learn.
- The dependency is very small.

**What Gx does better**

- A page with much markup reads as HTML. You can paste an HTML block into a `.gx` file.
- The typed loop from route to action to form, the registry and the dev loop.

**Choose gomponents when** your pages have little markup, or when you want no build step and no tool.

## Next.js

[Next.js](https://nextjs.org) is a full React framework with server rendering.

**What Next.js does better**

- The React ecosystem. A component exists for almost each need.
- Rich client state and interaction: drag and drop, editors, charts, offline use.
- Streaming, partial rendering and edge runtimes.
- A very large community, many hosts with one-step deploys, and many developers who know it.

**What Gx does better**

- No node, no bundler and no hydration. The build gives one Go binary.
- One language and one type system from the database call to the template.
- A page with no interactive part ships no JavaScript.
- The server and client boundary is small and explicit: client code is a client expression or nothing.

**Choose Next.js when** the product is a rich client application, or when the team works in React.

## Rails with Hotwire

[Rails](https://rubyonrails.org) with [Hotwire](https://hotwired.dev) is a full-stack framework that sends HTML over the wire.

**What Rails does better**

- It is a full stack: models, migrations, jobs, mail, auth generators and a console. Gx is the web layer only.
- Twenty years of conventions, gems, guides and hosts.
- Turbo Native takes the same HTML into a mobile app.
- A generator makes a working resource in one command.

**What Gx does better**

- Static types. A renamed field, route or fragment stops the build. In Rails, a Turbo Stream target is a string.
- One static binary with a low memory use and a fast start.
- Signals in the browser with no extra controller file.

**Choose Rails when** you want one framework for the whole product and the team likes Ruby.

## Which one

| You want | Use |
| --- | --- |
| Typed templates only, in production today | templ |
| No tool and no build step | gomponents |
| A rich client application | Next.js |
| One framework for the whole stack | Rails with Hotwire |
| A typed web layer in Go with a dev loop and components, and you accept a 0.x release | Gx |
