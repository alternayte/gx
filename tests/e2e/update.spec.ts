// Automatic updates: an action gives c.Update the component from the new
// data, and only the fragments that changed go to the browser (M22).
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
  await Bun.sleep(400)
  shop?.stop()
})

beforeEach(async () => {
  page = await browser.newPage()
  await page.goto(shop.url + '/basket')
  await page.waitForSelector('#basket-lines')
  // Each test starts from the first lines of the basket.
  await page.click('button:text-is("Reset")')
  await waitForText('#basket-count', '4')
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

async function waitForText(selector: string, want: string, timeout = 5000): Promise<void> {
  const deadline = Date.now() + timeout
  let last = ''
  for (;;) {
    last = (await page.textContent(selector)) ?? ''
    if (last.trim() === want) return
    if (Date.now() > deadline) throw new Error(`timeout: ${selector} = ${JSON.stringify(last)}, want ${JSON.stringify(want)}`)
    await Bun.sleep(50)
  }
}

const hashes = (): Promise<Record<string, string>> =>
  page.evaluate(() =>
    Object.fromEntries([...document.querySelectorAll('[data-gx-h][id]')].map((el) => [el.id, el.getAttribute('data-gx-h') ?? ''])),
  )

test('REQ-ACT-15 the page has a hash for each fragment, and an action sends them', async () => {
  const before = await hashes()
  expect(Object.keys(before).sort()).toEqual(['basket-count', 'basket-line-milk', 'basket-line-rice', 'basket-line-tea', 'basket-lines'])
  for (const hash of Object.values(before)) expect(hash).toMatch(/^[0-9a-f]{8}$/)

  const sent = page.waitForRequest((req) => req.url().endsWith('/basket/bump/milk'))
  await page.click('#basket-line-milk button')
  const header = (await sent).headers()['gx-fragments'] ?? ''
  const got = Object.fromEntries(header.split(',').map((part) => part.split('=')))
  expect(got).toEqual(before)
})

test('REQ-ACT-16 c.Update changes one line, and the answer holds only the fragments that changed', async () => {
  const before = await hashes()
  // A mark on each line shows which elements the answer replaced or morphed.
  const answer = page.waitForResponse((res) => res.url().endsWith('/basket/bump/milk'))
  await page.click('#basket-line-milk button')
  const body = await (await answer).text()
  await waitForText('#basket-line-milk b', '3')
  await waitForText('#basket-count', '5')

  // The answer names the line and the count, and no other fragment.
  expect(body).toContain('basket-line-milk')
  expect(body).toContain('basket-count')
  expect(body).not.toContain('basket-line-tea')
  expect(body).not.toContain('basket-line-rice')
  expect(body).not.toContain('id="basket-lines"')
  expect(body).not.toContain('Basket</h1>')

  // The page has the new hashes of the two fragments; the others stay.
  const after = await hashes()
  expect(after['basket-line-milk']).not.toBe(before['basket-line-milk'])
  expect(after['basket-count']).not.toBe(before['basket-count'])
  expect(after['basket-line-tea']).toBe(before['basket-line-tea'])
  expect(after['basket-line-rice']).toBe(before['basket-line-rice'])
  expect(after['basket-lines']).toBe(before['basket-lines'])

  // The next action sends the new hashes, so the answer is again two
  // fragments.
  const second = page.waitForResponse((res) => res.url().endsWith('/basket/bump/tea'))
  await page.click('#basket-line-tea button')
  const secondBody = await (await second).text()
  await waitForText('#basket-line-tea b', '2')
  expect(secondBody).toContain('basket-line-tea')
  expect(secondBody).not.toContain('basket-line-milk')
})
