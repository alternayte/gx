// The docs site, exported with `gx export` and served by a plain file
// server (REQ-EXP-01).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser } from './harness'
import { existsSync, mkdtempSync, readdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))

let dist: string
let server: ReturnType<typeof Bun.serve>
let browser: Browser
let url: string
// requests holds every path the file server answered, with its status.
const requests: { path: string; status: number }[] = []

beforeAll(async () => {
  dist = mkdtempSync(join(tmpdir(), 'gx-docs-dist-'))
  const proc = spawn(['go', 'run', './cmd/gx', 'export', '-main', '.', '--out', dist, 'docs'], {
    cwd: repo,
    stdout: 'ignore',
    stderr: 'pipe',
  })
  if ((await proc.exited) !== 0) {
    throw new Error(`gx export docs failed: ${await new Response(proc.stderr).text()}`)
  }
  // A plain file server: a path is a file, a directory answers its
  // index.html, and anything else is 404.html with status 404.
  server = Bun.serve({
    port: 0,
    fetch: async (req) => {
      let pathname = decodeURIComponent(new URL(req.url).pathname)
      if (pathname.endsWith('/')) pathname += 'index.html'
      const file = Bun.file(join(dist, pathname))
      const found = await file.exists()
      requests.push({ path: pathname, status: found ? 200 : 404 })
      if (found) return new Response(file)
      return new Response(Bun.file(join(dist, '404.html')), { status: 404 })
    },
  })
  url = `http://127.0.0.1:${server.port}`
  browser = await launchBrowser()
}, 300000)

afterAll(async () => {
  await Bun.sleep(200)
  server?.stop(true)
  if (dist) rmSync(dist, { recursive: true, force: true })
})

async function open(path: string): Promise<Page> {
  const page = await browser.newPage()
  await page.goto(url + path, { waitUntil: 'load' })
  return page
}

test('REQ-EXP-01 the docs site exports every page, with hashed assets and 404.html', async () => {
  for (const file of ['index.html', '404.html', 'components/index.html', 'components/button/index.html']) {
    expect(existsSync(join(dist, file))).toBe(true)
  }
  const assets = readdirSync(join(dist, '_gx'))
  expect(assets.length).toBeGreaterThan(3)
  for (const name of assets) expect(name).toMatch(/^[a-z]+\.[0-9a-f]{8}\.(js|css)$/)

  requests.length = 0
  const page = await open('/components/button/')
  expect(await page.title()).toContain('Button')
  // The stylesheet applies: the body has the theme background, not the
  // browser default.
  const sheets = await page.evaluate(() =>
    Array.from(document.querySelectorAll('link[rel="stylesheet"]'), (l) => (l as HTMLLinkElement).href),
  )
  expect(sheets.some((href) => /\/_gx\/app\.[0-9a-f]{8}\.css$/.test(href))).toBe(true)
  expect(await page.evaluate(() => document.styleSheets.length)).toBeGreaterThan(0)
  const scripts = await page.evaluate(() => Array.from(document.scripts, (s) => s.src).filter(Boolean))
  for (const src of scripts) expect(src).toMatch(/\/_gx\/[a-z]+\.[0-9a-f]{8}\.js$/)
  // The browser asks for /favicon.ico by itself; the docs site has none.
  expect(requests.filter((r) => r.status !== 200 && r.path !== '/favicon.ico')).toEqual([])
  await page.close()
})

// errorsOf records the script errors and failed requests of a page.
function errorsOf(page: Page): string[] {
  const errors: string[] = []
  page.on('pageerror', (err) => errors.push(String(err)))
  page.on('requestfailed', (req) => errors.push(`failed: ${req.url()}`))
  return errors
}

test('REQ-EXP-03 a link on the static host is a full load of a working page', async () => {
  const page = await browser.newPage()
  const errors = errorsOf(page)
  await page.goto(url + '/', { waitUntil: 'load' })
  // The page holds no layout slot, so the runtime leaves the click alone.
  expect(await page.locator('[data-gx-slot]').count()).toBe(0)
  await page.evaluate(() => ((window as unknown as { marker?: number }).marker = 1))
  await page.click('a[href="/components/"]')
  await page.waitForURL(url + '/components/')
  await page.waitForLoadState('load')
  expect(await page.evaluate(() => (window as unknown as { marker?: number }).marker)).toBeUndefined()
  expect(await page.locator('h1').first().textContent()).toBeTruthy()
  expect(errors).toEqual([])
  await page.close()
})

test('REQ-EXP-03 the theme control and the tabs work with no server', async () => {
  const page = await browser.newPage()
  const errors = errorsOf(page)
  await page.goto(url + '/components/button/', { waitUntil: 'load' })
  await page.click('[data-gx-theme="dark"]')
  await page.waitForFunction(() => document.documentElement.classList.contains('dark'))
  await page.reload({ waitUntil: 'load' })
  // The stored theme applies again: the hashed theme script runs first.
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)

  // The Preview and Code tabs of a component example.
  const tab = page.locator('[data-gx-tab]').nth(1)
  const panelID = await tab.getAttribute('aria-controls')
  expect(panelID).toBeTruthy()
  await tab.click()
  await page.waitForFunction((id) => document.getElementById(id as string)?.hidden === false, panelID)
  expect(errors).toEqual([])
  await page.close()
})

test('REQ-EXP-03 the search index of the export answers in the browser', async () => {
  const page = await browser.newPage()
  const errors = errorsOf(page)
  await page.goto(url + '/components/', { waitUntil: 'load' })
  await page.keyboard.press('Control+k')
  await page.waitForSelector('[data-gx-search-input]')
  await page.fill('[data-gx-search-input]', 'accordion')
  await page.waitForSelector('.gx-search-result', { timeout: 60000 })
  expect(await page.getAttribute('.gx-search-result', 'href')).toContain('/components/')
  expect(errors).toEqual([])
  await page.close()
}, 90000)

test('REQ-EXP-03 the live toast example runs from static files', async () => {
  const page = await browser.newPage()
  const errors = errorsOf(page)
  // The component page shows each example in a frame; the frame document
  // is an exported page too.
  await page.goto(url + '/components/toast/', { waitUntil: 'load' })
  const frame = await page.locator('iframe').first().getAttribute('src')
  expect(frame).toBe('/preview/toast/toast-default/')
  await page.goto(url + frame, { waitUntil: 'load' })
  await page.locator('[data-docs-toast]').first().click()
  await page.waitForSelector('[data-gx-toaster] [data-gx-toast]')
  expect(errors).toEqual([])
  await page.close()
})

test('REQ-EXP-03 an unknown address answers the exported 404 page', async () => {
  const page = await browser.newPage()
  const res = await page.goto(url + '/no/such/page/', { waitUntil: 'load' })
  expect(res?.status()).toBe(404)
  expect(await page.textContent('body')).toContain('Page not found')
  // The 404 page is a full document with the hashed stylesheet.
  expect(await page.evaluate(() => document.styleSheets.length)).toBeGreaterThan(0)
  await page.close()
})
