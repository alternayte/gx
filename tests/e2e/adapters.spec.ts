// The adapter contract (REQ-ACT-09): one app with no signals, one build, the
// same scenarios under Datastar and under htmx. The htmx app runs under a
// strict Content-Security-Policy with no 'unsafe-eval' (SI-11).
import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser } from './harness'
import { cpSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const adapters = ['datastar', 'htmx'] as const
type Adapter = (typeof adapters)[number]

let dir: string
let browser: Browser
let page: Page
let problems: string[] = []
const apps: Subprocess[] = []
const urls = {} as Record<Adapter, string>

async function run(argv: string[], cwd: string): Promise<void> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdout: 'pipe', stderr: 'pipe' })
  const code = await proc.exited
  if (code !== 0) {
    const out = (await new Response(proc.stdout).text()) + (await new Response(proc.stderr).text())
    throw new Error(`${argv.join(' ')} failed: ${out}`)
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
  dir = mkdtempSync(join(tmpdir(), 'gx-adapters-'))
  cpSync(join(repo, 'tests', 'e2e', 'adapterapp'), dir, { recursive: true })
  // The copied module needs an absolute replace to the repo.
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, (await Bun.file(goMod).text()).replace('=> ../../..', `=> ${repo}`))
  await run(['go', 'mod', 'tidy'], dir)
  // gx.toml names htmx, so the compiler would report a signal (GX4006).
  await run(['go', 'run', './cmd/gx', 'generate', dir], repo)
  await run(['go', 'build', '-o', 'app', '.'], dir)
  const base = 21500 + Math.floor(Math.random() * 1500)
  for (const [i, adapter] of adapters.entries()) {
    urls[adapter] = `http://127.0.0.1:${base + i}`
    apps.push(
      spawn([join(dir, 'app')], {
        cwd: dir,
        env: { ...process.env, ADAPTER: adapter, PORT: String(base + i) },
        stdout: 'ignore',
        stderr: 'ignore',
      }),
    )
  }
  for (const adapter of adapters) await waitForUrl(urls[adapter] + '/')
  browser = await launchBrowser()
}, 240000)

afterAll(async () => {
  await Bun.sleep(300)
  for (const app of apps) app.kill()
  if (dir) rmSync(dir, { recursive: true, force: true })
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

// open loads a page and records every console error, page error and CSP
// violation, so a test can require that the page had none.
async function open(adapter: Adapter, path: string): Promise<void> {
  problems = []
  page = await browser.newPage()
  page.on('console', (m) => {
    // A failed request is in the list with its address, from the response.
    if (m.type() === 'error' && !m.text().startsWith('Failed to load resource')) problems.push(m.text())
  })
  page.on('response', (r) => {
    // The app has no icon.
    if (r.status() >= 400 && !r.url().endsWith('/favicon.ico')) problems.push(`${r.status()} ${r.url()}`)
  })
  page.on('pageerror', (e) => problems.push(String(e)))
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (e) => console.error(`csp: ${e.violatedDirective} ${e.blockedURI}`))
  })
  await page.goto(urls[adapter] + path)
  // The adapter is ready when the load action has run.
  if (path === '/') await waitForText('#lazy-value', '1')
}

async function waitForText(selector: string, want: string, timeout = 5000): Promise<void> {
  const deadline = Date.now() + timeout
  let last = ''
  for (;;) {
    last = ((await page.textContent(selector).catch(() => '')) ?? '').trim()
    if (last === want) return
    if (Date.now() > deadline) throw new Error(`timeout: ${selector} = ${JSON.stringify(last)}, want ${JSON.stringify(want)}`)
    await Bun.sleep(25)
  }
}

async function waitFor(what: string, check: () => Promise<boolean>, timeout = 5000): Promise<void> {
  const deadline = Date.now() + timeout
  for (;;) {
    if (await check()) return
    if (Date.now() > deadline) throw new Error(`timeout: ${what}`)
    await Bun.sleep(25)
  }
}

const number = async (selector: string): Promise<number> => Number(((await page.textContent(selector)) ?? '').trim())

for (const adapter of adapters) {
  test(`REQ-ACT-09 ${adapter}: the page loads the scripts of its adapter only`, async () => {
    await open(adapter, '/')
    const scripts = await page.$$eval('script[src]', (els) => els.map((el) => el.getAttribute('src') ?? ''))
    const other = adapter === 'htmx' ? 'datastar' : 'htmx'
    expect(scripts.some((src) => src.includes(adapter))).toBe(true)
    expect(scripts.some((src) => src.includes(other))).toBe(false)
    const csp = (await (await fetch(urls[adapter] + '/')).headers.get('Content-Security-Policy')) ?? ''
    expect(csp).toContain("'strict-dynamic'")
    // SI-11: htmx needs no 'unsafe-eval'.
    expect(csp.includes("'unsafe-eval'")).toBe(adapter === 'datastar')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a morph patch changes the element in place`, async () => {
    await open(adapter, '/')
    const before = await number('#inc-value')
    await page.evaluate(() => {
      ;(document.querySelector('#inc-value') as Element & { kept?: boolean }).kept = true
    })
    await page.click('#inc')
    await waitForText('#inc-value', String(before + 1))
    await page.click('#inc')
    await waitForText('#inc-value', String(before + 2))
    // A morph keeps the element.
    expect(await page.evaluate(() => (document.querySelector('#inc-value') as Element & { kept?: boolean }).kept)).toBe(true)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: append and prepend put the node inside the target`, async () => {
    await open(adapter, '/')
    await page.click('#last')
    await waitFor('the appended node', async () => ((await page.textContent('[data-list] > :last-child')) ?? '').startsWith('last '))
    await page.click('#first')
    await waitFor('the prepended node', async () => ((await page.textContent('[data-list] > :first-child')) ?? '').startsWith('first '))
    expect(await page.$$eval('[data-list] > *', (els) => els.length)).toBe(3)
    expect(((await page.textContent('[data-list] > :nth-child(2)')) ?? '').trim()).toBe('start')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: replace puts a new element in the place of the target`, async () => {
    await open(adapter, '/')
    await page.evaluate(() => {
      ;(document.querySelector('#swap') as Element & { kept?: boolean }).kept = true
    })
    await page.click('#swap-it')
    await waitForText('#swap', 'new')
    expect(await page.evaluate(() => (document.querySelector('#swap') as Element & { kept?: boolean }).kept ?? false)).toBe(false)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: remove deletes the target`, async () => {
    await open(adapter, '/')
    expect(await page.$('#gone')).not.toBeNull()
    await page.click('#drop')
    await waitFor('the removed node', async () => (await page.$('#gone')) === null)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a redirect goes to the page`, async () => {
    await open(adapter, '/')
    await page.click('#leave')
    await page.waitForSelector('#about')
    expect(new URL(page.url()).pathname).toBe('/about')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a toast arrives in the toaster`, async () => {
    await open(adapter, '/')
    await page.click('#notify')
    await page.waitForSelector('#gx-toaster [data-gx-toast][data-kind="success"]')
    expect((await page.textContent('#gx-toaster [data-gx-toast]')) ?? '').toContain('Saved')
    await page.click('#notify')
    await waitFor('the second toast', async () => (await page.$$('#gx-toaster [data-gx-toast]')).length === 2)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a handler error shows an error toast`, async () => {
    await open(adapter, '/')
    await page.click('#fail')
    await page.waitForSelector('#gx-toaster [data-gx-toast][data-kind="error"]')
    expect((await page.textContent('#gx-toaster [data-gx-toast]')) ?? '').toContain('the board is full')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: an action with no answer changes nothing`, async () => {
    await open(adapter, '/')
    // The first sections hold no timer, so their content is stable.
    const stable = (): Promise<string> =>
      page.$$eval('main section', (els) => els.slice(0, 4).map((el) => el.textContent).join('|'))
    const before = await stable()
    const [res] = await Promise.all([page.waitForResponse((r) => r.url().endsWith('/quiet')), page.click('#quiet')])
    expect(res.status()).toBe(204)
    await Bun.sleep(100)
    expect(await stable()).toBe(before)
    expect(await page.$$eval('#gx-toaster [data-gx-toast]', (els) => els.length)).toBe(0)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a patch runs inside a view transition`, async () => {
    await open(adapter, '/')
    await page.evaluate(() => {
      const w = window as unknown as { transitions: number }
      w.transitions = 0
      const native = document.startViewTransition.bind(document)
      document.startViewTransition = ((cb: () => void) => {
        w.transitions++
        return native(cb)
      }) as typeof document.startViewTransition
    })
    await page.click('#fade')
    await waitForText('#fade-value', '1')
    expect(await page.evaluate(() => (window as unknown as { transitions: number }).transitions)).toBe(1)
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: load, visible and interval run their actions`, async () => {
    await open(adapter, '/')
    await waitForText('#lazy-value', '1')
    await waitFor('two interval runs', async () => (await number('#tick-value')) >= 2)
    // The element is below the viewport, so its action has not run.
    expect(await number('#seen-value')).toBe(0)
    await page.evaluate(() => document.querySelector('#seen-value')?.scrollIntoView())
    await waitForText('#seen-value', '1')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: once, debounce, throttle, window and stop change when an action runs`, async () => {
    await open(adapter, '/')
    const start = {
      once: await number('#once-value'),
      slow: await number('#slow-value'),
      fast: await number('#fast-value'),
      key: await number('#key-value'),
      outer: await number('#outer-value'),
      inner: await number('#inner-value'),
    }
    // once: three presses, one call.
    for (let i = 0; i < 3; i++) await page.click('#once')
    await waitForText('#once-value', String(start.once + 1))
    // debounce: three fast presses, one call after the pause.
    for (let i = 0; i < 3; i++) await page.click('#slow')
    expect(await number('#slow-value')).toBe(start.slow)
    await waitForText('#slow-value', String(start.slow + 1))
    // throttle: three fast presses inside the period, one call.
    for (let i = 0; i < 3; i++) await page.click('#fast')
    await waitForText('#fast-value', String(start.fast + 1))
    // window: a key press anywhere on the page.
    await page.click('h1')
    await page.keyboard.press('a')
    await waitForText('#key-value', String(start.key + 1))
    // stop: the press does not reach the action of the parent.
    await page.click('#inner')
    await waitForText('#inner-value', String(start.inner + 1))
    await Bun.sleep(400)
    expect(await number('#once-value')).toBe(start.once + 1)
    expect(await number('#slow-value')).toBe(start.slow + 1)
    expect(await number('#fast-value')).toBe(start.fast + 1)
    expect(await number('#outer-value')).toBe(start.outer)
    await page.click('#outer', { position: { x: 2, y: 2 } })
    await waitForText('#outer-value', String(start.outer + 1))
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a form shows a field error, keeps the value and redirects when valid`, async () => {
    await open(adapter, '/join')
    await page.waitForSelector('form[data-gx-form]')
    const error = 'form[data-gx-form] [role="alert"]'
    // Live validation on blur (REQ-FRM-06).
    await page.fill('form[data-gx-form] input[type="text"]', 'ab')
    await page.click('h1')
    await waitFor('the live error', async () => ((await page.textContent(error)) ?? '').trim() !== '')
    // A field error of the handler re-renders the form (REQ-ACT-10).
    await page.fill('form[data-gx-form] input[type="text"]', 'taken')
    await page.evaluate(() => {
      ;(window as unknown as { stay: boolean }).stay = true
    })
    await page.click('#join-submit')
    await waitFor('the handler error', async () => ((await page.textContent(error)) ?? '').includes('name.taken'))
    expect(await page.inputValue('form[data-gx-form] input[type="text"]')).toBe('taken')
    // The answer was a patch, not a page load.
    expect(await page.evaluate(() => (window as unknown as { stay?: boolean }).stay)).toBe(true)
    await page.fill('form[data-gx-form] input[type="text"]', 'robin')
    await page.click('#join-submit')
    await page.waitForSelector('#about')
    expect(new URL(page.url()).pathname).toBe('/about')
    expect(problems).toEqual([])
  })

  test(`REQ-ACT-09 ${adapter}: a link navigates inside the layout with no page load`, async () => {
    await open(adapter, '/')
    await page.evaluate(() => {
      ;(window as unknown as { stay: boolean }).stay = true
    })
    await page.click('#nav-about')
    await page.waitForSelector('#about')
    expect(new URL(page.url()).pathname).toBe('/about')
    expect(await page.title()).toBe('About the board')
    expect(await page.getAttribute('#nav-about', 'aria-current')).toBe('page')
    expect(await page.getAttribute('#nav-home', 'aria-current')).toBeNull()
    expect(await page.evaluate(() => (window as unknown as { stay?: boolean }).stay)).toBe(true)
    // The new page works: its actions run after a navigation back.
    await page.click('#nav-home')
    await page.waitForSelector('#home')
    const before = await number('#inc-value')
    await page.click('#inc')
    await waitForText('#inc-value', String(before + 1))
    await page.goBack()
    await page.waitForSelector('#about')
    expect(await page.evaluate(() => (window as unknown as { stay?: boolean }).stay)).toBe(true)
    expect(problems).toEqual([])
  })
}
