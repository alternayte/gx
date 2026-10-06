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

test('REQ-DEV-03 a rebuild morphs the page and keeps the signal, the input value and the scroll position', async () => {
  page = await browser.newPage({ viewport: { width: 900, height: 320 } })
  await page.goto(url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  // State of the page: a signal with its bound input, a plain property that
  // a full reload destroys, and a scroll position.
  const qty = '[data-label="Alpha"] input[type="number"]'
  await page.fill(qty, '7')
  await page.evaluate(() => {
    ;(window as unknown as { alive?: number }).alive = 1
    window.scrollTo(0, 120)
  })
  expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(60)

  // A change of a Go file takes the rebuild path: the app restarts, and the
  // browser morphs the page.
  const goFile = join(dir, 'home.go')
  const originalGo = readFileSync(goFile, 'utf8')
  const before = await appInfo()
  writeFileSync(goFile, originalGo + '\n// An edit of a Go file.\n')
  try {
    await restarted(before.pid)
    // The dev client asks for the page again after the restart and morphs
    // it.
    await Bun.sleep(800)
    // The page did not load again: it took a patch.
    expect(await page.evaluate(() => (window as unknown as { alive?: number }).alive)).toBe(1)
    // Read the scroll position before the click below: Playwright scrolls a
    // button into view before it clicks, and that moves the page when the
    // button is above the viewport.
    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(60)
    expect(await page.inputValue(qty)).toBe('7')
    // The signal holds 7 too: a client expression of the cart reads it.
    await page.click('[data-label="Alpha"] button:text-is("Add")')
    await page.waitForFunction(
      () => document.querySelector('[data-label="Alpha"] span[id^="cart-total"]')?.textContent === '70',
      undefined,
      { timeout: 10000 },
    )
    // The other cart did not get the value of the first one.
    expect(await page.inputValue('[data-label="Beta"] input[type="number"]')).toBe('1')
  } finally {
    const now = await appInfo()
    writeFileSync(goFile, originalGo)
    await restarted(now.pid)
  }
}, 120000)

// restarted waits until the app is a different process than pid and
// answers.
async function restarted(pid: number): Promise<void> {
  const deadline = Date.now() + 60000
  for (;;) {
    try {
      if ((await appInfo()).pid !== pid) return
    } catch {
      // the app is between two processes
    }
    if (Date.now() > deadline) throw new Error('the app did not restart')
    await Bun.sleep(50)
  }
}

// appInfo reads the dev-only info route of the app: its process id and the
// count of functions that run as interpreted code.
async function appInfo(): Promise<{ pid: number; swapped: number }> {
  return (await fetch(url + '/_gx/dev/info')).json() as Promise<{ pid: number; swapped: number }>
}

test('REQ-DEV-02 a markup edit swaps into the running app and keeps the state of the page', async () => {
  page = await browser.newPage({ viewport: { width: 900, height: 320 } })
  await page.goto(url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  const qty = '[data-label="Alpha"] input[type="number"]'
  await page.fill(qty, '7')
  await page.evaluate(() => {
    ;(window as unknown as { alive?: number }).alive = 1
    window.scrollTo(0, 120)
  })
  const before = await appInfo()
  const view = join(dir, 'Home.gx')
  const original = readFileSync(view, 'utf8')
  const started = Date.now()
  writeFileSync(view, original.replace('Two carts', 'Two carts, swapped'))
  try {
    await page.waitForFunction(() => document.querySelector('h1')?.textContent === 'Two carts, swapped', undefined, {
      timeout: 30000,
      polling: 5,
    })
    const took = Date.now() - started
    // NFR-01: save to morphed page under 150 ms on the reference machine.
    // A shared runner is slower (D-110).
    console.log(`NFR-01: save to morphed page ${took} ms`)
    expect(took).toBeLessThan(process.env.CI ? 1500 : 150)
    // The app is the same process: no rebuild and no restart.
    const after = await appInfo()
    expect(after.pid).toBe(before.pid)
    expect(after.swapped).toBeGreaterThan(0)
    // The page kept its state.
    expect(await page.evaluate(() => (window as unknown as { alive?: number }).alive)).toBe(1)
    expect(await page.evaluate(() => window.scrollY)).toBeGreaterThan(60)
    expect(await page.inputValue(qty)).toBe('7')
    await page.click('[data-label="Alpha"] button:text-is("Add")')
    await page.waitForFunction(
      () => document.querySelector('[data-label="Alpha"] span[id^="cart-total"]')?.textContent === '70',
      undefined,
      { timeout: 10000 },
    )
  } finally {
    writeFileSync(view, original)
    await page.waitForFunction(() => document.querySelector('h1')?.textContent === 'Two carts', undefined, {
      timeout: 30000,
    })
  }
}, 90000)

test('REQ-DEV-02 a markup edit keeps the state of an island', async () => {
  page = await browser.newPage({ viewport: { width: 1000, height: 700 } })
  await page.goto(url + '/dashboard')
  const chart = 'gx-island[name$="/BarChart"]'
  await page.waitForFunction((sel) => document.querySelector(sel)?.matches(':state(mounted)'), chart)
  const zoom = page.locator(chart + ' [data-zoom]')
  await zoom.click()
  await zoom.click()
  const before = await appInfo()
  const view = join(dir, 'dashboard', 'Dashboard.gx')
  const original = readFileSync(view, 'utf8')
  writeFileSync(view, original.replace('>Dashboard</h1>', '>Dashboard, swapped</h1>'))
  try {
    await page.waitForFunction(() => document.querySelector('main h1')?.textContent === 'Dashboard, swapped', undefined, {
      timeout: 30000,
    })
    expect((await appInfo()).pid).toBe(before.pid)
    // The island did not mount again: its zoom level is the one from
    // before the edit.
    expect(await zoom.getAttribute('data-zoom')).toBe('3')
    expect(await page.getAttribute(chart + ' [data-gx-island-root]', 'data-mounts')).toBe('1')
  } finally {
    writeFileSync(view, original)
    await page.waitForFunction(() => document.querySelector('main h1')?.textContent === 'Dashboard', undefined, {
      timeout: 30000,
    })
  }
}, 90000)

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
