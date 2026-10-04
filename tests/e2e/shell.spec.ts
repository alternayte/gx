// The docs-shell parity checklist in a real browser (REQ-CNT-06): header,
// sidebar, table of contents, pagination, page meta, splash, 404 and the
// client behaviours.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const appDir = join(repo, 'tests', 'e2e', 'shellapp')
const shellSrc = join(repo, 'registry', 'docs-shell')

let dir: string
let app: Subprocess
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

async function waitForUrl(target: string, timeout = 30000): Promise<void> {
  const deadline = Date.now() + timeout
  for (;;) {
    try {
      const res = await fetch(target)
      if (res.ok) return
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`app did not start at ${target}`)
    await Bun.sleep(100)
  }
}

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-shell-'))
  cpSync(appDir, dir, { recursive: true })
  // The copied module needs an absolute replace to the repo.
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, (await Bun.file(goMod).text()).replace('replace github.com/alternayte/gx => ../../..', `replace github.com/alternayte/gx => ${repo}`))
  mkdirSync(join(dir, 'ui', 'shell'), { recursive: true })
  for (const entry of readdirSync(shellSrc, { withFileTypes: true })) {
    const name = entry.name
    if (!entry.isFile()) continue
    if (name.endsWith('_gx.go') || !(name.endsWith('.gx') || name.endsWith('.go'))) continue
    writeFileSync(join(dir, 'ui', 'shell', name), await Bun.file(join(shellSrc, name)).text())
  }
  await run(['go', 'mod', 'tidy'], dir)
  const port = 20000 + Math.floor(Math.random() * 1500)
  url = `http://127.0.0.1:${port}`
  app = spawn(['go', 'run', './cmd/gx', 'dev', '-addr', `127.0.0.1:${port}`, '-main', '.', dir], {
    cwd: repo,
    stdout: 'ignore',
    stderr: 'ignore',
    detached: true,
  })
  await waitForUrl(url + '/')
  browser = await chromium.launch({ channel: 'chrome', headless: true })
}), 240000

afterAll(async () => {
  await Bun.sleep(200)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
  if (app?.pid) {
    try {
      process.kill(-app.pid, 'SIGTERM')
    } catch {
      app.kill()
    }
  }
  await Bun.sleep(200)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

test('REQ-CNT-06 the header shows the title, version, links and search', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  expect((await page.textContent('.gx-site-title'))?.trim()).toBe('Deedbox docs')
  expect((await page.textContent('[data-gx-version]'))?.trim()).toBe('v0.42.0')
  const social = await page.getAttribute('a.gx-header-link', 'href')
  expect(social).toBe('https://example.com/deedbox')
  expect(await page.$('[data-gx-search-open]')).not.toBeNull()
  expect(await page.$('[data-gx-theme="dark"]')).not.toBeNull()
})

test('REQ-CNT-06 the sidebar has ordered groups, nesting, badges and collapse', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  const groups = await page.$$eval('.gx-nav-group > summary', (els) => els.map((e) => e.textContent?.trim() ?? ''))
  const start = groups.findIndex((g) => g.startsWith('Start'))
  const guides = groups.findIndex((g) => g.startsWith('Guides'))
  const errors = groups.findIndex((g) => g.startsWith('Errors'))
  expect(start).toBeGreaterThanOrEqual(0)
  expect(guides).toBeGreaterThan(start)
  expect(errors).toBeGreaterThan(guides)
  // A badge and a nested group.
  expect(await page.textContent('.gx-nav-badge')).toContain('New')
  const nested = await page.$$('.gx-nav-sub summary')
  expect(await nested[0]?.textContent()).toContain('Routing')
  expect(await page.$('.gx-nav-sub .gx-nav-link')).not.toBeNull()
  // The Errors group starts collapsed.
  const open = await page.$$eval('.gx-nav-group', (els) => els.map((e) => (e as HTMLDetailsElement).open))
  expect(open[open.length - 1]).toBe(false)
})

test('REQ-CNT-06 the table of contents follows the depth and the scroll', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  const links = await page.$$eval('[data-gx-toc] a', (els) => els.map((e) => e.textContent ?? ''))
  expect(links).toContain('Install')
  expect(links).toContain('The database')
  expect(links).toContain('First stream')
  expect(links).not.toContain('Details')
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  await page.waitForFunction(() => {
    const link = document.querySelector('[data-gx-toc-target="next-steps"]')
    return link?.getAttribute('aria-current') === 'location'
  })
  expect(await page.getAttribute('[data-gx-toc-target="next-steps"]', 'aria-current')).toBe('location')
  expect(await page.getAttribute('[data-gx-toc-target="install"]', 'aria-current')).toBeNull()
})

test('REQ-CNT-06 pagination, edit link, last updated and the skip link', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  expect((await page.textContent('.gx-pagination-prev'))?.trim()).toContain('Deedbox')
  expect((await page.textContent('.gx-pagination-next'))?.trim()).toContain('Routing')
  const edit = await page.getAttribute('.gx-edit-link', 'href')
  expect(edit).toBe('https://example.com/deedbox/edit/main/content/docs/start.md')
  expect((await page.textContent('[data-gx-updated]'))?.trim()).toContain('Oct 2, 2026')
  expect(await page.getAttribute('.gx-skip-link', 'href')).toBe('#gx-main')
})

test('REQ-CNT-06 the splash template renders without the shell', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('.gx-splash')
  expect(await page.$('.gx-shell')).toBeNull()
  expect(await page.textContent('.gx-splash h1')).toBe('Deedbox')
})

test('REQ-CNT-06 a missing page answers 404 with the shell 404', async () => {
  page = await browser.newPage()
  const res = await page.goto(url + '/missing/')
  expect(res?.status()).toBe(404)
  expect(await page.textContent('.gx-not-found h1')).toBe('Page not found')
})

test('REQ-CNT-06 the theme select switches and remembers the choice', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  await page.click('[data-gx-theme="dark"]')
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)
  expect(await page.getAttribute('[data-gx-theme="dark"]', 'aria-pressed')).toBe('true')
  await page.reload()
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)
  await page.click('[data-gx-theme="auto"]')
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(false)
})

test('REQ-CNT-06 Ctrl+K opens the search dialog and Close closes it', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  await page.keyboard.press('Control+k')
  await page.waitForFunction(() => document.querySelector('dialog[data-gx-search]')?.hasAttribute('open') === true)
  await page.click('[data-gx-search-close]')
  await page.waitForFunction(() => document.querySelector('dialog[data-gx-search]')?.hasAttribute('open') !== true)
})

test('REQ-CNT-07 search finds a page by body text', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  await page.keyboard.press('Control+k')
  await page.waitForSelector('[data-gx-search-input]')
  await page.fill('[data-gx-search-input]', 'lorem ipsum')
  await page.waitForSelector('.gx-search-result', { timeout: 60000 })
  const href = await page.getAttribute('.gx-search-result', 'href')
  expect(href).toContain('/start')
  const text = (await page.textContent('.gx-search-results')) ?? ''
  expect(text.toLowerCase()).toContain('lorem')
}, 90000)

test('REQ-CNT-12 a Markdown edit reaches the page under 2.5 s', async () => {
  page = await browser.newPage()
  await page.goto(url + '/start/')
  await page.waitForSelector('#gx-main')
  const file = join(dir, 'content', 'docs', 'start.md')
  const original = readFileSync(file, 'utf8')
  const start = Date.now()
  writeFileSync(file, original + '\n\nFreshmarker appears.\n')
  await page.waitForFunction(() => document.body.innerText.includes('Freshmarker'), null, { timeout: 30000 })
  const elapsed = Date.now() - start
  writeFileSync(file, original)
  expect(elapsed).toBeLessThan(2500)
}, 60000)

test('REQ-CNT-06 the mobile menu shows and hides the sidebar', async () => {
  page = await browser.newPage({ viewport: { width: 390, height: 800 } })
  await page.goto(url + '/start/')
  const aside = await page.$('#gx-sidebar')
  expect(await aside?.isVisible()).toBe(false)
  await page.click('[data-gx-menu]')
  await page.waitForFunction(() => document.getElementById('gx-sidebar')?.hidden === false)
  expect(await aside?.isVisible()).toBe(true)
  expect(await page.getAttribute('[data-gx-menu]', 'aria-expanded')).toBe('true')
  await page.click('[data-gx-menu]')
  await page.waitForFunction(() => document.getElementById('gx-sidebar')?.hidden === true)
  expect(await aside?.isVisible()).toBe(false)
})
