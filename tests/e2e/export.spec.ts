// The exported static site in a real browser: content images carry their
// size, so the page does not shift (REQ-CNT-11).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { cpSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const appDir = join(repo, 'tests', 'e2e', 'shellapp')
const shellSrc = join(repo, 'registry', 'docs-shell')

// dotPNG is a 1200x600 PNG, base64 encoded.
const dotPNG = atob('iVBORw0KGgoAAAANSUhEUgAABLAAAAJYCAIAAAD9hIhNAAANdklEQVR4nOzZwQkCQRAF0RYmqU3F/AMRjMGhwXqPz57n0Jdiz7znNWY/3rP9APvLuSu7MXdlN+au7Mbcld3Y+X4BAADIEYQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAos7Ms/0GAAAAFvhDCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAESdmWf7DQAAACzwhxAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACIEoQAAABRghAAACBKEAIAAEQJQgAAgChBCAAAECUIAQAAogQhAABAlCAEAACI+gQAAP//xboSSa6VvisAAAAASUVORK5CYII=')

let dir: string
let dist: string
let server: ReturnType<typeof Bun.serve>
let browser: Browser
let page: Page
let url: string

async function run(argv: string[], cwd: string): Promise<void> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdout: 'pipe', stderr: 'pipe' })
  const code = await proc.exited
  if (code !== 0) {
    const err = await new Response(proc.stderr).text()
    throw new Error(`${argv.join(' ')} failed: ${err}`)
  }
}

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-export-'))
  cpSync(appDir, dir, { recursive: true })
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, (await Bun.file(goMod).text()).replace('replace github.com/alternayte/gx => ../../..', `replace github.com/alternayte/gx => ${repo}`))
  mkdirSync(join(dir, 'ui', 'shell'), { recursive: true })
  for (const entry of readdirSync(shellSrc, { withFileTypes: true })) {
    const name = entry.name
    if (!entry.isFile()) continue
    if (name.endsWith('_gx.go') || !(name.endsWith('.gx') || name.endsWith('.go'))) continue
    writeFileSync(join(dir, 'ui', 'shell', name), await Bun.file(join(shellSrc, name)).text())
  }
  mkdirSync(join(dir, 'public', 'images'), { recursive: true })
  writeFileSync(join(dir, 'public', 'images', 'dot.png'), Buffer.from(dotPNG, 'binary'))
  const start = join(dir, 'content', 'docs', 'start.md')
  writeFileSync(start, readFileSync(start, 'utf8') + '\n![Dot](/images/dot.png)\n')
  await run(['go', 'mod', 'tidy'], dir)
  dist = join(dir, 'dist')
  await run(['go', 'run', './cmd/gx', 'export', '-main', '.', '--out', dist, dir], repo)
  const port = 21000 + Math.floor(Math.random() * 1500)
  url = `http://127.0.0.1:${port}`
  server = Bun.serve({
    port,
    fetch: async (req) => {
      let pathname = new URL(req.url).pathname
      if (pathname.endsWith('/')) pathname += 'index.html'
      const file = Bun.file(join(dist, pathname))
      if (!(await file.exists())) return new Response('not found', { status: 404 })
      return new Response(file)
    },
  })
  browser = await chromium.launch({ channel: 'chrome', headless: true })
}, 180000)

afterAll(async () => {
  await Bun.sleep(200)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
  server?.stop(true)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

test('REQ-CNT-11 the exported image has a hash, a size, lazy loading and a srcset', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  const img = await page.waitForSelector('img')
  const src = (await img.getAttribute('src')) ?? ''
  expect(src).toMatch(/^\/images\/dot\.[0-9a-f]{8}\.png$/)
  expect(await img.getAttribute('width')).toBe('1200')
  expect(await img.getAttribute('height')).toBe('600')
  expect(await img.getAttribute('loading')).toBe('lazy')
  const srcset = (await img.getAttribute('srcset')) ?? ''
  expect(srcset).toContain(' 960w')
  expect(srcset).toContain(' 480w')
  await page.waitForFunction(() => {
    const image = document.querySelector('img')
    return image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0
  })
  // With a w-descriptor srcset and no sizes, Chrome reports the
  // density-corrected width; the currentSrc is the 960w variant.
  expect(await img.evaluate((el) => (el as HTMLImageElement).naturalWidth)).toBeGreaterThanOrEqual(960)
})

test('REQ-CNT-11 the page has no layout shift', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  const cls = await page.evaluate(
    () =>
      new Promise<number>((resolve) => {
        let value = 0
        new PerformanceObserver((list) => {
          for (const entry of list.getEntries()) {
            const shift = entry as PerformanceEntry & { hadRecentInput?: boolean; value?: number }
            if (!shift.hadRecentInput) value += shift.value ?? 0
          }
        }).observe({ type: 'layout-shift', buffered: true })
        setTimeout(() => resolve(value), 800)
      }),
  )
  expect(cls).toBeLessThan(0.01)
})
