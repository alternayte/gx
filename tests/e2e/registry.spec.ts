// The registry component set, driven in a real browser (M10).
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

test('REQ-REG-10 the data table pages over 10,000 rows', async () => {
  await page.goto(shop.url + '/datatable')
  const rows = page.locator('table tbody tr')
  expect(await rows.count()).toBe(25)
  expect(await page.textContent('body')).toContain('1–25 of 10000')

  // A sort header is a typed GET link.
  await page.click('th a:has-text("Total")')
  await page.waitForURL(/sort=total/)
  await page.waitForFunction(() => document.querySelectorAll('table tbody tr').length === 25)
  expect(await page.getAttribute('th[aria-sort]', 'aria-sort')).toBe('ascending')

  // Paging shows the next 25 of 10,000.
  await page.click('a[aria-label="Go to next page"]')
  await page.waitForURL(/page=2/)
  await page.waitForFunction(() => document.body.innerText.includes('26–50 of 10000'))
  expect(await rows.count()).toBe(25)
})

test('REQ-REG-10 the filter is a typed GET request', async () => {
  await page.goto(shop.url + '/datatable')
  await page.fill('input[name="q"]', 'Item 0001')
  await page.press('input[name="q"]', 'Enter')
  await page.waitForURL(/q=/)
  await page.waitForFunction(() => document.body.innerText.includes('of 10'))
  const rows = page.locator('table tbody tr')
  const count = await rows.count()
  expect(count).toBeGreaterThan(0)
  expect(count).toBeLessThanOrEqual(25)
  expect(await page.textContent('body')).toContain('of 10')
})
