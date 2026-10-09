// Visual parity of examples/deedbox-docs with the Deedbox site, which uses
// the default Starlight theme (F-92). The reference is tests/e2e/starlight-ref:
// pages of the built Starlight site. The suite renders the same page of the
// two sites in one browser, in light and dark, and fails when the share of
// changed pixels is above the budget of the page.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser } from 'playwright-core'
import pixelmatch from 'pixelmatch'
import { PNG } from 'pngjs'
import { freePort, launchBrowser } from './harness'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const appDir = join(repo, 'examples', 'deedbox-docs')
const refDir = fileURLToPath(new URL('./starlight-ref', import.meta.url))

type Entry = {
  name: string
  path: string
  // gxPath is the address of the page on the Gx site, when it is not path.
  gxPath?: string
  width: number
  height: number
  // budget is the highest share of changed pixels, in percent. The code
  // highlighter of Gx gives some tokens a different class than the one of
  // Starlight, so a page with much code has a higher number.
  budget: number
}

// The pages of scripts/starlight-ref.sh. The measured numbers are 0.00 to
// 0.37 on macOS (D-301); the budget has a margin for the text rendering of a
// different machine.
const entries: Entry[] = [
  { name: 'home', path: '/', width: 1440, height: 900, budget: 1 },
  { name: 'tutorial', path: '/tutorials/first-stream/', width: 1440, height: 900, budget: 1 },
  { name: 'how-to', path: '/how-to/add-a-projection/', width: 1440, height: 900, budget: 1 },
  { name: 'reference', path: '/reference/configuration/', width: 1440, height: 900, budget: 1 },
  { name: 'error', path: '/reference/errors/dbx001/', width: 1440, height: 900, budget: 1 },
  { name: 'not-found', path: '/404.html', width: 1440, height: 900, budget: 1 },
  { name: 'tablet', path: '/tutorials/first-stream/', width: 900, height: 900, budget: 1 },
  { name: 'phone', path: '/tutorials/first-stream/', width: 390, height: 844, budget: 1 },
  { name: 'phone-home', path: '/', width: 390, height: 844, budget: 1 },
]

let dir: string
let servers: ReturnType<typeof Bun.serve>[] = []
let browser: Browser
let gxURL: string
let refURL: string

async function run(argv: string[], cwd: string): Promise<void> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdout: 'pipe', stderr: 'pipe' })
  const code = await proc.exited
  if (code !== 0) {
    const err = await new Response(proc.stderr).text()
    throw new Error(`${argv.join(' ')} failed: ${err}`)
  }
}

async function serve(root: string): Promise<string> {
  const port = await freePort()
  servers.push(
    Bun.serve({
      port,
      fetch: async (req) => {
        let pathname = decodeURIComponent(new URL(req.url).pathname)
        if (pathname.endsWith('/')) pathname += 'index.html'
        const file = Bun.file(join(root, pathname))
        if (!(await file.exists())) return new Response('not found', { status: 404 })
        return new Response(file)
      },
    }),
  )
  return `http://127.0.0.1:${port}`
}

beforeAll(async () => {
  // GX_STARLIGHT_DIST=<dir> compares an export that exists, to tune the theme.
  let dist = process.env.GX_STARLIGHT_DIST ?? ''
  if (dist === '') {
    dir = mkdtempSync(join(tmpdir(), 'gx-starlight-'))
    cpSync(appDir, dir, { recursive: true })
    const goMod = join(dir, 'go.mod')
    writeFileSync(goMod, readFileSync(goMod, 'utf8').replace('replace github.com/alternayte/gx => ../..', `replace github.com/alternayte/gx => ${repo}`))
    await run(['go', 'mod', 'tidy'], dir)
    dist = join(dir, 'dist')
    await run(['go', 'run', './cmd/gx', 'export', '-main', '.', '--out', dist, dir], repo)
  }
  gxURL = await serve(dist)
  refURL = await serve(refDir)
  browser = await launchBrowser()
}, 300000)

afterAll(async () => {
  await Bun.sleep(200)
  for (const server of servers) server.stop(true)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

// freeze turns off transitions and the caret, so a capture is the settled
// state on both pages.
const freeze = `*, *::before, *::after { transition: none !important; animation: none !important; caret-color: transparent !important; }`

async function shoot(url: string, entry: Entry, scheme: 'light' | 'dark'): Promise<Buffer> {
  const context = await browser.newContext({
    viewport: { width: entry.width, height: entry.height },
    colorScheme: scheme,
    deviceScaleFactor: 1,
  })
  try {
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'load' })
    await page.addStyleTag({ content: freeze })
    await page.evaluate(() => document.fonts.ready)
    return await page.screenshot({ fullPage: true })
  } finally {
    await context.close()
  }
}

// pad grows a PNG to width x height with the colour of its last pixel: the
// page background.
function pad(img: PNG, width: number, height: number): PNG {
  if (img.width === width && img.height === height) return img
  const out = new PNG({ width, height })
  const at = (img.width * img.height - 1) * 4
  for (let i = 0; i < out.data.length; i += 4) {
    out.data[i] = img.data[at]
    out.data[i + 1] = img.data[at + 1]
    out.data[i + 2] = img.data[at + 2]
    out.data[i + 3] = 255
  }
  PNG.bitblt(img, out, 0, 0, img.width, img.height, 0, 0)
  return out
}

function changed(a: Buffer, b: Buffer): { percent: number; diff: Buffer; heights: [number, number] } {
  const left = PNG.sync.read(a)
  const right = PNG.sync.read(b)
  const width = Math.max(left.width, right.width)
  const height = Math.max(left.height, right.height)
  const diff = new PNG({ width, height })
  const n = pixelmatch(pad(left, width, height).data, pad(right, width, height).data, diff.data, width, height, { threshold: 0.1 })
  return { percent: (n / (width * height)) * 100, diff: PNG.sync.write(diff), heights: [left.height, right.height] }
}

for (const entry of entries) {
  for (const scheme of ['light', 'dark'] as const) {
    test(`F-92 ${entry.name} in ${scheme} matches the Starlight page`, async () => {
      const gx = await shoot(gxURL + (entry.gxPath ?? entry.path), entry, scheme)
      const ref = await shoot(refURL + entry.path, entry, scheme)
      const r = changed(gx, ref)
      // GX_STARLIGHT_REPORT=<dir> prints each number and keeps each capture.
      const report = process.env.GX_STARLIGHT_REPORT
      const keep = report ?? (r.percent > entry.budget ? '/tmp/gx-parity-starlight' : '')
      if (keep) {
        mkdirSync(keep, { recursive: true })
        await Bun.write(`${keep}/${entry.name}-${scheme}-gx.png`, gx)
        await Bun.write(`${keep}/${entry.name}-${scheme}-ref.png`, ref)
        await Bun.write(`${keep}/${entry.name}-${scheme}-diff.png`, r.diff)
      }
      if (report) console.log(`starlight ${entry.name} ${scheme} ${r.percent.toFixed(2)} heights ${r.heights[0]} ${r.heights[1]}`)
      expect(r.percent).toBeLessThanOrEqual(entry.budget)
    }, 120000)
  }
}
