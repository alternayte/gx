// Action answers and signal scoping, driven in a real browser (M4).
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop()
  browser = await chromium.launch({ channel: 'chrome', headless: true })
})

afterAll(async () => {
  // Let pending CDP preview callbacks drain before the browser goes away;
  // playwright-core can otherwise report a dropped handle while it closes.
  await Bun.sleep(300)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
  await Bun.sleep(100)
  shop?.stop()
})

beforeEach(async () => {
  page = await browser.newPage()
  await page.goto(shop.url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

async function clickCart(label: string, button: string): Promise<void> {
  await page.click(`[data-label="${label}"] button:text-is("${button}")`)
}

async function total(label: string): Promise<string> {
  return (await page.textContent(`[data-label="${label}"] span[id^="cart-total"]`)) ?? ''
}

// waitForText polls a selector until its text contains want.
async function waitForText(selector: string, want: string, timeout = 5000): Promise<string> {
  const deadline = Date.now() + timeout
  let last = ''
  for (;;) {
    last = (await page.textContent(selector)) ?? ''
    if (last.includes(want)) return last
    if (Date.now() > deadline) throw new Error(`timeout: ${selector} = ${JSON.stringify(last)}, want ${JSON.stringify(want)}`)
    await Bun.sleep(50)
  }
}

// waitForValue polls an input until it holds want.
async function waitForValue(selector: string, want: string, timeout = 5000): Promise<void> {
  const deadline = Date.now() + timeout
  let last = ''
  for (;;) {
    last = await page.inputValue(selector)
    if (last === want) return
    if (Date.now() > deadline) throw new Error(`timeout: ${selector} = ${last}, want ${want}`)
    await Bun.sleep(50)
  }
}

test('REQ-ACT-01 patch answer morphs one instance', async () => {
  expect(await total('Alpha')).toBe('1')
  await clickCart('Alpha', 'Add')
  await waitForText('[data-label="Alpha"] span[id^="cart-total"]', '10')
  expect(await total('Beta')).toBe('2')
})

test('REQ-ACT-14 patching one instance leaves the other alone', async () => {
  await clickCart('Beta', 'Add')
  await waitForText('[data-label="Beta"] span[id^="cart-total"]', '10')
  expect(await total('Alpha')).toBe('1')
})

test('REQ-ACT-05 set signals updates the invoking input', async () => {
  const input = '[data-label="Alpha"] input[type="number"]'
  await clickCart('Alpha', 'Set 2')
  await waitForValue(input, '2')
  // The other instance keeps its own Qty (REQ-ACT-06).
  expect(await page.inputValue('[data-label="Beta"] input[type="number"]')).toBe('1')
})

test('REQ-ACT-06 signal edits stay in one instance', async () => {
  const alpha = '[data-label="Alpha"] input[type="number"]'
  await page.fill(alpha, '7')
  await page.focus(alpha)
  await page.keyboard.press('Tab')
  await Bun.sleep(200)
  expect(await page.inputValue(alpha)).toBe('7')
  expect(await page.inputValue('[data-label="Beta"] input[type="number"]')).toBe('1')
})

test('REQ-ACT-01 redirect answer navigates', async () => {
  await clickCart('Alpha', 'Go')
  await page.waitForURL(shop.url + '/', { timeout: 5000 })
  expect(await total('Alpha')).toBe('1')
})

test('REQ-ACT-01 toast answer shows a toast', async () => {
  await clickCart('Alpha', 'Toast')
  await waitForText('#gx-toaster', 'Saved')
})

test('REQ-ACT-01 no-patch answer keeps the page', async () => {
  await clickCart('Alpha', 'Nothing')
  await Bun.sleep(300)
  expect(await total('Alpha')).toBe('1')
  expect(await page.textContent('#gx-toaster')).toBe('')
})

test('REQ-ACT-10 handler error shows a toast', async () => {
  await clickCart('Alpha', 'Fail')
  await waitForText('#gx-toaster', 'demo action failed')
})

test('REQ-ACT-11 a visible action fills the lazy slot', async () => {
  await page.evaluate(() => {
    document.querySelector('#lazy-slot')?.scrollIntoView({ block: 'center' })
  })
  await waitForText('#lazy-slot', 'loaded')
})

test('REQ-ACT-12 a patch inside a view transition lands', async () => {
  await page.addInitScript(() => {
    // Count startViewTransition calls (REQ-ACT-12).
    ;(window as unknown as { __vt: number }).__vt = 0
    const original = document.startViewTransition?.bind(document)
    if (original) {
      document.startViewTransition = ((...args: Parameters<typeof original>) => {
        ;(window as unknown as { __vt: number }).__vt++
        return original(...args)
      }) as typeof original
    }
  })
  await page.reload()
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  await page.click('button:text-is("Transition")')
  await waitForText('#transition-target', 'done')
  const calls = await page.evaluate(() => (window as unknown as { __vt: number }).__vt)
  expect(calls).toBeGreaterThan(0)
})

test('REQ-ACT-06 the dev runtime reports a duplicate signal scope', async () => {
  const errors: string[] = []
  page.on('console', (m) => {
    if (m.type() === 'error') errors.push(m.text())
  })
  await page.goto(shop.url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  await page.evaluate(() => {
    const meta = document.createElement('meta')
    meta.name = 'gx-dev'
    meta.content = '1'
    document.head.append(meta)
    for (let i = 0; i < 2; i++) {
      const el = document.createElement('div')
      el.setAttribute('data-gx-instance', 'cart.Cart.dup')
      document.body.append(el)
    }
  })
  const deadline = Date.now() + 3000
  while (!errors.some((e) => e.includes('share the scope')) && Date.now() < deadline) {
    await Bun.sleep(50)
  }
  expect(errors.join('\n')).toContain('share the scope')
})

test('REQ-RTE-12 a partial navigation keeps the layout DOM node', async () => {
  await page.evaluate(() => {
    ;(document.querySelector('header') as unknown as { __keep: number }).__keep = 1
  })
  await page.click('a:text-is("About")')
  await page.waitForFunction(() => document.title === 'Gx shop about', undefined, { timeout: 5000 })
  expect(await page.evaluate(() => new URL(location.href).pathname)).toBe('/about')
  expect(
    await page.evaluate(
      () => (document.querySelector('header') as unknown as { __keep?: number }).__keep === 1,
    ),
  ).toBe(true)
  // Back and forward restore the pages (REQ-RTE-12).
  await page.goBack()
  await page.waitForFunction(() => document.title === 'Gx shop home', undefined, { timeout: 5000 })
  expect(await page.textContent('#lazy-slot')).toBe('waiting')
  await page.goForward()
  await page.waitForFunction(() => document.title === 'Gx shop about', undefined, { timeout: 5000 })
  expect(await page.evaluate(() => new URL(location.href).pathname)).toBe('/about')
})

test('REQ-RTE-13 the active link moves after a partial navigation', async () => {
  expect(await page.getAttribute('a:text-is("Home")', 'aria-current')).toBe('page')
  await page.click('a:text-is("About")')
  await page.waitForFunction(() => document.title === 'Gx shop about', undefined, { timeout: 5000 })
  expect(await page.getAttribute('a:text-is("About")', 'aria-current')).toBe('page')
  expect(await page.getAttribute('a:text-is("Home")', 'aria-current')).toBeNull()
})

test('REQ-RTE-12 JS off keeps navigation links plain', async () => {
  const context = await browser.newContext({ javaScriptEnabled: false })
  const plain = await context.newPage()
  await plain.goto(shop.url + '/')
  await plain.click('a:text-is("About")')
  await plain.waitForURL('**/about')
  expect(await plain.title()).toBe('Gx shop about')
  await context.close()
})

