// A widget on a host page of a different origin, in a real browser
// (REQ-ISL-10, DR-09). The host is a plain file server and not a Gx app; the
// Gx app renders the widget and answers its requests.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { spawn, type Subprocess } from 'bun'
import { type Browser, type Page, type Request } from 'playwright-core'
import { freePort, launchBrowser } from './harness'
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))

let browser: Browser
let server: Subprocess
let api = ''
let host = ''

beforeAll(async () => {
  const dir = mkdtempSync(join(tmpdir(), 'gx-widget-'))
  const bin = join(dir, process.platform === 'win32' ? 'widgetapp.exe' : 'widgetapp')
  const build = spawn(['go', 'build', '-o', bin, './tests/e2e/widgetapp'], { cwd: repo, stderr: 'pipe' })
  if ((await build.exited) !== 0) throw new Error('go build failed: ' + (await new Response(build.stderr).text()))
  const apiAddr = `127.0.0.1:${await freePort()}`
  const hostAddr = `127.0.0.1:${await freePort()}`
  api = 'http://' + apiAddr
  host = 'http://' + hostAddr
  // An app root for the stylesheet build: the default theme of gx init, the
  // Tailwind pin of the shop, and the class list that the compiler writes
  // for the widget.
  const root = join(dir, 'app-root')
  mkdirSync(join(root, 'app'), { recursive: true })
  mkdirSync(join(root, '.gx'), { recursive: true })
  writeFileSync(join(root, 'app', 'theme.css'), readFileSync(join(repo, 'examples', 'shop', 'app', 'theme.css')))
  writeFileSync(join(root, 'gx.lock'), readFileSync(join(repo, 'examples', 'shop', 'gx.lock')))
  writeFileSync(
    join(root, '.gx', 'widget-classes.json'),
    JSON.stringify({ 'acme-cart': ['p-4', 'shadow-lg', 'rounded-xl', 'bg-primary', 'dark:bg-foreground'] }),
  )
  server = spawn([bin, '-api', apiAddr, '-host', hostAddr, '-styles', root], { stdout: 'ignore', stderr: 'inherit' })
  const deadline = Date.now() + 120000
  for (;;) {
    try {
      if ((await fetch(host + '/')).ok && (await fetch(api + '/widgets/cart')).ok) break
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error('widgetapp did not start')
    await Bun.sleep(100)
  }
  browser = await launchBrowser()
})

afterAll(() => {
  server?.kill()
})

const open = async (path: string): Promise<Page> => {
  const page = await browser.newPage()
  page.on('pageerror', (err) => {
    throw err
  })
  await page.goto(host + path)
  return page
}

const log = (page: Page): Promise<unknown[][]> => page.evaluate(() => (window as any).log)

const waitState = (page: Page, state: string): Promise<unknown> =>
  page.waitForFunction((s) => document.querySelector('acme-cart')?.getAttribute('data-gx-state') === s, state)

test('REQ-ISL-10 a plain page on a second origin shows the widget as the server renders it', async () => {
  const page = await open('/')
  await waitState(page, 'ready')
  // The server renders the component. The shadow root holds that HTML.
  const answer = (await (await fetch(api + '/widgets/cart?currency=USD')).json()) as { html: string; tag: string }
  expect(answer.tag).toBe('acme-cart')
  const shown = await page.evaluate(() => document.querySelector('acme-cart')!.shadowRoot!.querySelector('[data-gx-widget]')!.innerHTML)
  expect(shown.replace(/<p id="loads">\d+<\/p>/, '')).toBe(answer.html.replace(/<p id="loads">\d+<\/p>/, ''))
  expect(await page.locator('acme-cart #line').textContent()).toBe('2 items in USD')
  // The host document does not hold the HTML of the widget.
  expect(await page.evaluate(() => document.getElementById('line'))).toBeNull()
  // The fallback content of the host does not show after the load.
  expect(await page.locator('#fallback').isVisible()).toBe(false)
  await page.close()
})

test('REQ-ISL-16 the element has a state attribute and fires gx-ready one time', async () => {
  const page = await open('/')
  await waitState(page, 'ready')
  // A second load does not fire the event again.
  await page.evaluate(() => document.querySelector('acme-cart')!.setAttribute('currency', 'EUR'))
  await page.waitForFunction(() => document.querySelector('acme-cart')!.shadowRoot!.querySelector('#line')!.textContent === '2 items in EUR')
  const seen = await log(page)
  expect(seen.filter((e) => e[0] === 'state').map((e) => e[1])).toEqual(['loading', 'ready'])
  // The listener is on the document: the event leaves the element.
  expect(seen.filter((e) => e[0] === 'gx-ready')).toEqual([['gx-ready', 'ACME-CART', null]])
  expect(seen.filter((e) => e[0] === 'gx-error')).toEqual([])
  await page.close()
})

test('REQ-ISL-16 a server that is down gives gx-error, and the fallback content shows', async () => {
  const page = await open('/down')
  await waitState(page, 'error')
  const seen = await log(page)
  expect(seen.filter((e) => e[0] === 'gx-error')).toEqual([['gx-error', 'ACME-CART', { status: 0, key: 'gx.network' }]])
  expect(seen.filter((e) => e[0] === 'gx-ready')).toEqual([])
  expect(await page.locator('#fallback').isVisible()).toBe(true)
  await page.close()
})

test('REQ-ISL-16 an error of the loader gives its status and a key', async () => {
  const page = await open('/refused')
  await waitState(page, 'error')
  expect((await log(page)).filter((e) => e[0] === 'gx-error')).toEqual([['gx-error', 'ACME-CART', { status: 403, key: 'gx.forbidden' }]])
  expect(await page.locator('#fallback').isVisible()).toBe(true)
  await page.close()
})

test('REQ-ISL-15 a bad attribute value gives gx-error with status 400', async () => {
  const page = await open('/bad')
  await waitState(page, 'error')
  const errors = (await log(page)).filter((e) => e[0] === 'gx-error')
  expect(errors.length).toBe(1)
  expect((errors[0][2] as { status: number }).status).toBe(400)
  expect(await page.locator('#fallback').isVisible()).toBe(true)
  await page.close()
})

test('REQ-ISL-15 a changed attribute fetches again and morphs; a typed value stays', async () => {
  const page = await open('/')
  await waitState(page, 'ready')
  await page.locator('acme-cart #note').fill('leave at the door')
  await page.evaluate(() => {
    const el = document.querySelector('acme-cart')!
    ;(window as any).noteBefore = el.shadowRoot!.querySelector('#note')
    ;(window as any).loadsBefore = el.shadowRoot!.querySelector('#loads')!.textContent
    el.setAttribute('currency', 'EUR')
    el.setAttribute('compact', '')
  })
  await page.waitForFunction(() => document.querySelector('acme-cart')!.shadowRoot!.querySelector('#line')!.textContent === '2 items in EUR')
  const after = await page.evaluate(() => {
    const root = document.querySelector('acme-cart')!.shadowRoot!
    const note = root.querySelector('#note') as HTMLInputElement
    return {
      sameNode: note === (window as any).noteBefore,
      value: note.value,
      compact: root.querySelector('#cart')!.getAttribute('data-compact'),
      loads: Number(root.querySelector('#loads')!.textContent) - Number((window as any).loadsBefore),
    }
  })
  // The two attribute changes of one task give one request.
  expect(after).toEqual({ sameNode: true, value: 'leave at the door', compact: 'true', loads: 1 })
  await page.close()
})

test('REQ-ISL-19 the element file is a loader: the script of the widget comes from the Gx server with a hash', async () => {
  const page = await browser.newPage()
  const requests: Request[] = []
  page.on('request', (r) => requests.push(r))
  await page.goto(host + '/')
  await waitState(page, 'ready')
  const scripts = requests.map((r) => r.url()).filter((u) => u.endsWith('.js'))
  expect(scripts[0]).toBe(host + '/acme-cart.js')
  expect(scripts[1]).toMatch(new RegExp('^' + api.replaceAll('.', '\\.') + '/_gx/widget\\.[0-9a-f]{12}\\.js$'))
  expect(scripts.length).toBe(2)
  // The answer names the build of the server.
  const answer = (await (await fetch(api + '/widgets/cart')).json()) as { build: string; script: string }
  expect(answer.build).toMatch(/^[0-9a-f]{16}$/)
  expect(api + answer.script).toBe(scripts[1])
  // The element file of the host is small: it holds no runtime.
  const element = await (await fetch(host + '/acme-cart.js')).text()
  expect(element.length).toBeLessThan(4096)
  await page.close()
})

test('SI-14 the element sends no cookie to an origin that it does not share', async () => {
  const context = await browser.newContext()
  // The user has a session cookie of the Gx server.
  await context.addCookies([{ name: 'session', value: 'alice', url: api, sameSite: 'Lax' }])
  const page = await context.newPage()
  const requests: Request[] = []
  page.on('request', (r) => requests.push(r))
  await page.goto(host + '/')
  await waitState(page, 'ready')
  const toAPI = requests.filter((r) => r.url().startsWith(api))
  expect(toAPI.length).toBeGreaterThan(1)
  for (const r of toAPI) {
    expect((await r.allHeaders())['cookie']).toBeUndefined()
  }
  await context.close()
})

test('REQ-ISL-11 the stylesheet of the widget is in its shadow root; the styles of the host stay out', async () => {
  const page = await browser.newPage()
  const requests: Request[] = []
  page.on('request', (r) => requests.push(r))
  await page.goto(host + '/themed')
  await page.waitForFunction(() => [...document.querySelectorAll('acme-cart')].every((el) => el.getAttribute('data-gx-state') === 'ready'))
  const styles = await page.evaluate(() => {
    const color = (el: Element | null) => getComputedStyle(el!).color
    const plain = document.querySelector('#plain')!.shadowRoot!
    const branded = document.querySelector('#branded')!.shadowRoot!
    return {
      // The rule of the widget stylesheet, with the token of the widget.
      line: color(plain.querySelector('#line')),
      // The p rule of the host does not reach into the widget.
      loads: color(plain.querySelector('#loads')),
      margin: getComputedStyle(plain.querySelector('#line')!).marginTop,
      // The host gives a token of the widget its own value on the element.
      branded: color(branded.querySelector('#line')),
      // The stylesheet of the widget does not reach the host page.
      host: color(document.querySelector('#hostline')),
      // Two widgets of one tag share one stylesheet object.
      shared: plain.adoptedStyleSheets.length === 1 && plain.adoptedStyleSheets[0] === branded.adoptedStyleSheets[0],
      links: plain.querySelectorAll('link, style').length,
    }
  })
  expect(styles).toEqual({
    line: 'rgb(1, 2, 3)',
    loads: 'rgb(9, 9, 9)',
    margin: '0px',
    branded: 'rgb(0, 0, 250)',
    host: 'rgb(200, 0, 0)',
    shared: true,
    links: 0,
  })
  // The server serves the stylesheet one time, with a hash in its name.
  const sheets = requests.map((r) => r.url()).filter((u) => u.endsWith('.css'))
  expect(sheets.length).toBe(1)
  expect(sheets[0]).toMatch(new RegExp('^' + api.replaceAll('.', '\\.') + '/_gx/widgets/acme-cart\\.[0-9a-f]{12}\\.css$'))
  await page.close()
})

test('REQ-ISL-11 a Tailwind build works in the shadow root: shadows, theme tokens and dark mode', async () => {
  const page = await open('/tailwind')
  await page.waitForFunction(() => [...document.querySelectorAll('acme-cart')].every((el) => el.getAttribute('data-gx-state') === 'ready'))
  const seen = await page.evaluate(() => {
    const box = (id: string) => getComputedStyle(document.querySelector('#' + id)!.shadowRoot!.querySelector('#box')!)
    const light = box('light')
    const dark = box('dark')
    const hostbox = getComputedStyle(document.querySelector('#hostbox')!)
    return {
      padding: light.paddingTop,
      // A shadow needs the start values of the Tailwind variables, which a
      // browser does not take from @property in a shadow root.
      shadow: light.boxShadow !== 'none' && light.boxShadow !== '',
      radius: parseFloat(light.borderTopLeftRadius) > 0,
      // The token of the theme has a value on the host element.
      lightBackground: light.backgroundColor,
      // The class dark of the host element turns on dark: classes and the
      // dark tokens.
      darkBackground: dark.backgroundColor,
      // The classes of the widget give the host page no styles.
      hostPadding: hostbox.paddingTop,
      hostShadow: hostbox.boxShadow,
    }
  })
  expect(seen.padding).toBe('16px')
  expect(seen.shadow).toBe(true)
  expect(seen.radius).toBe(true)
  expect(seen.lightBackground).not.toBe('rgba(0, 0, 0, 0)')
  expect(seen.darkBackground).not.toBe('rgba(0, 0, 0, 0)')
  expect(seen.darkBackground).not.toBe(seen.lightBackground)
  expect(seen.hostPadding).toBe('0px')
  expect(seen.hostShadow).toBe('none')
  // The stylesheet holds the classes of the widget and no other class of
  // Tailwind.
  const answer = (await (await fetch(api + '/widgets/cart')).json()) as { style: string }
  const css = await (await fetch(api + answer.style)).text()
  expect(css).toContain('.shadow-lg{')
  expect(css).not.toContain('.underline')
  // The start values of the Tailwind variables hold with no condition.
  expect(css).toContain('@layer properties{*,:before,:after,::backdrop{--tw-shadow:0 0 #0000;')
  await page.close()
})
