// The cart widget of the example shop on a host page of a different origin
// (REQ-ISL-10). The host loads the files that `gx wc build` writes. It is a
// plain file server and not a Gx app.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { spawn } from 'bun'
import { type Browser, type Page } from 'playwright-core'
import { freePort, launchBrowser, startShop, type Shop } from './harness'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))

let browser: Browser
let shop: Shop
let host = ''
let files = ''
let server: ReturnType<typeof Bun.serve>

const pages: Record<string, string> = {
  // The log of the host: the events of the widget.
  '/log.js': `window.log = []
for (const name of ['gx-ready', 'gx-error', 'gx-navigate', 'cart-changed', 'composer-sent']) {
  document.addEventListener(name, (e) => window.log.push([name, e.target.tagName, e.detail ?? null]))
}`,
  '/': `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Partner</title>
<script src="/log.js"></script>
<script type="module" src="/widgets/shop-cart.js"></script></head>
<body><h1>Partner page</h1><shop-cart label="Partner cart"><p id="fallback">Loading the cart</p></shop-cart></body></html>`,
  '/composer': `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Partner mail</title>
<script src="/log.js"></script>
<script type="module" src="/widgets/shop-composer.js"></script></head>
<body><main><h1>Partner mail</h1><button id="host-button">A button of the host</button>
<shop-composer to="pat@example.com"></shop-composer></main></body></html>`,
  '/react': `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>React partner</title>
<script type="module" src="/widgets/shop-cart.js"></script>
<script type="module" src="/react/app.js"></script></head>
<body><div id="root"></div></body></html>`,
}

beforeAll(async () => {
  const port = await freePort()
  host = `http://127.0.0.1:${port}`
  shop = await startShop({ widgetOrigins: host })
  // The files of the widget for a host, with the origin of the shop.
  files = mkdtempSync(join(tmpdir(), 'gx-shopwidget-'))
  const build = spawn(['go', 'run', './cmd/gx', 'wc', 'build', '--server', shop.url, '--out', files, 'examples/shop'], { cwd: repo, stdout: 'pipe', stderr: 'pipe' })
  if ((await build.exited) !== 0) throw new Error('gx wc build failed: ' + (await new Response(build.stderr).text()))
  // The React host: one bundle of the app in widgethost/, with React from
  // the dev dependencies of this suite.
  const bundle = await Bun.build({ entrypoints: [fileURLToPath(new URL('./widgethost/app.tsx', import.meta.url))], target: 'browser', minify: true })
  if (!bundle.success) throw new Error('the React host did not build: ' + bundle.logs.join('\n'))
  pages['/react/app.js'] = await bundle.outputs[0].text()
  server = Bun.serve({
    hostname: '127.0.0.1',
    port,
    fetch(req) {
      const path = new URL(req.url).pathname
      if (path.startsWith('/widgets/')) {
        const file = Bun.file(join(files, path.slice('/widgets/'.length)))
        return new Response(file, { headers: { 'Content-Type': 'text/javascript; charset=utf-8' } })
      }
      const body = pages[path]
      if (body === undefined) return new Response('not found', { status: 404 })
      return new Response(body, { headers: { 'Content-Type': path.endsWith('.js') ? 'text/javascript; charset=utf-8' : 'text/html; charset=utf-8' } })
    },
  })
  browser = await launchBrowser()
})

afterAll(() => {
  server?.stop(true)
  shop?.stop()
})

const open = async (path: string): Promise<Page> => {
  const page = await browser.newPage()
  page.on('pageerror', (err) => {
    throw err
  })
  await page.goto(host + path)
  await page.waitForFunction(() => document.querySelector('shop-cart, shop-composer')?.getAttribute('data-gx-state') === 'ready')
  return page
}

const log = (page: Page): Promise<unknown[][]> => page.evaluate(() => (window as any).log)

const inCart = <T>(page: Page, body: string): Promise<T> =>
  page.evaluate((b) => new Function('root', 'return ' + b)(document.querySelector('shop-cart')!.shadowRoot!), body)

test('REQ-ISL-10 gx wc build writes the files of the cart widget for a host', () => {
  const element = readFileSync(join(files, 'shop-cart.js'), 'utf8')
  expect(element).toContain(`{"tag":"shop-cart","attrs":["label"],"server":"${shop.url}","path":"/widgets/cart"}`)
  const types = readFileSync(join(files, 'shop-cart.d.ts'), 'utf8')
  expect(types).toContain('export interface ChangedDetail {\n  total: number;\n}')
  expect(types).toContain('"cart-changed": CustomEvent<ChangedDetail>;')
  expect(types).toContain('  /** Default "Cart". */\n  label?: string;')
  const manifest = JSON.parse(readFileSync(join(files, 'custom-elements.json'), 'utf8'))
  expect(manifest.modules.map((m: { path: string }) => m.path)).toContain('shop-cart.js')
})

test('REQ-ISL-10 a plain page on a second origin shows the cart widget of the shop', async () => {
  const page = await open('/')
  // The component of the shop pages, rendered by the shop for the widget.
  const answer = (await (await fetch(shop.url + '/widgets/cart?label=Partner%20cart')).json()) as { html: string }
  const compare = await page.evaluate((html) => {
    const rendered = document.createElement('template')
    rendered.innerHTML = html
    const box = document.querySelector('shop-cart')!.shadowRoot!.querySelector('[data-gx-widget]')!
    const names = (root: ParentNode) => [...root.querySelectorAll('*')].map((el) => el.tagName + '.' + el.className)
    return { rendered: names(rendered.content), shown: names(box), text: box.querySelector('p')!.textContent }
  }, answer.html)
  expect(compare.shown).toEqual(compare.rendered)
  expect(compare.shown.length).toBeGreaterThan(8)
  expect(compare.text).toBe('Partner cart total: 10')
  // The stylesheet of the widget is the Tailwind build of its classes: the
  // button has the primary colour of the theme, and the card has a border.
  const style = await inCart<{ button: string; border: string; radius: string }>(
    page,
    `({ button: getComputedStyle(root.querySelector('button')).backgroundColor,
        border: getComputedStyle(root.querySelector('.cart')).borderTopWidth,
        radius: getComputedStyle(root.querySelector('.cart')).borderTopLeftRadius })`,
  )
  expect(style.button).not.toBe('rgba(0, 0, 0, 0)')
  expect(style.border).toBe('1px')
  expect(parseFloat(style.radius)).toBeGreaterThan(0)
  // The host document holds no part of the cart, and its fallback is gone.
  expect(await page.evaluate(() => document.querySelector('.cart'))).toBeNull()
  expect(await page.locator('#fallback').isVisible()).toBe(false)
  await page.close()
})

test('REQ-ISL-10 the signals and the actions of the cart work in the host page', async () => {
  const page = await open('/')
  const total = (): Promise<string> => inCart<string>(page, `root.querySelector('[id^="cart-total"]').textContent`)
  const qty = (): Promise<string> => inCart<string>(page, `root.querySelector('input[type=number]').value`)
  expect(await qty()).toBe('1')
  // bind: and the Add action, with the signal of this cart.
  await page.locator('shop-cart input[type=number]').fill('4')
  await page.locator('shop-cart button', { hasText: 'Add' }).click()
  await page.waitForFunction(() => document.querySelector('shop-cart')!.shadowRoot!.querySelector('[id^="cart-total"]')!.textContent === '40')
  // The domain event of the shop reaches the host, with its typed detail.
  expect((await log(page)).filter((e) => e[0] === 'cart-changed')).toEqual([['cart-changed', 'SHOP-CART', { total: 40 }]])
  // c.SetSignals of the Set action.
  await page.locator('shop-cart button', { hasText: 'Set 2' }).click()
  await page.waitForFunction(() => (document.querySelector('shop-cart')!.shadowRoot!.querySelector('input[type=number]') as HTMLInputElement).value === '2')
  // An action with no answer changes nothing.
  await page.locator('shop-cart button', { hasText: 'Nothing' }).click()
  expect(await total()).toBe('40')
  // A redirect is an event for the host. The page stays.
  await page.locator('shop-cart button', { hasText: 'Go' }).click()
  await page.waitForFunction(() => (window as any).log.some((e: unknown[]) => e[0] === 'gx-navigate'))
  expect((await log(page)).filter((e) => e[0] === 'gx-navigate')).toEqual([['gx-navigate', 'SHOP-CART', { url: '/' }]])
  expect(page.url()).toBe(host + '/')
  // An action that fails is gx-error, with no text of the shop.
  await page.locator('shop-cart button', { hasText: 'Fail' }).click()
  await page.waitForFunction(() => (window as any).log.some((e: unknown[]) => e[0] === 'gx-error'))
  expect((await log(page)).filter((e) => e[0] === 'gx-error')).toEqual([['gx-error', 'SHOP-CART', { status: 500, key: 'gx.error' }]])
  expect(JSON.stringify(await log(page))).not.toContain('demo action failed')
  await page.close()
})

// The composer widget: a form with rules, live validation and an upload, a
// dialog and a select of the registry, a toaster and a typed link, all in the
// shadow root of <shop-composer> on a page of a different origin
// (REQ-ISL-20).

const axeSource = readFileSync(new URL('./node_modules/axe-core/axe.min.js', import.meta.url), 'utf8')

const inComposer = <T>(page: Page, body: string): Promise<T> =>
  page.evaluate((b) => new Function('root', 'return ' + b)(document.querySelector('shop-composer')!.shadowRoot!), body)

const errorOf = (page: Page, field: string): Promise<string> =>
  inComposer<string>(page, `root.querySelector('[id$="-${field}-error"]').textContent.trim()`)

// waitComposer waits for a condition in the shadow root of the composer.
const waitComposer = async (page: Page, body: string): Promise<void> => {
  try {
    await page.waitForFunction((b) => new Function('root', 'return ' + b)(document.querySelector('shop-composer')!.shadowRoot!), body, { timeout: 10000 })
  } catch {
    throw new Error('the composer did not reach: ' + body)
  }
}

const field = (page: Page, name: string) => page.locator(`shop-composer input[name="${name}"]`)

test('REQ-ISL-20 an invalid submit shows the field errors in the widget and keeps the values', async () => {
  const page = await open('/composer')
  // The attribute of the host fills the first field, through the loader.
  expect(await field(page, 'to').inputValue()).toBe('pat@example.com')
  await field(page, 'subject').fill('Hello')
  await field(page, 'body').fill('short')
  await page.locator('shop-composer button[type=submit]').click()
  await waitComposer(page, `root.querySelector('[data-gx-error-summary]') !== null`)
  expect(await errorOf(page, 'body')).not.toBe('')
  expect(await errorOf(page, 'subject')).toBe('')
  expect(await field(page, 'subject').inputValue()).toBe('Hello')
  expect(await field(page, 'body').inputValue()).toBe('short')
  expect(await field(page, 'body').getAttribute('aria-invalid')).toBe('true')
  // The error summary of the form has the focus, inside the widget.
  expect(await inComposer<string>(page, `root.activeElement?.id ?? ''`)).toBe('composer-errors')
  // The host page did not move and holds no part of the form.
  expect(page.url()).toBe(host + '/composer')
  expect(await page.evaluate(() => document.querySelector('form'))).toBeNull()
  expect((await log(page)).filter((e) => e[0] === 'gx-error')).toEqual([])
  await page.close()
})

test('REQ-ISL-20 live validation patches the error of one field in the widget', async () => {
  const page = await open('/composer')
  // At blur.
  await field(page, 'to').fill('not an address')
  await field(page, 'subject').focus()
  await waitComposer(page, `root.querySelector('[id$="-to-error"]').textContent.trim() !== ''`)
  await field(page, 'to').fill('kim@example.com')
  await field(page, 'subject').focus()
  await waitComposer(page, `root.querySelector('[id$="-to-error"]').textContent.trim() === ''`)
  // After the user stops typing: a subject that is empty again is an error.
  await field(page, 'subject').fill('Hi')
  await field(page, 'subject').fill('')
  await waitComposer(page, `root.querySelector('[id$="-subject-error"]').textContent.trim() !== ''`)
  await field(page, 'subject').fill('Hello')
  await waitComposer(page, `root.querySelector('[id$="-subject-error"]').textContent.trim() === ''`)
  // The other fields keep their text: only the error element is patched.
  expect(await field(page, 'to').inputValue()).toBe('kim@example.com')
  await page.close()
})

test('REQ-ISL-20 a field error of the handler comes back in the widget', async () => {
  const page = await open('/composer')
  await field(page, 'to').fill('kim@blocked.example')
  await field(page, 'subject').fill('Hello')
  await field(page, 'body').fill('A message that is long enough.')
  await page.locator('shop-composer button[type=submit]').click()
  await waitComposer(page, `root.querySelector('[id$="-to-error"]').textContent.trim() !== ''`)
  expect(await errorOf(page, 'to')).toContain('composer.blocked')
  expect((await log(page)).filter((e) => e[0] === 'composer-sent')).toEqual([])
  await page.close()
})

test('REQ-ISL-20 a valid submit uploads the file, tells the host and shows a toast in the widget', async () => {
  const page = await open('/composer')
  const note = join(files, 'note.txt')
  writeFileSync(note, 'Twelve bytes')
  await field(page, 'subject').fill('Hello')
  await field(page, 'body').fill('A message that is long enough.')
  await page.locator('shop-composer select[name=priority]').selectOption('high')
  await field(page, 'attachment').setInputFiles(note)
  await page.locator('shop-composer button[type=submit]').click()
  await page.waitForFunction(() => (window as any).log.some((e: unknown[]) => e[0] === 'composer-sent'))
  // The shop got the fields and the file. The host gets the typed event.
  expect((await log(page)).filter((e) => e[0] === 'composer-sent')).toEqual([
    ['composer-sent', 'SHOP-COMPOSER', { to: 'pat@example.com', subject: 'Hello', attachment: 'note.txt', bytes: 12 }],
  ])
  // The toast of the shop is in the toaster of the widget, with the styles
  // of the toast component, and not in the document of the host.
  await waitComposer(page, `root.querySelector('[data-gx-toaster] [data-gx-toast]') !== null`)
  const toast = await inComposer<{ text: string; styled: boolean; fixed: string }>(
    page,
    `(() => { const t = root.querySelector('[data-gx-toaster] [data-gx-toast]');
       return { text: t.textContent, styled: getComputedStyle(t).borderTopWidth !== '0px', fixed: getComputedStyle(root.querySelector('[data-gx-toaster]')).position } })()`,
  )
  expect(toast.text).toContain('Message sent')
  expect(toast.styled).toBe(true)
  expect(toast.fixed).toBe('fixed')
  expect(await page.evaluate(() => document.querySelector('[data-gx-toast]'))).toBeNull()
  // The toast module of the registry runs in the shadow root: the close
  // button of the toast removes it.
  await page.locator('shop-composer [data-gx-toast] [data-gx-close]').last().click()
  await waitComposer(page, `root.querySelector('[data-gx-toast]') === null`)
  await page.close()
})

test('REQ-ISL-20 an upload over the limit of the field is a field error in the widget', async () => {
  const page = await open('/composer')
  const big = join(files, 'big.txt')
  writeFileSync(big, 'x'.repeat(70 * 1024))
  await field(page, 'subject').fill('Hello')
  await field(page, 'body').fill('A message that is long enough.')
  await field(page, 'attachment').setInputFiles(big)
  await page.locator('shop-composer button[type=submit]').click()
  await waitComposer(page, `root.querySelector('[id$="-attachment-error"]').textContent.trim() !== ''`)
  expect((await log(page)).filter((e) => e[0] === 'composer-sent')).toEqual([])
  await page.close()
})

test('REQ-ISL-20 a dialog of the registry works in the widget: open, focus trap, Escape and close', async () => {
  const page = await open('/composer')
  const isOpen = (): Promise<boolean> => inComposer<boolean>(page, `root.querySelector('#composer-discard').open`)
  const focused = (): Promise<string> =>
    inComposer<string>(page, `(() => { const a = root.activeElement; return a ? (a.textContent.trim() || a.tagName) : '' })()`)
  expect(await isOpen()).toBe(false)
  await page.locator('shop-composer button', { hasText: /^Discard$/ }).click()
  await waitComposer(page, `root.querySelector('#composer-discard').open`)
  // The focus is in the dialog, and Tab stays in it.
  const stops = new Set<string>()
  for (let i = 0; i < 4; i++) {
    stops.add(await focused())
    expect(await inComposer<boolean>(page, `root.querySelector('#composer-discard').contains(root.activeElement)`)).toBe(true)
    await page.keyboard.press('Tab')
  }
  expect(stops.has('Discard the draft')).toBe(true)
  expect(stops.has('Close')).toBe(true)
  // Escape closes it.
  await page.keyboard.press('Escape')
  await waitComposer(page, `!root.querySelector('#composer-discard').open`)
  // The button of the footer closes the dialog and invokes its action:
  // the toast of the answer shows in the widget.
  await page.locator('shop-composer button', { hasText: /^Discard$/ }).click()
  await waitComposer(page, `root.querySelector('#composer-discard').open`)
  await page.locator('shop-composer button', { hasText: 'Discard the draft' }).click()
  await waitComposer(page, `!root.querySelector('#composer-discard').open`)
  await waitComposer(page, `[...root.querySelectorAll('[data-gx-toast]')].some((t) => t.textContent.includes('Draft discarded'))`)
  // A button of the host still works: the widget took no key and no click
  // of the page.
  await page.locator('#host-button').focus()
  expect(await page.evaluate(() => document.activeElement?.id)).toBe('host-button')
  await page.close()
})

test('REQ-ISL-20 a select of the registry follows the keyboard in the widget', async () => {
  const page = await open('/composer')
  const select = page.locator('shop-composer select[name=priority]')
  expect(await select.inputValue()).toBe('normal')
  expect(await select.getAttribute('aria-label')).toBe('Priority')
  // Typeahead selects the option that starts with the typed letter, as on
  // a page of the shop.
  await select.focus()
  await page.keyboard.press('h')
  expect(await select.inputValue()).toBe('high')
  // The browser joins keys that come close together into one search text.
  await Bun.sleep(1200)
  await page.keyboard.press('l')
  expect(await select.inputValue()).toBe('low')
  await page.close()
})

test('REQ-ISL-20 a typed link of the widget is an absolute URL of the shop', async () => {
  const page = await open('/composer')
  expect(await inComposer<string>(page, `root.querySelector('#composer-help').getAttribute('href')`)).toBe(shop.url + '/about')
  // After a morph of the widget it is the same URL.
  await page.evaluate(() => document.querySelector('shop-composer')!.setAttribute('to', 'kim@example.com'))
  await page.waitForFunction(() => (document.querySelector('shop-composer')!.shadowRoot!.querySelector('input[name=to]') as HTMLInputElement).value === 'kim@example.com')
  expect(await inComposer<string>(page, `root.querySelector('#composer-help').getAttribute('href')`)).toBe(shop.url + '/about')
  await page.close()
})

test('REQ-ISL-20 an island mounts in the widget and follows a signal of the widget', async () => {
  const page = await browser.newPage()
  const scripts: string[] = []
  page.on('request', (r) => {
    if (r.url().endsWith('.js')) scripts.push(r.url())
  })
  await page.goto(host + '/composer')
  await page.waitForFunction(() => document.querySelector('shop-composer')?.getAttribute('data-gx-state') === 'ready')
  await waitComposer(page, `root.querySelector('gx-island').matches(':state(mounted)')`)
  expect(await inComposer<string>(page, `root.querySelector('gx-island').textContent`)).toBe('0 of 40')
  // The island follows the signal of the widget that the field writes.
  await page.locator('shop-composer #composer-note').fill('hello')
  await waitComposer(page, `root.querySelector('gx-island').textContent === '5 of 40'`)
  // The island loader and the file of the island come from the shop.
  expect(scripts).toContain(shop.url + '/_gx/island.js')
  expect(scripts.some((u) => u.startsWith(shop.url + '/_gx/islands/') && u.includes('NoteCount'))).toBe(true)
  expect(scripts.filter((u) => u.startsWith(host) && !u.endsWith('/log.js'))).toEqual([host + '/widgets/shop-composer.js'])
  // A morph of the widget leaves the island alone: the same element stays
  // mounted, with its text.
  await page.evaluate(() => {
    const el = document.querySelector('shop-composer')!
    ;(window as any).islandBefore = el.shadowRoot!.querySelector('gx-island')
    ;(window as any).islandEvents = 0
    el.shadowRoot!.addEventListener('gx:island', () => (window as any).islandEvents++)
    el.setAttribute('to', 'kim@example.com')
  })
  await page.waitForFunction(() => (document.querySelector('shop-composer')!.shadowRoot!.querySelector('input[name=to]') as HTMLInputElement).value === 'kim@example.com')
  const after = await page.evaluate(() => {
    const island = document.querySelector('shop-composer')!.shadowRoot!.querySelector('gx-island')!
    return { same: island === (window as any).islandBefore, mounted: island.matches(':state(mounted)'), text: island.textContent, mounts: (window as any).islandEvents }
  })
  expect(after).toEqual({ same: true, mounted: true, text: '5 of 40', mounts: 0 })
  await page.close()
})

test('REQ-ISL-20 the composer widget has no serious axe violation, with its dialog open and closed', async () => {
  const page = await open('/composer')
  await page.addStyleTag({ content: '*,*::before,*::after{transition:none!important;animation:none!important}' })
  await page.addScriptTag({ content: axeSource })
  const audit = (): Promise<{ id: string; impact: string | null; help: string }[]> =>
    page.evaluate(async () => {
      const axe = (window as any).axe
      const result = await axe.run(document.querySelector('shop-composer'))
      return result.violations.filter((v: { impact: string | null }) => v.impact === 'serious' || v.impact === 'critical')
    })
  expect(await audit()).toEqual([])
  await page.locator('shop-composer button', { hasText: /^Discard$/ }).click()
  await waitComposer(page, `root.querySelector('#composer-discard').open`)
  expect(await audit()).toEqual([])
  await page.close()
})

test('REQ-ISL-16 a React host renders the widget and gets its events from the props of the tag', async () => {
  const page = await browser.newPage()
  page.on('pageerror', (err) => {
    throw err
  })
  await page.goto(host + '/react')
  // gx-ready reaches the handler of the React component.
  await page.waitForFunction(() => document.querySelector('#state')?.textContent === 'ready')
  expect(await page.locator('shop-cart').getAttribute('data-gx-state')).toBe('ready')
  expect(await inCart<string>(page, `root.querySelector('p').textContent`)).toBe('React cart total: 10')
  expect(await page.locator('#fallback').isVisible()).toBe(false)
  // A domain event of the shop sets state of the React component.
  await page.locator('shop-cart input[type=number]').fill('3')
  await page.locator('shop-cart button', { hasText: 'Add' }).click()
  await page.waitForFunction(() => document.querySelector('#total')?.textContent === '30')
  // React renders the tag again with a new attribute: the widget loads
  // again and morphs, and its signal keeps its value.
  await page.locator('#relabel').click()
  await page.waitForFunction(() => document.querySelector('shop-cart')!.shadowRoot!.querySelector('p')!.textContent!.startsWith('Second label total:'))
  expect(await inCart<string>(page, `root.querySelector('input[type=number]').value`)).toBe('3')
  // The render of React did not touch the shadow root of the element.
  expect(await page.evaluate(() => document.querySelector('#state')!.textContent)).toBe('ready')
  // An action that fails reaches the gx-error handler, with no text of the
  // shop.
  await page.locator('shop-cart button', { hasText: 'Fail' }).click()
  await page.waitForFunction(() => document.querySelector('#state')?.textContent === 'error 500 gx.error')
  await page.close()
})

test('REQ-ACT-18 an optimistic update in a widget comes back when the action fails', async () => {
  const page = await open('/')
  const qty = (): Promise<string> =>
    page.evaluate(() => (document.querySelector('shop-cart')!.shadowRoot!.querySelector('input[type=number]') as HTMLInputElement).value)
  const start = await qty()
  // The answer of the action waits, so the page shows the optimistic value.
  await page.route('**/cart/error', async (route) => {
    await Bun.sleep(300)
    await route.continue()
  })
  await page.click('shop-cart button:text-is("Fail")')
  await page.waitForFunction(() => (document.querySelector('shop-cart')!.shadowRoot!.querySelector('input[type=number]') as HTMLInputElement).value === '99')
  // The action fails: the value of before comes back.
  await page.waitForFunction(
    (want) => (document.querySelector('shop-cart')!.shadowRoot!.querySelector('input[type=number]') as HTMLInputElement).value === want,
    start,
  )
  expect(await qty()).toBe(start)
  await page.close()
})
