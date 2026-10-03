// Forms: the example signup slice, driven against the real app (M5).
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
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

// csrf reads the token cookie the page set (SI-03).
async function csrf(): Promise<string> {
  return (await page.context().cookies()).find((c) => c.name === 'gx_csrf')?.value ?? ''
}

test('REQ-FRM-02 a rule failure re-renders with the value and the error', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'a@b.co', age: '7', terms: 'on', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(422)
  const body = await res.text()
  expect(body).toContain('id="signup-age-error"')
  expect(body).toContain('too small')
  expect(body).toContain('value="a@b.co"')
  expect(body).toContain('value="7"')
})

test('REQ-FRM-02 the handler runs only when the rules pass', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: '', age: '20', terms: 'on', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(422)
  expect(await res.text()).toContain('This field is required.')
})

test('REQ-FRM-02 a handler FieldError re-renders with the key', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'taken@example.com', age: '20', terms: 'on', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(422)
  expect(await res.text()).toContain('email.taken')
})

test('REQ-FRM-02 a valid submit redirects with 303', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'a@b.co', age: '20', terms: 'on', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(303)
  expect(res.headers()['location']).toBe('/')
})
