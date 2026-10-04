// The dev-only shadcn reference server of the visual parity suite
// (REQ-REG-08). It serves one reference page per registry item and renders
// the reference body in the browser, so Radix portals and open overlays are
// real DOM for the captures. Bun and React are repo tools; users never run
// this.
import { readFileSync } from "node:fs"
import { refs } from "./refs"

const shellCSS = readFileSync(new URL("../../../gallery.css", import.meta.url), "utf8")
const refCSS = readFileSync(new URL("../public/ref.css", import.meta.url), "utf8")
const clientEntry = new URL("./client.tsx", import.meta.url).pathname

// bundleClient builds the browser bundle once.
async function bundleClient(): Promise<string> {
  const result = await Bun.build({
    entrypoints: [clientEntry],
    target: "browser",
    minify: true,
    define: { "process.env.NODE_ENV": JSON.stringify("production") },
  })
  if (!result.success) {
    throw new Error(`ref client build: ${result.logs.map(String).join("\n")}`)
  }
  return result.outputs[0].text()
}

let clientJS: Promise<string> | undefined

// renderRef returns the reference document shell of one item. The body is
// rendered by the client.
export function renderRef(name: string, dark: boolean, open: boolean): string {
  if (refs[name] === undefined) throw new Error(`refs: unknown item ${name}`)
  const cls = dark ? "dark" : ""
  return `<!DOCTYPE html>
<html lang="en" class="${cls}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>ref ${name}</title>
<style>${shellCSS}</style>
<style>${refCSS}</style>
</head>
<body>
<section class="fixture" data-ref="${name}">
<h2>ref ${name}</h2>
<div class="fixture-body" data-parity><div id="parity-root"></div></div>
</section>
<script>window.__REF__ = { name: ${JSON.stringify(name)}, open: ${open ? "true" : "false"} }</script>
<script type="module" src="/client.js"></script>
</body>
</html>`
}

// startRef serves the reference app on a random port.
export function startRef(): { url: string; stop: () => void } {
  const server = Bun.serve({
    port: 0,
    async fetch(req) {
      const url = new URL(req.url)
      if (url.pathname === "/") return new Response("ok")
      if (url.pathname === "/client.js") {
        clientJS ??= bundleClient()
        return new Response(await clientJS, {
          headers: { "Content-Type": "text/javascript; charset=utf-8" },
        })
      }
      if (url.pathname.startsWith("/c/")) {
        const name = url.pathname.slice(3)
        const dark = url.searchParams.get("dark") === "1"
        const open = url.searchParams.get("open") === "1"
        try {
          return new Response(renderRef(name, dark, open), {
            headers: { "Content-Type": "text/html; charset=utf-8" },
          })
        } catch (err) {
          return new Response(String(err), { status: 404 })
        }
      }
      return new Response("not found", { status: 404 })
    },
  })
  return { url: `http://127.0.0.1:${server.port}`, stop: () => server.stop(true) }
}

if (import.meta.main) {
  const app = startRef()
  console.log(`ref listening on ${app.url}`)
}
