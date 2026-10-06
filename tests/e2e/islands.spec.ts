// The islands of the example shop in a real browser: the load strategies
// (REQ-ISL-05), the shared chunk (REQ-ISL-03) and a signal of the page
// inside an island (REQ-ISL-04).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser

beforeAll(async () => {
  shop = await startShop()
  browser = await launchBrowser()
}, 300000)

afterAll(() => {
  shop?.stop()
})

const island = (name: string): string => `gx-island[name$="/${name}"]`

// isMounted waits until the island runs.
const isMounted = (page: Page, name: string): Promise<unknown> =>
  page.waitForFunction((sel) => document.querySelector(sel)?.matches(':state(mounted)'), island(name))

// dashboard opens the dashboard and records each request for an island
// file, in order.
async function dashboard(width = 1000): Promise<{ page: Page; files: string[] }> {
  const page = await browser.newPage({ viewport: { width, height: 700 } })
  const files: string[] = []
  page.on('request', (req) => {
    const i = req.url().indexOf('/_gx/islands/')
    if (i >= 0) files.push(req.url().slice(i + '/_gx/islands/'.length))
  })
  await page.goto(shop.url + '/dashboard')
  return { page, files }
}

const requested = (files: string[], name: string): number => files.findIndex((f) => f.includes(`/dashboard/${name}-`))

test('REQ-ISL-05 each load strategy fetches the island file at its time', async () => {
  const { page, files } = await dashboard()
  try {
    // eager and idle load with no user action.
    await isMounted(page, 'BarChart')
    await isMounted(page, 'Stepper')
    await isMounted(page, 'Sparkline')
    expect(requested(files, 'BarChart')).toBeGreaterThanOrEqual(0)
    expect(requested(files, 'Stepper')).toBeGreaterThanOrEqual(0)
    // idle waits for the browser, so it comes after each eager island.
    expect(requested(files, 'Sparkline')).toBeGreaterThan(requested(files, 'BarChart'))
    expect(requested(files, 'Sparkline')).toBeGreaterThan(requested(files, 'Stepper'))

    // visible is the default: the legend is 2400 px down and not loaded.
    await page.waitForTimeout(300)
    expect(requested(files, 'Legend')).toBe(-1)
    expect(requested(files, 'WideTable')).toBe(-1)
    expect(await page.evaluate((sel) => document.querySelector(sel)!.matches(':state(mounted)'), island('Legend'))).toBe(false)

    await page.locator(island('Legend')).scrollIntoViewIfNeeded()
    await isMounted(page, 'Legend')
    expect(requested(files, 'Legend')).toBeGreaterThanOrEqual(0)
    expect(await page.textContent(island('Legend'))).toBe('Legend: Revenue, Users')

    // media waits for its query, also when the element is on the screen.
    await page.locator(island('WideTable')).scrollIntoViewIfNeeded()
    await page.waitForTimeout(300)
    expect(requested(files, 'WideTable')).toBe(-1)
    await page.setViewportSize({ width: 1700, height: 700 })
    await isMounted(page, 'WideTable')
    expect(requested(files, 'WideTable')).toBeGreaterThanOrEqual(0)
    expect(await page.locator(island('WideTable') + ' tr').count()).toBe(4)
  } finally {
    await page.close()
  }
})

test('REQ-ISL-05 a media island loads at once when its query matches', async () => {
  const { page, files } = await dashboard(1700)
  try {
    await isMounted(page, 'WideTable')
    expect(requested(files, 'WideTable')).toBeGreaterThanOrEqual(0)
    expect(requested(files, 'Legend')).toBe(-1)
  } finally {
    await page.close()
  }
})

test('REQ-ISL-03 two islands load their shared chunk one time', async () => {
  const { page, files } = await dashboard()
  try {
    await isMounted(page, 'BarChart')
    await isMounted(page, 'Sparkline')
    expect(await page.locator(island('BarChart') + ' [data-bars="bar"] [data-bar]').count()).toBe(4)
    expect(await page.locator(island('Sparkline') + ' [data-bars="spark"] [data-bar]').count()).toBe(4)
    const chunks = files.filter((f) => f.startsWith('chunks/'))
    expect(chunks).toHaveLength(1)
    // Each file name holds a content hash, and the server lets the browser
    // keep the file.
    const res = await page.request.get(shop.url + '/_gx/islands/' + chunks[0])
    expect(res.headers()['cache-control']).toContain('immutable')
    expect(await res.text()).toContain('data-bars')
    for (const f of files) expect(f).toMatch(/-[0-9A-Z]{8}\.js$/)
  } finally {
    await page.close()
  }
})

test('REQ-ISL-04 an island reads and writes a signal of the page', async () => {
  const { page } = await dashboard()
  try {
    await isMounted(page, 'Stepper')
    const button = page.locator(island('Stepper') + ' button')
    expect(await button.textContent()).toBe('Quantity 1, add one')
    await button.click()
    await page.waitForFunction(() => document.querySelector<HTMLInputElement>('input[type=number]')!.value === '2')
    await page.fill('input[type=number]', '7')
    await page.waitForFunction((sel) => document.querySelector(sel)!.textContent === 'Quantity 7, add one', island('Stepper') + ' button')
  } finally {
    await page.close()
  }
})
