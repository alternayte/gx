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

async function fillSignup(email: string, age: string): Promise<void> {
  await page.fill('input[name=email]', email)
  await page.fill('input[name=age]', age)
  await page.check('input[name=terms]')
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

test('REQ-FRM-05 with the adapter only the form is patched', async () => {
  await page.goto(shop.url + '/signup')
  await page.evaluate(() => {
    ;(document.querySelector('h1') as unknown as { __keep: number }).__keep = 1
  })
  await fillSignup('taken@example.com', '20')
  await page.click('button[type=submit]')
  await waitForText('#signup-email-error', 'email.taken')
  expect(await page.evaluate(() => (document.querySelector('h1') as unknown as { __keep?: number }).__keep === 1)).toBe(true)
  expect(new URL(page.url()).pathname).toBe('/signup')
  expect(await page.inputValue('input[name=email]')).toBe('taken@example.com')
})

test('REQ-FRM-05 JS off: an invalid submit answers 422 with values and errors', async () => {
  const context = await browser.newContext({ javaScriptEnabled: false })
  const plain = await context.newPage()
  await plain.goto(shop.url + '/signup')
  await plain.fill('input[name=email]', 'taken@example.com')
  await plain.fill('input[name=age]', '20')
  await plain.check('input[name=terms]')
  const [res] = await Promise.all([plain.waitForNavigation(), plain.click('button[type=submit]')])
  expect(res?.status()).toBe(422)
  expect(await plain.textContent('#signup-email-error')).toContain('email.taken')
  expect(await plain.inputValue('input[name=email]')).toBe('taken@example.com')
  expect(await plain.inputValue('input[name=age]')).toBe('20')
  await context.close()
})

test('REQ-FRM-06 blur validation patches only that field', async () => {
  await page.goto(shop.url + '/signup')
  await page.evaluate(() => {
    ;(document.querySelector('h1') as unknown as { __keep: number }).__keep = 1
  })
  await page.fill('input[name=email]', 'nope')
  await page.focus('input[name=age]')
  await waitForText('#signup-email-error', 'valid email')
  expect(await page.textContent('#signup-age-error')).toBe('')
  expect(await page.evaluate(() => (document.querySelector('h1') as unknown as { __keep?: number }).__keep === 1)).toBe(true)
})

test('REQ-FRM-06 input validation clears the error on fix', async () => {
  await page.goto(shop.url + '/signup')
  const age = 'input[name=age]'
  await page.fill(age, '7')
  await waitForText('#signup-age-error', 'at least 18')
  await page.fill(age, '20')
  const deadline = Date.now() + 5000
  while ((await page.textContent('#signup-age-error')) !== '') {
    if (Date.now() > deadline) throw new Error('the age error did not clear')
    await Bun.sleep(50)
  }
})

test('REQ-FRM-05 JS off: a valid submit redirects with 303', async () => {
  const context = await browser.newContext({ javaScriptEnabled: false })
  const plain = await context.newPage()
  await plain.goto(shop.url + '/signup')
  await plain.fill('input[name=email]', 'a@b.co')
  await plain.fill('input[name=age]', '20')
  await plain.check('input[name=terms]')
  await Promise.all([plain.waitForNavigation(), plain.click('button[type=submit]')])
  expect(new URL(plain.url()).pathname).toBe('/')
  await context.close()
})
