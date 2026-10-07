// The app that `gx init` writes, in a real browser: the signal, the action
// and the patch of the example slice work as written (REQ-DEV-10).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser } from 'playwright-core'
import { freePort, launchBrowser } from './harness'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))

let parent: string
let server: Subprocess
let browser: Browser
let url: string

async function run(argv: string[], cwd: string): Promise<void> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdin: 'ignore', stdout: 'ignore', stderr: 'pipe' })
  if ((await proc.exited) !== 0) {
    throw new Error(`${argv.join(' ')} failed: ${await new Response(proc.stderr).text()}`)
  }
}

beforeAll(async () => {
  parent = mkdtempSync(join(tmpdir(), 'gx-init-'))
  const app = join(parent, 'acme')
  await run(['go', 'run', './cmd/gx', 'init', '--adapter', 'datastar', '--module', 'example.com/acme', '--replace', repo, app], repo)
  await run(['go', 'run', './cmd/gx', 'new', 'slice', 'shop', app], repo)
  const bin = join(parent, process.platform === 'win32' ? 'acme.exe' : 'acme-bin')
  await run(['go', 'build', '-o', bin, './cmd/app'], app)
  const port = await freePort()
  url = `http://127.0.0.1:${port}`
  server = spawn([bin], { cwd: app, env: { ...process.env, GX_DEV_ADDR: `127.0.0.1:${port}` }, stdout: 'ignore', stderr: 'ignore' })
  const deadline = Date.now() + 20000
  for (;;) {
    try {
      if ((await fetch(url + '/')).ok) break
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error('the scaffolded app did not start')
    await Bun.sleep(100)
  }
  browser = await launchBrowser()
}, 240000)

afterAll(async () => {
  await Bun.sleep(200)
  server?.kill()
  if (parent) rmSync(parent, { recursive: true, force: true })
})

test('REQ-DEV-10 the example slice of a new app counts in the browser and saves on the server', async () => {
  const page = await browser.newPage()
  const errors: string[] = []
  page.on('pageerror', (err) => errors.push(String(err)))
  await page.goto(url + '/')
  expect(await page.textContent('h1')).toBe('Welcome to acme')
  await page.click('button:text-is("Add one")')
  await page.click('button:text-is("Add one")')
  await page.waitForFunction(() => document.querySelector('section span')?.textContent === '2')
  await page.click('button:text-is("Save")')
  await page.waitForFunction(() => document.querySelector('section p:last-child')?.textContent?.includes('Saved on the server: 2'))
  // The signal survives the patch of the saved text.
  expect(await page.textContent('section span')).toBe('2')
  expect(errors).toEqual([])
  await page.close()
})

test('REQ-DEV-10 a new slice is mounted and an unknown path answers 404', async () => {
  const page = await browser.newPage()
  const res = await page.goto(url + '/shop')
  expect(res?.status()).toBe(200)
  expect(await page.textContent('h1')).toBe('Shop')
  // The layout wraps the new page too.
  expect(await page.locator('header nav a').first().textContent()).toBe('Acme')
  expect((await page.goto(url + '/no-such-page'))?.status()).toBe(404)
  await page.close()
})
