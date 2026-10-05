---
title: "Security"
description: "Escaping, safe HTML, URLs, cross-site requests, secrets, signal values and the Content Security Policy."
section: Guides
order: 11
---

Gx closes the common holes of a server-rendered app with types and checks. This page lists each rule and the diagnostic that enforces it.

## Output is escaped

Gx escapes each value for its place: text, attribute, URL or style.

Raw HTML needs the type `gx.SafeHTML`. A constant converts with no mark. A value that is not a constant needs `//gx:trusted <reason>` on the same line, or `gx lint` reports [GX7001](/errors/GX7001/).

```go title="app/legal.go"
package app

import "github.com/alternayte/gx"

// notice is HTML that the team wrote. It is a constant.
const notice = "<p>Prices include <abbr title=\"value added tax\">VAT</abbr>.</p>"

// Notice returns the legal notice.
func Notice() gx.SafeHTML {
	return gx.SafeHTML(notice)
}

// Legacy returns HTML from the old system, which the team checked.
func Legacy(html string) gx.SafeHTML {
	return gx.SafeHTML(html) //gx:trusted the old CMS escapes its output
}
```

Markdown content compiles from the files of the repository only. Package `gx` has no function that renders Markdown from a user. Use a sanitizer for user Markdown.

## URLs

`href`, `src`, `action` and `formaction` take a static string, a route value or a `gx.URL` value. A string from data is the diagnostic [GX2011](/errors/GX2011/). Gx cannot write a `javascript:` address.

An HTML event attribute such as `onclick` cannot take an expression. That is [GX2007](/errors/GX2007/).

## Cross-site requests

Each request that is not a `GET` passes the cross-origin check of Go `net/http`. A browser that sends no Fetch Metadata needs a token. The Gx runtime adds the token to each request.

`gx.App` applies the check to each route. For a different router, `gx.CSRF` is the same check as middleware.

## Input

- The binder fills only the fields that the route type declares. It does not fill a field that is not exported or that has the tag `bind:"-"`.
- The browser controls each signal value. An action input with a signal field needs `Rules()` or `gx.Unchecked`, or the check reports [GX4008](/errors/GX4008/).
- A file field has a size limit and a type list. Gx checks the size before the handler runs.

## Secrets

A value of type `gx.Secret` cannot go to the browser. In a signal or in a client expression it is the diagnostic [GX7002](/errors/GX7002/). When it reaches text or JSON by a different path, Gx writes `[redacted]`.

A value from the server in a client expression goes into the page as JSON. Do not put private data in a client expression.

## Content Security Policy

`gx.CSP` is middleware that sends a strict policy with a nonce. The browser then runs only the scripts that have the nonce of the response.

```go title="app/secure.go"
package app

import (
	"net/http"

	"github.com/alternayte/gx"
)

// Secure sends a strict Content Security Policy with each response.
// Datastar evaluates client expressions when the page runs, so the policy
// needs 'unsafe-eval'.
func Secure(next http.Handler) http.Handler {
	return gx.CSP(gx.CSPOptions{
		UnsafeEval: true,
		Directives: "img-src 'self' data:; frame-ancestors 'none'",
	})(next)
}
```

Wrap the app with it: `http.ListenAndServe(addr, app.Secure(server))`.

The policy is `script-src 'nonce-<nonce>' 'strict-dynamic'; object-src 'none'; base-uri 'self'`, plus your directives. Each response has a new nonce.

Gx writes the nonce on each `<script>` element of a `.gx` file, on its own scripts, and on the dev client of `gx dev`. A script inside `gx.SafeHTML` is raw text: write the nonce in it with `gx.Nonce(r)`.

If the app has its own policy middleware, give its nonce to Gx with `gx.WithNonce(r, nonce)`.

## Production builds

A binary from `gx build` has no dev route, no gallery and no dev client. The registry installer checks the hash of each file, and each download has its hash in `gx.lock`.
