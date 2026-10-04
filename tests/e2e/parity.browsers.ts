// Visual parity of every registry component against the dev-only shadcn
// reference rendering (REQ-REG-08). The spec runs under `just parity`.
//
// Each entry of tools/shadcn-ref/parity.json names a gallery fixture and a
// reference rendering. The spec captures the fixture body and the reference
// body in the light, dark and focus states and fails when more than 2% of the
// pixels differ.
import { afterAll, beforeAll, expect, test } from "bun:test"
import { readFileSync } from "node:fs"
import { chromium, type Browser, type Page } from "playwright-core"
import pixelmatch from "pixelmatch"
import { PNG } from "pngjs"
import { startShop, type Shop } from "./harness"

type Entry = {
  ref: string
  fixture: string
  focus?: { gx: string; ref: string }
  // open names the Gx action that shows an overlay before a capture. Use
  // one of gx (a click), js (page script) or focus (keyboard focus).
  open?: { gx?: string; js?: string; focus?: string }
  // capture overrides the captured element. The defaults are the fixture
  // body on the Gx side and the reference body on the reference side.
  capture?: { gx: string; ref: string }
  // threshold raises the per-pixel tolerance for floating content whose
  // subpixel position differs between the two renderings.
  threshold?: number
}

const entries: Entry[] = JSON.parse(
  readFileSync(new URL("../../tools/shadcn-ref/parity.json", import.meta.url), "utf8"),
)

// The dev-only reference app. It is imported, not spawned, so the spec owns
// its lifetime.
const { startRef } = await import("../../tools/shadcn-ref/src/server")

let shop: Shop
let ref: { url: string; stop: () => void }
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop({ gxdev: true })
  ref = startRef()
  browser = await chromium.launch({ channel: "chrome", headless: true })
}), 180000

afterAll(async () => {
  await Bun.sleep(300)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
  ref?.stop()
  shop?.stop()
})

// shoot returns the PNG of one element. It captures through CDP with a
// document-coordinate clip and captureBeyondViewport, because the gallery is
// tall and Playwright's element screenshot can rasterize stale tiles for an
// element far below the fold.
async function shoot(p: Page, selector: string): Promise<Buffer> {
  const el = p.locator(selector).first()
  await el.waitFor({ state: "visible", timeout: 15000 })
  const box = await p.evaluate((s) => {
    const r = document.querySelector(s)!.getBoundingClientRect()
    return {
      x: r.left + window.scrollX,
      y: r.top + window.scrollY,
      width: r.width,
      height: r.height,
    }
  }, selector)
  const client = await p.context().newCDPSession(p)
  try {
    const shot = await client.send("Page.captureScreenshot", {
      format: "png",
      captureBeyondViewport: true,
      clip: { ...box, scale: 1 },
    })
    return Buffer.from(shot.data, "base64")
  } finally {
    await client.detach()
  }
}

// pad grows a PNG to width x height with its own corner colour. The corner is
// the section background, so padding never counts as a changed pixel on its
// own.
function pad(img: PNG, width: number, height: number): PNG {
  if (img.width === width && img.height === height) return img
  const out = new PNG({ width, height })
  const [r, g, b, a] = [img.data[0], img.data[1], img.data[2], img.data[3]]
  for (let i = 0; i < width * height; i++) {
    out.data[i * 4] = r
    out.data[i * 4 + 1] = g
    out.data[i * 4 + 2] = b
    out.data[i * 4 + 3] = a
  }
  PNG.bitblt(img, out, 0, 0, img.width, img.height, 0, 0)
  return out
}

// ratio returns the fraction of pixels that differ between two PNGs.
function ratio(a: Buffer, b: Buffer, threshold = 0.1): { ratio: number; diff: Buffer } {
  const left = PNG.sync.read(a)
  const right = PNG.sync.read(b)
  const width = Math.max(left.width, right.width)
  const height = Math.max(left.height, right.height)
  const la = pad(left, width, height)
  const rb = pad(right, width, height)
  const diff = new PNG({ width, height })
  const n = pixelmatch(la.data, rb.data, diff.data, width, height, { threshold })
  return { ratio: n / (width * height), diff: PNG.sync.write(diff) }
}

// freeze disables transitions and animations so a captured state is the
// settled state, the same on both pages.
async function freeze(p: Page): Promise<void> {
  await p.addStyleTag({
    content: "*,*::before,*::after{transition:none!important;animation:none!important}",
  })
}

// applyDark turns the dark theme on for both pages.
async function applyDark(p: Page, dark: boolean): Promise<void> {
  await p.evaluate((on) => {
    document.documentElement.classList.toggle("dark", on)
    document.documentElement.classList.toggle("light", !on)
  }, dark)
}

// focusElement focuses the element the way a keyboard user reaches it, so
// :focus-visible matches on both sides.
async function focusElement(p: Page, selector: string): Promise<void> {
  await p.keyboard.press("Tab")
  await p.evaluate((sel) => {
    const el = document.querySelector(sel)
    if (el instanceof HTMLElement) el.focus()
  }, selector)
  const visible = await p.evaluate(
    (sel) => document.querySelector(sel)?.matches(":focus-visible") ?? false,
    selector,
  )
  if (!visible) throw new Error(`focus: ${selector} did not match :focus-visible`)
}

// compare captures the same state on both pages and returns the diff ratio.
async function compare(entry: Entry, state: "light" | "dark" | "focus"): Promise<{ ratio: number; detail: string }> {
  await page.goto(shop.url + "/_gx/gallery")
  await page.waitForSelector(`section[data-fixture="${entry.fixture}"]`)
  await freeze(page)
  const params: string[] = []
  if (state === "dark") params.push("dark=1")
  if (entry.open) params.push("open=1")
  const refURL = ref.url + "/c/" + entry.ref + (params.length > 0 ? "?" + params.join("&") : "")
  const gxSelector = entry.capture?.gx ?? `section[data-fixture="${entry.fixture}"] .fixture-body`
  const refSelector = entry.capture?.ref ?? "[data-parity]"
  const bodySelector = `section[data-fixture="${entry.fixture}"] .fixture-body`

  if (state === "dark") {
    await applyDark(page, true)
  }
  if (entry.open) {
    if (entry.open.js) await page.evaluate(entry.open.js)
    if (entry.open.gx) {
      await page.locator(`${bodySelector} ${entry.open.gx}`).first().click()
    }
    if (entry.open.focus) {
      await page.locator(`${bodySelector} ${entry.open.focus}`).first().focus()
    }
    await page.locator(gxSelector).first().waitFor({ state: "visible", timeout: 15000 })
  }
  if (state === "focus") {
    await focusElement(page, `${gxSelector} ${entry.focus!.gx}`)
  }
  const gxShot = await shoot(page, gxSelector)

  const refPage = await browser.newPage()
  try {
    await refPage.goto(refURL)
    await refPage.waitForSelector("html[data-ready='1']")
    await freeze(refPage)
    if (state === "dark") await applyDark(refPage, true)
    if (state === "focus") await focusElement(refPage, `${refSelector} ${entry.focus!.ref}`)
    const refShot = await shoot(refPage, refSelector)
    const r = ratio(gxShot, refShot, entry.threshold)
    if (r.ratio >= 0.02) {
      // Keep the failing captures for triage: gx, ref and the changed pixels.
      const dir = "/tmp/gx-parity"
      await Bun.write(`${dir}/${entry.ref}-${state}-gx.png`, gxShot)
      await Bun.write(`${dir}/${entry.ref}-${state}-ref.png`, refShot)
      await Bun.write(`${dir}/${entry.ref}-${state}-diff.png`, r.diff)
    }
    return { ratio: r.ratio, detail: `${(r.ratio * 100).toFixed(1)}% changed` }
  } finally {
    await refPage.close()
  }
}

for (const entry of entries) {
  test(`REQ-REG-08 ${entry.ref}: light, dark and focus match the shadcn reference`, async () => {
    page ??= await browser.newPage()
    const light = await compare(entry, "light")
    expect(light.ratio, `${entry.ref} light ${light.detail}`).toBeLessThan(0.02)
    const dark = await compare(entry, "dark")
    expect(dark.ratio, `${entry.ref} dark ${dark.detail}`).toBeLessThan(0.02)
    if (entry.focus) {
      const focus = await compare(entry, "focus")
      expect(focus.ratio, `${entry.ref} focus ${focus.detail}`).toBeLessThan(0.02)
    }
  }, 120000)
}
