// The stylesheet of each page route (REQ-STY-13): a page links a file with
// only the classes that it can use, and looks the same as with the
// stylesheet of the app.
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
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

const sheet = (): Promise<string> =>
  page.evaluate(() => document.querySelector('link[rel="stylesheet"][href*="/_gx/"]')?.getAttribute('href') ?? '')

// styles returns the computed style of each element of the page, as one
// text for each element.
const styles = (): Promise<string[]> =>
  page.evaluate(() =>
    [...document.querySelectorAll('body, body *')].map((el) => {
      const cs = getComputedStyle(el)
      const out: string[] = [el.tagName + '#' + el.id + '.' + el.className]
      for (let i = 0; i < cs.length; i++) {
        const name = cs[i]
        // A custom property is a token of the theme: a stylesheet has
        // the tokens that its classes use.
        if (!name.startsWith('--')) out.push(name + ':' + cs.getPropertyValue(name))
      }
      return out.join(';')
    }),
  )

// withAppSheet swaps the stylesheet of the page for the stylesheet of the
// app and waits for it.
async function withAppSheet(): Promise<void> {
  await page.evaluate(
    () =>
      new Promise<void>((done, fail) => {
        const link = document.querySelector('link[rel="stylesheet"][href*="/_gx/"]') as HTMLLinkElement
        link.addEventListener('load', () => done())
        link.addEventListener('error', () => fail(new Error('app.css did not load')))
        link.href = '/_gx/app.css'
      }),
  )
}

// same compares the page with its own stylesheet and with app.css.
async function same(what: string): Promise<void> {
  const own = await styles()
  await withAppSheet()
  const app = await styles()
  expect(app.length).toBe(own.length)
  const differ = own.map((s, i) => (s === app[i] ? '' : s.split(';')[0])).filter((s) => s !== '')
  if (differ.length > 0) throw new Error(`${what}: ${differ.length} elements look different with app.css: ${differ.slice(0, 5).join(', ')}`)
}

for (const path of ['/', '/about', '/basket', '/dashboard', '/signup']) {
  test(`REQ-STY-13 ${path} has its own stylesheet and the computed styles of app.css`, async () => {
    // A page with a room keeps a stream open, so the network is never
    // idle.
    await page.goto(shop.url + path, { waitUntil: 'load' })
    await Bun.sleep(300)
    const href = await sheet()
    expect(href).toMatch(/^\/_gx\/css\/app\.[0-9a-f]{12}\.css$/)
    await same(path)
  })
}

test('REQ-STY-13 the stylesheet of the home page is smaller than app.css', async () => {
  await page.goto(shop.url + '/')
  const href = await sheet()
  const own = await (await fetch(shop.url + href)).text()
  const app = await (await fetch(shop.url + '/_gx/app.css')).text()
  expect(own.length).toBeGreaterThan(1000)
  expect(own.length).toBeLessThan(app.length / 2)
})

test('REQ-STY-13 a patch of an action brings no element with a class that has no rule', async () => {
  // The cart of the home page: a patch of the total, and a toast that the
  // server pushes.
  await page.goto(shop.url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  await page.click('[data-label="Alpha"] button:text-is("Add")')
  await page.click('[data-label="Alpha"] button:text-is("Toast")')
  await page.waitForSelector('text=Saved')
  // The toast has a transition when it comes: the page is at rest after it.
  await Bun.sleep(500)
  await same('the home page after two actions')

  // The basket: c.Update sends fragments.
  await page.goto(shop.url + '/basket')
  await page.waitForSelector('#basket-lines')
  await page.click('#basket-line-milk button')
  await page.waitForFunction(() => document.querySelector('#basket-count')?.textContent !== '')
  await Bun.sleep(200)
  await same('the basket page after an update')
})

test('REQ-STY-13 a navigation to a page with a different stylesheet stays in the document and changes the stylesheet', async () => {
  await page.goto(shop.url + '/')
  await page.evaluate(() => ((window as unknown as { sameDocument: boolean }).sameDocument = true))
  const home = await sheet()
  await page.click('nav a:text-is("About")')
  await page.waitForSelector('h1:text-is("About")')
  // The home page and the about page share one stylesheet.
  expect(await sheet()).toBe(home)
  await page.click('nav a:text-is("Sign up")')
  await page.waitForSelector('form')
  await page.waitForFunction((home) => document.querySelector('link[rel="stylesheet"][href*="/_gx/"]')?.getAttribute('href') !== home, home)
  const signup = await sheet()
  expect(signup).toMatch(/^\/_gx\/css\/app\./)
  // The page has one stylesheet of the app, and it is the same document.
  expect(await page.evaluate(() => document.querySelectorAll('link[rel="stylesheet"][href*="/_gx/"]').length)).toBe(1)
  expect(await page.evaluate(() => (window as unknown as { sameDocument?: boolean }).sameDocument)).toBe(true)
  await Bun.sleep(300)
  await same('the signup page after a navigation')
})
