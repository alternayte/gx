// The dev loop in a real browser: reachable app, overlay on error, morph
// after the fix (REQ-DEV-01, REQ-DEV-03, REQ-DEV-06).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser } from './harness'
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const shop = join(repo, 'examples', 'shop')

let dir: string
let dev: Subprocess
let browser: Browser
let page: Page
let url: string

// freePort picks a port for the dev proxy.
function freePort(): number {
  const listener = Bun.serve({ port: 0, fetch: () => new Response('') })
  const port = listener.port
  listener.stop(true)
  return port
}

// waitForUrl polls until the url answers.
async function waitForUrl(target: string, timeout = 60000): Promise<void> {
  const deadline = Date.now() + timeout
  for (;;) {
    try {
      const res = await fetch(target)
      if (res.ok) return
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`dev server did not start at ${target}`)
    await Bun.sleep(100)
  }
}

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-dev-'))
  cpSync(shop, dir, { recursive: true })
  // The copied module needs an absolute replace to the repo.
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, readFileSync(goMod, 'utf8').replace('replace github.com/alternayte/gx => ../..', `replace github.com/alternayte/gx => ${repo}`))
  const port = freePort()
  url = `http://127.0.0.1:${port}`
  dev = spawn(['go', 'run', './cmd/gx', 'dev', '-addr', `127.0.0.1:${port}`, dir], {
    cwd: repo,
    stdout: 'ignore',
    stderr: 'ignore',
    detached: true,
  })
  await waitForUrl(url + '/')
  browser = await launchBrowser()
}), 180000

afterAll(async () => {
  await Bun.sleep(300)
  if (dev?.pid) {
    try {
      process.kill(-dev.pid, 'SIGTERM')
    } catch {
      dev.kill()
    }
  }
  await Bun.sleep(200)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

test('REQ-DEV-01 the dev server serves the app with the dev client', async () => {
  page = await browser.newPage()
  await page.goto(url + '/signup')
  expect(await page.title()).toContain('Gx shop signup')
  expect(await page.$('#gx-dev-overlay')).toBeNull()
  const client = await page.evaluate(() =>
    [...document.scripts].some((s) => (s.getAttribute('src') ?? '').includes('/_gx/dev-client.js')),
  )
  expect(client).toBe(true)
})

test('REQ-DEV-06 a broken template shows the overlay and the fix recovers', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('h1')
  const view = join(dir, 'Home.gx')
  const original = readFileSync(view, 'utf8')
  // Leave a block open: this is a parse error.
  writeFileSync(view, original + '\nif true {\n')
  await page.waitForSelector('#gx-dev-overlay', { timeout: 30000 })
  const text = (await page.textContent('#gx-dev-overlay')) ?? ''
  expect(text).toContain('unclosed if block')
  expect(await page.$('#gx-dev-overlay a[href^="vscode://file/"]')).not.toBeNull()
  writeFileSync(view, original)
  await page.waitForSelector('#gx-dev-overlay', { state: 'detached', timeout: 30000 })
  expect(await page.textContent('h1')).toBe('Two carts')
  expect(new URL(page.url()).pathname).toBe('/')
}, 60000)

test('REQ-AI-03 the dev gallery renders fixtures and missing components', async () => {
  const res = await fetch(url + '/_gx/gallery')
  expect(res.status).toBe(200)
  const body = await res.text()
  // The cart fixtures render.
  expect(body).toContain('data-fixture="Cart-Default"')
  expect(body).toContain('data-fixture="Cart-Empty"')
  expect(body).toContain('Fixture cart')
  // A component without a fixtures file shows as missing.
  expect(body).toContain('data-fixture="Checkbox"')
  expect(body).toContain('missing fixtures')
})

test('REQ-STY-03 the gallery follows the theme tokens in dark mode', async () => {
  page = await browser.newPage()
  await page.goto(url + '/_gx/gallery')
  const light = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  const dark = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
  expect(light).not.toBe(dark)
  await page.evaluate(() => {
    document.documentElement.classList.remove('dark')
    document.documentElement.classList.add('light')
  })
  const forcedLight = await page.evaluate(() => getComputedStyle(document.body).backgroundColor)
  expect(forcedLight).toBe(light)
})

test('REQ-STY-07/09 navigation starts a view transition with the typed name', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  const name = await page.evaluate(
    () => getComputedStyle(document.querySelector('#hero') as Element).viewTransitionName,
  )
  expect(name).toBe('hero-1')
  await page.evaluate(() => {
    const w = window as unknown as { __gxTransitions: number }
    w.__gxTransitions = 0
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
    const orig = doc.startViewTransition?.bind(doc)
    if (orig) {
      doc.startViewTransition = (cb: () => void) => {
        w.__gxTransitions += 1
        return orig(cb)
      }
    }
  })
  await page.click('a:text-is("About")')
  await page.waitForFunction(() => document.title === 'Gx shop about')
  const transitions = await page.evaluate(
    () => (window as unknown as { __gxTransitions: number }).__gxTransitions,
  )
  expect(transitions).toBeGreaterThan(0)
})

test('REQ-STY-10 reduced motion skips the view transition', async () => {
  page = await browser.newPage()
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto(url + '/')
  await page.evaluate(() => {
    const w = window as unknown as { __gxTransitions: number }
    w.__gxTransitions = 0
    const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
    const orig = doc.startViewTransition?.bind(doc)
    if (orig) {
      doc.startViewTransition = (cb: () => void) => {
        w.__gxTransitions += 1
        return orig(cb)
      }
    }
  })
  await page.click('a:text-is("About")')
  await page.waitForFunction(() => document.title === 'Gx shop about')
  const transitions = await page.evaluate(
    () => (window as unknown as { __gxTransitions: number }).__gxTransitions,
  )
  expect(transitions).toBe(0)
})

test('REQ-STY-08 a forced duplicate view-transition-name still transitions', async () => {
  page = await browser.newPage()
  const warnings: string[] = []
  page.on('console', (msg) => {
    if (msg.type() === 'warning') warnings.push(msg.text())
  })
  await page.goto(url + '/')
  await page.evaluate(() => {
    const meta = document.createElement('meta')
    meta.name = 'gx-dev'
    meta.content = '1'
    document.head.append(meta)
    const hero = document.querySelector('#hero') as HTMLElement
    const dup = hero.cloneNode(true) as HTMLElement
    dup.id = 'hero-dup'
    dup.style.viewTransitionName = 'hero-1'
    dup.style.viewTransitionClass = 'hero'
    hero.after(dup)
  })
  await page.click('a:text-is("About")')
  await page.waitForFunction(() => document.title === 'Gx shop about')
  expect(await page.evaluate(() => new URL(location.href).pathname)).toBe('/about')
  expect(await page.$('#hero')).not.toBeNull()
  expect(warnings.some((w) => w.includes('duplicate view-transition-name'))).toBe(true)
})
