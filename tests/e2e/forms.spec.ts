// Forms: the example signup slice, driven against the real app (M5).
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop()
  browser = await launchBrowser()
}), 180000

afterAll(async () => {
  await Bun.sleep(300)
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
  await page.fill('input[name="address.street"]', 'Main')
  await page.check('input[name=terms]')
}

test('REQ-FRM-02 a rule failure re-renders with the value and the error', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'a@b.co', age: '7', terms: 'on', 'address.street': 'Main', gx_csrf: await csrf() },
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
    form: { email: '', age: '20', terms: 'on', 'address.street': 'Main', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(422)
  expect(await res.text()).toContain('This field is required.')
})

test('REQ-FRM-02 a handler FieldError re-renders with the key', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'taken@example.com', age: '20', terms: 'on', 'address.street': 'Main', gx_csrf: await csrf() },
    maxRedirects: 0,
  })
  expect(res.status()).toBe(422)
  expect(await res.text()).toContain('email.taken')
})

test('REQ-FRM-02 a valid submit redirects with 303', async () => {
  await page.goto(shop.url + '/signup')
  const res = await page.request.post(shop.url + '/signup', {
    form: { email: 'a@b.co', age: '20', terms: 'on', 'address.street': 'Main', gx_csrf: await csrf() },
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
  await page.click('button:text-is("Create account")')
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
  await plain.fill('input[name="address.street"]', 'Main')
  await plain.check('input[name=terms]')
  const [res] = await Promise.all([plain.waitForNavigation(), plain.click('button:text-is("Create account")')])
  expect(res?.status()).toBe(422)
  expect(await plain.textContent('#signup-email-error')).toContain('email.taken')
  expect(await plain.inputValue('input[name=email]')).toBe('taken@example.com')
  expect(await plain.inputValue('input[name=age]')).toBe('20')
  await context.close()
})

test('REQ-FRM-11 a field error wires aria and the summary takes focus', async () => {
  await page.goto(shop.url + '/signup')
  await fillSignup('taken@example.com', '20')
  await page.click('button:text-is("Create account")')
  await waitForText('#signup-email-error', 'email.taken')
  expect(await page.getAttribute('input[name=email]', 'aria-invalid')).toBe('true')
  expect(await page.getAttribute('input[name=email]', 'aria-describedby')).toContain('signup-email-error')
  expect(await page.getAttribute('label[for=signup-email]', 'for')).toBe('signup-email')
  expect(await page.evaluate(() => document.activeElement?.id)).toBe('signup-errors')
})

test('REQ-FRM-11 a hint is described on the control', async () => {
  await page.goto(shop.url + '/signup')
  expect(await page.getAttribute('input[name=email]', 'aria-describedby')).toContain('signup-email-hint')
  expect((await page.textContent('#signup-email-hint')) ?? '').toContain('never share')
})

test('REQ-FRM-09 a small upload reaches the handler', async () => {
  await page.goto(shop.url + '/signup')
  await fillSignup('a@b.co', '20')
  await page.setInputFiles('input[name=avatar]', { name: 'a.png', mimeType: 'image/png', buffer: Buffer.from('png') })
  await Promise.all([page.waitForURL(shop.url + '/', { timeout: 5000 }), page.click('button:text-is("Create account")')])
  expect(new URL(page.url()).pathname).toBe('/')
})

test('REQ-FRM-09 an oversize upload is a field error before the handler', async () => {
  await page.goto(shop.url + '/signup')
  await fillSignup('a@b.co', '20')
  await page.setInputFiles('input[name=avatar]', { name: 'big.png', mimeType: 'image/png', buffer: Buffer.alloc(1_200_000, 1) })
  await page.click('button:text-is("Create account")')
  await waitForText('#signup-avatar-error', 'too large')
  expect(new URL(page.url()).pathname).toBe('/signup')
})

test('REQ-FRM-09 an upload over the body cap is rejected with 413', async () => {
  await page.goto(shop.url + '/signup')
  await fillSignup('a@b.co', '20')
  await page.setInputFiles('input[name=avatar]', { name: 'huge.png', mimeType: 'image/png', buffer: Buffer.alloc(3_000_000, 1) })
  const [res] = await Promise.all([page.waitForNavigation(), page.click('button:text-is("Create account")')])
  expect(res?.status()).toBe(413)
})

test('REQ-FRM-08 add and remove repeated rows through actions', async () => {
  await page.goto(shop.url + '/signup')
  await page.click('button:text-is("Add address")')
  await page.waitForSelector('input[name="addresses[0].street"]')
  await page.fill('input[name="addresses[0].street"]', 'A0')
  await page.click('button:text-is("Add address")')
  await page.waitForSelector('input[name="addresses[1].street"]')
  await page.fill('input[name="addresses[1].street"]', 'A1')
  await Bun.sleep(200)
  expect(await page.inputValue('input[name="addresses[0].street"]')).toBe('A0')
  await page.click('[data-index="0"] button:text-is("Remove")')
  await page.waitForFunction(
    () => document.querySelectorAll('input[name^="addresses["][name$=".street"]').length === 1,
    undefined,
    { timeout: 5000 },
  )
  expect(await page.inputValue('input[name="addresses[0].street"]')).toBe('A1')
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
  await plain.fill('input[name="address.street"]', 'Main')
  await plain.check('input[name=terms]')
  await Promise.all([plain.waitForNavigation(), plain.click('button:text-is("Create account")')])
  expect(new URL(plain.url()).pathname).toBe('/')
  await context.close()
})
