# Content Security Policy

Gx can send a strict Content Security Policy (CSP) with a nonce. The browser
then runs only the scripts that carry the nonce of the response.

## Turn the policy on

`gx.CSP` is middleware. Put it around the app.

```go
app := gx.New(gx.Config{Adapter: datastar.Adapter()})
app.Group("/", shop.Routes)

handler := gx.CSP(gx.CSPOptions{UnsafeEval: true})(app)
log.Fatal(http.ListenAndServe(addr, handler))
```

You can also put it in one group.

```go
app.Group("/", gx.CSP(gx.CSPOptions{UnsafeEval: true}), shop.Routes)
```

Each response gets a new nonce. The policy is:

```text
script-src 'nonce-<nonce>' 'strict-dynamic'; object-src 'none'; base-uri 'self'
```

`CSPOptions.Directives` adds your own directives, for example
`"img-src 'self' data:; frame-ancestors 'none'"`.

## What carries the nonce

- Every `<script>` element in a `.gx` file.
- The Gx runtime, the adapter runtime, the behaviour modules and the theme
  script.
- The dev client that `gx dev` adds to a page.

A script inside `gx.SafeHTML` is raw text. Gx cannot change it. Write the
nonce yourself with `gx.Nonce(r)`.

## Datastar needs `unsafe-eval`

Datastar evaluates client expressions when the page runs. The browser calls
this `eval`. A policy without `'unsafe-eval'` stops every signal, every
`show` and every `on:` handler.

Set `UnsafeEval: true` when the app uses the Datastar adapter. The policy
still blocks inline scripts with no nonce and inline event attributes.

## Use your own policy middleware

If the app already has CSP middleware, give its nonce to Gx. This also works
for `gx.Render` in a plain handler.

```go
func page(w http.ResponseWriter, r *http.Request) {
  r = gx.WithNonce(r, nonceFrom(r))
  gx.Render(w, r, cart.Cart(props))
}
```
