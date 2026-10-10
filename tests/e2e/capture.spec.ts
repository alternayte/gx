// The capture of a fixture in a real browser: the dev client saves the
// props of a component of the page into its fixtures file, and the gallery
// shows the fixture with the HTML of the page (REQ-AI-12).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser } from 'playwright-core'
import { launchBrowser } from './harness'
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const shop = join(repo, 'examples', 'shop')
const cartPackage = 'github.com/alternayte/gx/examples/shop/cart'

let dir: string
let dev: Subprocess
let browser: Browser
let url: string

function freePort(): number {
  const listener = Bun.serve({ port: 0, fetch: () => new Response('') })
  const port = listener.port
  listener.stop(true)
  return port
}

async function waitFor(target: string, timeout = 120000): Promise<string> {
  const deadline = Date.now() + timeout
  for (;;) {
    try {
      const res = await fetch(target)
      if (res.ok) return await res.text()
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`no answer at ${target}`)
    await Bun.sleep(200)
  }
}

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-capture-'))
  cpSync(shop, dir, { recursive: true })
  // The copied module needs an absolute replace to the repo.
  const goMod = join(dir, 'go.mod')
  writeFileSync(goMod, readFileSync(goMod, 'utf8').replace('replace github.com/alternayte/gx => ../..', `replace github.com/alternayte/gx => ${repo}`))
  const port = freePort()
  url = `http://127.0.0.1:${port}`
  dev = spawn(['go', 'run', './cmd/gx', 'dev', '-addr', `127.0.0.1:${port}`, dir], {
    cwd: repo,
    stdout: 'ignore',
    stderr: 'ignore',
    detached: true,
  })
  await waitFor(url + '/')
  browser = await launchBrowser()
}, 180000)

afterAll(async () => {
  await browser?.close()
  if (dev?.pid) {
    try {
      process.kill(-dev.pid, 'SIGTERM')
    } catch {
      dev.kill()
    }
  }
  await Bun.sleep(200)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

test('REQ-AI-12 the dev client saves the props of a component of the page as a fixture, and the gallery shows the same HTML', async () => {
  const fixtures = join(dir, 'cart', 'Cart.fixtures.go')
  expect(readFileSync(fixtures, 'utf8')).not.toContain('FromPage')

  const page = await browser.newPage()
  await page.goto(url + '/')
  await page.click('#gx-dev-capture')
  const dialog = page.locator('#gx-dev-capture-dialog')
  await dialog.locator('select option').first().waitFor({ state: 'attached' })

  // The list holds the components of this page: the two carts and the
  // toasts of the home page, and the shell around them. A component of a
  // different page is not in it.
  const options = await dialog.locator('select option').allTextContents()
  expect(options).toContain(`Cart (${cartPackage})`)
  expect(options.some((o) => o.startsWith('Shell ('))).toBe(true)
  expect(options.some((o) => o.startsWith('Dashboard ('))).toBe(false)

  // A name that is not a fixture name does not go to the server.
  await dialog.locator('select').selectOption({ label: `Cart (${cartPackage})` })
  await dialog.locator('input[name=name]').fill('From Page')
  await dialog.locator('button[type=submit]').click()
  expect(await dialog.locator('[role=status]').getAttribute('data-state')).toBeNull()

  await dialog.locator('input[name=name]').fill('FromPage')
  await dialog.locator('button[type=submit]').click()
  await dialog.locator('[role=status][data-state]').waitFor()
  expect(await dialog.locator('[role=status]').textContent()).toBe('Saved the fixture FromPage in cart/Cart.fixtures.go.')

  // The last cart of the page is Beta: its props are the kept ones.
  const source = readFileSync(fixtures, 'utf8')
  expect(source).toContain('"FromPage": {Label: "Beta", Total: 2, GxKey: "beta"},')
  expect(source).toContain('"Default":')

  // The same name again is an error with a message, and the file stays.
  await dialog.locator('button[type=submit]').click()
  await dialog.locator('[role=status][data-state=error]').waitFor()
  expect(await dialog.locator('[role=status]').textContent()).toContain('the fixture "FromPage" exists')
  expect(readFileSync(fixtures, 'utf8')).toBe(source)
  await page.close()

  // The dev server builds the app again. The gallery then has the
  // fixture, with the HTML that the page has for the cart.
  const q = new URLSearchParams({ component: 'Cart', name: 'FromPage', package: cartPackage })
  const doc = await waitFor(`${url}/_gx/gallery/fixture?${q}`)
  const inGallery = doc.match(/<main id="gx-fixture"[^>]*>([\s\S]*)<\/main>/)?.[1]
  const home = await waitFor(url + '/')
  const onPage = home.match(/<div class="cart [^>]*data-label="Beta"[\s\S]*?<\/div>/)?.[0]
  expect(onPage).toBeDefined()
  expect(onPage).toContain('Beta total:')
  // The cut of the page starts at the root element; the fixture has the
  // white space of the template around it.
  expect(inGallery?.trim()).toBe(onPage!)
}, 240000)
