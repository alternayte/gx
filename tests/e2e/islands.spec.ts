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

// A server patch of a fragment that holds islands (REQ-ISL-06). The morph
// keeps what each island put in the page; the props change on the element.
test('REQ-ISL-06 an island with update keeps its state across a fragment patch', async () => {
  const { page } = await dashboard()
  try {
    await isMounted(page, 'BarChart')
    await isMounted(page, 'Sparkline')
    const zoom = page.locator(island('BarChart') + ' [data-zoom]')
    await zoom.click()
    await zoom.click()
    expect(await zoom.getAttribute('data-zoom')).toBe('3')
    // Mark the nodes, so the test can see that the morph kept them.
    await page.evaluate((sel) => {
      const root = document.querySelector(sel + ' [data-gx-island-root]') as HTMLElement & { marked?: boolean }
      root.marked = true
      ;(root.querySelector('[data-zoom]') as HTMLElement & { marked?: boolean }).marked = true
    }, island('BarChart'))
    expect(await page.textContent(island('BarChart') + ' h2')).toBe('Revenue, round 0')
    expect(await page.textContent(island('BarChart') + ' [data-bar="Jan"]')).toBe('Jan 10')

    await page.click('#charts-panel > button')
    await page.waitForFunction((sel) => document.querySelector(sel + ' h2')?.textContent === 'Revenue, round 1', island('BarChart'))

    // New props reached the island through its update export.
    expect(await page.textContent(island('BarChart') + ' [data-bar="Jan"]')).toBe('Jan 11')
    expect(await page.getAttribute('#charts-panel', 'data-round')).toBe('1')
    // The zoom level, the nodes and the mount count are the old ones.
    expect(await zoom.getAttribute('data-zoom')).toBe('3')
    expect(
      await page.evaluate((sel) => {
        const root = document.querySelector(sel + ' [data-gx-island-root]') as HTMLElement & { marked?: boolean }
        return [root.marked, (root.querySelector('[data-zoom]') as HTMLElement & { marked?: boolean }).marked, root.getAttribute('data-mounts')]
      }, island('BarChart')),
    ).toEqual([true, true, '1'])
    expect(await page.evaluate((sel) => document.querySelector(sel)!.matches(':state(mounted)'), island('BarChart'))).toBe(true)

    // A second patch works the same way.
    await page.click('#charts-panel > button')
    await page.waitForFunction((sel) => document.querySelector(sel + ' h2')?.textContent === 'Revenue, round 2', island('BarChart'))
    expect(await zoom.getAttribute('data-zoom')).toBe('3')
  } finally {
    await page.close()
  }
})

test('REQ-ISL-06 an island with no update mounts again with the new props', async () => {
  const { page } = await dashboard()
  try {
    await isMounted(page, 'Sparkline')
    const root = island('Sparkline') + ' [data-gx-island-root]'
    expect(await page.getAttribute(root, 'data-mounts')).toBe('1')
    expect(await page.textContent(island('Sparkline') + ' [data-bar="Apr"]')).toBe('Apr 40')
    await page.click('#charts-panel > button')
    await page.waitForFunction((sel) => document.querySelector(sel)?.getAttribute('data-mounts') === '2', root)
    expect(await page.textContent(island('Sparkline') + ' [data-bar="Apr"]')).toBe('Apr 41')
    expect(await page.locator(island('Sparkline') + ' [data-bar]').count()).toBe(4)
  } finally {
    await page.close()
  }
})

test('REQ-ISL-06 a patch that does not change the props leaves an island alone', async () => {
  const { page } = await dashboard()
  try {
    await isMounted(page, 'Stepper')
    await isMounted(page, 'Sparkline')
    await page.click(island('Stepper') + ' button')
    await page.click('#charts-panel > button')
    await page.waitForFunction(() => document.querySelector('#charts-panel')?.getAttribute('data-round') === '1')
    // The stepper is outside the patched fragment and keeps its signal.
    expect(await page.textContent(island('Stepper') + ' button')).toBe('Quantity 2, add one')
  } finally {
    await page.close()
  }
})

test('REQ-ISL-06 islands unmount and mount across a morph navigation', async () => {
  const { page, files } = await dashboard()
  try {
    await isMounted(page, 'BarChart')
    await page.evaluate(() => ((window as unknown as { sameDocument: boolean }).sameDocument = true))
    await page.click('nav a[href="/about"]')
    await page.waitForFunction(() => document.querySelector('gx-island') === null && location.pathname === '/about')
    await page.click('nav a[href="/dashboard"]')
    await isMounted(page, 'BarChart')
    await isMounted(page, 'Stepper')
    expect(await page.evaluate(() => (window as unknown as { sameDocument?: boolean }).sameDocument)).toBe(true)
    expect(await page.textContent(island('BarChart') + ' h2')).toBe('Revenue, round 0')
    expect(await page.textContent(island('Stepper') + ' button')).toBe('Quantity 1, add one')
    // The browser keeps the module: the second visit asks for no file again.
    expect(files.filter((f) => f.includes('/dashboard/BarChart-'))).toHaveLength(1)
  } finally {
    await page.close()
  }
})

// chartlib.ts imports d3-scale, which `gx pin` vendored into js/vendor/ of
// the shop. No node tool takes part in the build (REQ-ISL-07).
test('REQ-ISL-07 an island runs a pinned npm package', async () => {
  const { page, files } = await dashboard()
  try {
    await isMounted(page, 'BarChart')
    const width = (label: string): Promise<string> =>
      page.evaluate((sel) => (document.querySelector(sel) as HTMLElement).style.width, `${island('BarChart')} [data-bar="${label}"]`)
    // scaleLinear maps the largest value, 40, to 200 px.
    expect(await width('Apr')).toBe('200px')
    expect(await width('Jan')).toBe('50px')
    // The package is part of the bundle: the browser asks for no other host.
    const res = await page.request.get(shop.url + '/_gx/islands/' + files.find((f) => f.startsWith('chunks/')))
    expect(await res.text()).not.toContain('cdn.jsdelivr.net')
  } finally {
    await page.close()
  }
})

// The first page has no island, so its head has no island loader. The core
// runtime loads it when the navigation brings the islands in.
test('REQ-ISL-04 a morph navigation from a page with no island mounts the islands', async () => {
  const page = await browser.newPage({ viewport: { width: 1000, height: 700 } })
  try {
    await page.goto(shop.url + '/about')
    expect(await page.evaluate(() => [...document.scripts].some((s) => s.src.endsWith('/_gx/island.js')))).toBe(false)
    await page.evaluate(() => ((window as unknown as { sameDocument: boolean }).sameDocument = true))
    await page.click('nav a[href="/dashboard"]')
    await isMounted(page, 'BarChart')
    await isMounted(page, 'Stepper')
    expect(await page.evaluate(() => (window as unknown as { sameDocument?: boolean }).sameDocument)).toBe(true)
    expect(await page.textContent(island('Stepper') + ' button')).toBe('Quantity 1, add one')
  } finally {
    await page.close()
  }
})
