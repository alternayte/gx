// fetch.ts re-pins the shadcn/ui component sources of the reference app.
// The generated files are committed, so `just parity` never needs the
// network. Run it only when the pin moves:
//
//   cd tools/shadcn-ref && bun run scripts/fetch.ts
//
// The pin is upstream main of 2026-10-02 (D-146); the Gx ports follow the
// new-york-v4 recipes of that snapshot.
const PIN = "295a1f114a138f23b5dfee0e0c6812394dfeb90c"
const BASE = `https://raw.githubusercontent.com/shadcn-ui/ui/${PIN}/apps/v4/registry/new-york-v4`

const components = [
  "accordion", "alert", "alert-dialog", "aspect-ratio", "avatar", "badge",
  "breadcrumb", "button", "button-group", "card", "checkbox", "collapsible",
  "context-menu", "dialog", "drawer", "dropdown-menu", "empty", "field",
  "hover-card", "input", "input-group", "item", "kbd", "label", "menubar",
  "navigation-menu", "pagination", "popover", "progress", "radio-group",
  "scroll-area", "select", "separator", "sheet", "sidebar", "skeleton",
  "slider", "sonner", "spinner", "switch", "table", "tabs", "textarea",
  "toggle", "toggle-group", "tooltip",
]

// rewrite maps the upstream import paths to the local tree.
function rewrite(src: string): string {
  return src
    .replaceAll("@/registry/new-york-v4/ui/", "@/ui/")
    .replaceAll("@/registry/new-york-v4/lib/utils", "@/lib/utils")
    .replaceAll("@/registry/new-york-v4/hooks/use-mobile", "@/hooks/use-mobile")
}

async function get(path: string): Promise<string> {
  const res = await fetch(`${BASE}/${path}`)
  if (!res.ok) throw new Error(`fetch ${path}: ${res.status}`)
  return rewrite(await res.text())
}

const root = new URL("..", import.meta.url).pathname

for (const name of components) {
  const src = await get(`ui/${name}.tsx`)
  await Bun.write(`${root}/src/ui/${name}.tsx`, src)
}
await Bun.write(`${root}/src/lib/utils.ts`, await get("lib/utils.ts"))
await Bun.write(`${root}/src/hooks/use-mobile.ts`, await get("hooks/use-mobile.ts"))
console.log(`fetched ${components.length} components at ${PIN.slice(0, 10)}`)
