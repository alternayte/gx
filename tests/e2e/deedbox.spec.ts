// The Deedbox docs parity checklist in a real browser (REQ-CNT-14): the
// shell, the docs kit components, search, code frames, llms files and meta.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const appDir = join(repo, 'examples', 'deedbox-docs')

let dir: string
let dist: string
let server: ReturnType<typeof Bun.serve>
let browser: Browser
let page: Page
let url: string

async function run(argv: string[], cwd: string): Promise<string> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdout: 'pipe', stderr: 'pipe' })
  const code = await proc.exited
  const out = await new Response(proc.stdout).text()
  if (code !== 0) {
    const err = await new Response(proc.stderr).text()
    throw new Error(`${argv.join(' ')} failed: ${err}`)
  }
  return out
}

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-deedbox-'))
  cpSync(appDir, dir, { recursive: true })
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, readFileSync(goMod, 'utf8').replace('replace github.com/alternayte/gx => ../..', `replace github.com/alternayte/gx => ${repo}`))
  await run(['go', 'mod', 'tidy'], dir)
  dist = join(dir, 'dist')
  await run(['go', 'run', './cmd/gx', 'export', '-main', '.', '--out', dist, dir], repo)
  const port = 22000 + Math.floor(Math.random() * 1500)
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
}, 300000)

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

test('REQ-CNT-14 the splash home shows the hero, actions and card grid', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('.gx-splash')
  expect(await page.$$('.gx-link-button')).toHaveLength(2)
  expect(await page.$$('.gx-card-grid .gx-card')).toHaveLength(4)
})

test('REQ-CNT-14 the sidebar has the Starlight groups and a nested Errors group', async () => {
  page = await browser.newPage()
  await page.goto(url + '/tutorials/first-stream/')
  const labels = await page.$$eval('.gx-nav-group > summary', (els) => els.map((e) => e.textContent?.trim() ?? ''))
  for (const want of ['Tutorials', 'How-to guides', 'Concepts', 'Reference', 'Operations']) {
    expect(labels.some((l) => l.startsWith(want))).toBe(true)
  }
  const nested = await page.$$eval('.gx-nav-sub > summary', (els) => els.map((e) => e.textContent?.trim() ?? ''))
  expect(nested.some((l) => l.startsWith('Errors'))).toBe(true)
  expect(await page.textContent('.gx-nav-sub .gx-nav-badge')).toContain('37')
})

test('REQ-CNT-14 Markdown pages render Steps, Tabs and code frames', async () => {
  page = await browser.newPage()
  await page.goto(url + '/tutorials/first-stream/')
  expect(await page.$('.gx-steps')).not.toBeNull()
  expect(await page.$('.gx-tabs .gx-tab')).not.toBeNull()
  expect(await page.$$('.gx-code')).not.toHaveLength(0)
  expect(await page.$$('.gx-code.terminal')).not.toHaveLength(0)
  expect(await page.$('[data-gx-copy]')).not.toBeNull()
})

test('REQ-CNT-14 synced tabs switch every group with the same key', async () => {
  page = await browser.newPage()
  await page.goto(url + '/tutorials/first-stream/')
  const groups = await page.$$('[data-gx-tabs][data-sync="db"]')
  expect(groups.length).toBe(2)
  const before = await groups[1].$$('[data-gx-tab-panel]')
  expect(await before[0].isVisible()).toBe(true)
  const buttons = await groups[0].$$('[data-gx-tab]')
  await buttons[1].click()
  const after = await groups[1].$$('[data-gx-tab-panel]')
  expect(await after[0].isVisible()).toBe(false)
  expect(await after[1].isVisible()).toBe(true)
})

test('REQ-CNT-14 the table of contents, anchors, edit link and pagination render', async () => {
  page = await browser.newPage()
  await page.goto(url + '/tutorials/first-stream/')
  expect(await page.$$('[data-gx-toc] a')).not.toHaveLength(0)
  expect(await page.$('a.anchor[href^="#"]')).not.toBeNull()
  const edit = await page.getAttribute('.gx-edit-link', 'href')
  expect(edit).toContain('https://github.com/alternayte/deedbox/edit/main/site/src/content/docs/')
  expect(await page.$('.gx-pagination-next')).not.toBeNull()
})

test('REQ-CNT-14 search finds a page by body text', async () => {
  page = await browser.newPage()
  await page.goto(url + '/concepts/erasure-and-keys/')
  await page.keyboard.press('Control+k')
  await page.fill('[data-gx-search-input]', 'unreadable at once')
  await page.waitForSelector('.gx-search-result', { timeout: 60000 })
  const hrefs = await page.$$eval('.gx-search-result', (els) => els.map((e) => e.getAttribute('href') ?? ''))
  expect(hrefs.some((href) => href.includes('/concepts/erasure-and-keys'))).toBe(true)
}, 90000)

test('REQ-CNT-14 the theme select, version badge, GitHub link, skip link and menu', async () => {
  page = await browser.newPage()
  await page.goto(url + '/tutorials/first-stream/')
  expect((await page.textContent('[data-gx-version]'))?.trim()).toBe('v1.0.0')
  const github = await page.getAttribute('a.gx-header-link', 'href')
  expect(github).toBe('https://github.com/alternayte/deedbox')
  expect(await page.getAttribute('.gx-skip-link', 'href')).toBe('#gx-main')
  await page.click('[data-gx-theme="dark"]')
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)
  await page.reload()
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)
  await page.click('[data-gx-theme="auto"]')
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(false)
  await page.setViewportSize({ width: 390, height: 800 })
  await page.waitForFunction(() => document.getElementById('gx-sidebar')?.hidden === true)
  const aside = await page.$('#gx-sidebar')
  expect(await aside?.isVisible()).toBe(false)
  await page.click('[data-gx-menu]')
  await page.waitForFunction(() => document.getElementById('gx-sidebar')?.hidden === false)
  expect(await aside?.isVisible()).toBe(true)
})

test('REQ-CNT-14 the export writes llms files, a raw copy, sitemap and meta', async () => {
  const llms = readFileSync(join(dist, 'llms.txt'), 'utf8')
  expect(llms).toContain('# Deedbox')
  expect(llms).toContain('(/tutorials/first-stream/)')
  const full = readFileSync(join(dist, 'llms-full.txt'), 'utf8')
  expect(full).toContain('Your first stream')
  expect(full).toContain('In this tutorial')
  readFileSync(join(dist, 'llms-small.txt'), 'utf8')
  const raw = readFileSync(join(dist, 'tutorials', 'first-stream.md'), 'utf8')
  expect(raw).toContain('In this tutorial')
  readFileSync(join(dist, 'sitemap.xml'), 'utf8')
  readFileSync(join(dist, 'robots.txt'), 'utf8')
  const html = readFileSync(join(dist, 'concepts', 'erasure-and-keys', 'index.html'), 'utf8')
  for (const want of [
    '<link rel="canonical" href="https://deedbox-docs.pages.dev/concepts/erasure-and-keys/">',
    '<meta property="og:title"',
    '<meta name="twitter:card" content="summary">',
  ]) {
    expect(html).toContain(want)
  }
})
