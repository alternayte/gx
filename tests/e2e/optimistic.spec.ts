// Optimistic updates in a real browser (REQ-ACT-18): the page shows the new
// value before the answer, and the value of before comes back when the
// action fails.
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { type Browser, type BrowserContext, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let context: BrowserContext
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
  context = await browser.newContext()
  page = await context.newPage()
  await page.goto(shop.url + '/basket')
  await page.waitForSelector('#stars')
  // history records each text that the counter shows.
  await page.evaluate(() => {
    const el = document.querySelector('#stars')!
    const seen: string[] = [el.textContent ?? '']
    ;(window as unknown as { seen: string[] }).seen = seen
    new MutationObserver(() => seen.push(el.textContent ?? '')).observe(el, { childList: true, characterData: true, subtree: true })
  })
})

afterEach(async () => {
  try {
    await context?.close()
  } catch {
    // already closed
  }
})

const history = (): Promise<string[]> => page.evaluate(() => (window as unknown as { seen: string[] }).seen)

// settle waits until the counter shows want and returns what it showed.
async function settle(want: string, timeout = 8000): Promise<string[]> {
  const deadline = Date.now() + timeout
  for (;;) {
    const seen = await history()
    if (seen[seen.length - 1] === want && seen.length > 1) return seen
    if (Date.now() > deadline) throw new Error(`timeout: the counter showed ${JSON.stringify(seen)}, want the last value ${want}`)
    await Bun.sleep(50)
  }
}

test('REQ-ACT-18 the page shows the new value before the answer, and keeps it', async () => {
  const answer = page.waitForResponse((res) => res.url().endsWith('/basket/star'))
  await page.click('#star-ok')
  expect((await answer).status()).toBe(204)
  expect(await settle('1')).toEqual(['0', '1'])
  await Bun.sleep(300)
  expect(await history()).toEqual(['0', '1'])
})

test('REQ-ACT-18 an action error puts the value back', async () => {
  await page.click('#star-fail')
  // The value shows while the server works, then the value of before.
  expect(await settle('0')).toEqual(['0', '1', '0'])
  await page.waitForSelector('text=the shop did not save the star')
})

test('REQ-ACT-18 with the network off, the value moves and then moves back', async () => {
  await context.setOffline(true)
  await page.click('#star-ok')
  expect(await settle('0')).toEqual(['0', '1', '0'])
  // The page works again when the network is back.
  await context.setOffline(false)
  await page.click('#star-ok')
  expect(await settle('1')).toEqual(['0', '1', '0', '1'])
})

test('REQ-ACT-18 the answer of the server wins', async () => {
  await page.click('#star-set')
  const seen = await settle('100')
  expect(seen[0]).toBe('0')
  expect(seen[seen.length - 1]).toBe('100')
})
