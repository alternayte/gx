// The shop under a strict Content-Security-Policy with nonces, in a real
// browser (SI-11).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser

beforeAll(async () => {
  shop = await startShop({ csp: true, gxdev: true })
  browser = await launchBrowser()
}, 180000)

afterAll(async () => {
  await Bun.sleep(300)
  shop?.stop()
})

// open loads a path and records what the policy blocked.
async function open(path: string): Promise<{ page: Page; policy: string; blocked: string[] }> {
  const page = await browser.newPage()
  const blocked: string[] = []
  page.on('console', (msg) => {
    if (msg.text().includes('Content Security Policy')) blocked.push(msg.text())
  })
  await page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (e) => {
      console.error(`Content Security Policy violation: ${e.violatedDirective} ${e.blockedURI}`)
    })
  })
  const res = await page.goto(shop.url + path)
  const policy = res?.headers()['content-security-policy'] ?? ''
  return { page, policy, blocked }
}

test('SI-11 every script of a page carries the nonce of its policy', async () => {
  const { page, policy, blocked } = await open('/')
  const nonce = /'nonce-([^']+)'/.exec(policy)?.[1] ?? ''
  expect(nonce.length).toBeGreaterThan(15)
  expect(policy).toContain(`script-src 'nonce-${nonce}' 'strict-dynamic'`)
  expect(policy).not.toContain('unsafe-inline')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  const nonces = await page.evaluate(() => Array.from(document.scripts, (s) => s.nonce))
  expect(nonces.length).toBeGreaterThan(1)
  for (const n of nonces) expect(n).toBe(nonce)
  expect(blocked).toEqual([])
  await page.close()
})

test('SI-11 a second response has another nonce', async () => {
  const a = await open('/')
  const b = await open('/')
  expect(a.policy).not.toBe(b.policy)
  await a.page.close()
  await b.page.close()
})

test('SI-11 signals, actions and navigation work under the policy', async () => {
  const { page, blocked } = await open('/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  await page.click('[data-label="Alpha"] button:text-is("Add")')
  await page.waitForFunction(
    () => document.querySelector('[data-label="Alpha"] span[id^="cart-total"]')?.textContent === '10',
    undefined,
    { timeout: 5000 },
  )
  await page.click('[data-label="Alpha"] button:text-is("Toast")')
  await page.waitForFunction(() => document.querySelector('#gx-toaster')?.textContent?.includes('Saved'), undefined, {
    timeout: 5000,
  })
  await page.click('a[href="/about"]')
  await page.waitForURL(shop.url + '/about', { timeout: 5000 })
  expect(blocked).toEqual([])
  await page.close()
})

test('SI-11 the policy blocks an inline script with no nonce', async () => {
  const { page, blocked } = await open('/about')
  await page.evaluate(() => {
    // An HTML injection: the parser of innerHTML never runs scripts, so
    // the attack is an event handler attribute.
    const div = document.createElement('div')
    div.innerHTML = '<img src="/missing.png" onerror="window.injected = true">'
    document.body.append(div)
  })
  await page.waitForFunction(() => true)
  await Bun.sleep(500)
  expect(await page.evaluate(() => (window as unknown as { injected?: boolean }).injected)).toBeUndefined()
  expect(blocked.length).toBeGreaterThan(0)
  await page.close()
})

test('SI-11 the dev gallery, with its inline script, works under the policy', async () => {
  const { page, blocked } = await open('/_gx/gallery')
  await page.waitForSelector('script', { state: 'attached' })
  const nonces = await page.evaluate(() => Array.from(document.scripts, (s) => s.nonce))
  expect(nonces.length).toBeGreaterThan(0)
  for (const n of nonces) expect(n.length).toBeGreaterThan(15)
  // The inline gallery script runs: it sets the theme class.
  await page.click('[data-gallery-theme="dark"]')
  expect(await page.evaluate(() => document.documentElement.classList.contains('dark'))).toBe(true)
  expect(blocked).toEqual([])
  await page.close()
})
