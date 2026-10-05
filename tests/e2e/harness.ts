// The gx e2e harness. It builds the example shop and starts it on a free
// port. Bun and Playwright are for the repo's own tests only (G3).
import { spawn } from 'bun'
import { chromium, type Browser } from 'playwright-core'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const shopDir = fileURLToPath(new URL('../../examples/shop', import.meta.url))

// Playwright's CDP client can race Chrome during browser teardown and report
// a dropped object handle. All assertions have finished by then; ignore that
// one message so the run reports real failures only.
process.on('unhandledRejection', (err: unknown) => {
  const message = String((err as Error)?.message ?? err)
  if (message.includes('Cannot find object to')) return
  console.error('e2e unhandled rejection:', err)
  process.exitCode = 1
})

export type Shop = {
  url: string
  stop: () => void
}

export type ShopOptions = {
  // gxdev builds the app with the gxdev tag, so the dev gallery exists.
  gxdev?: boolean
  // csp starts the app with a strict Content-Security-Policy (SI-11).
  csp?: boolean
}

// startShop builds the example app and starts it.
export async function startShop(opts: ShopOptions = {}): Promise<Shop> {
  const dir = mkdtempSync(join(tmpdir(), 'gx-shop-'))
  const bin = join(dir, process.platform === 'win32' ? 'shop.exe' : 'shop')
  const args = ['go', 'build', '-o', bin]
  if (opts.gxdev) args.push('-tags', 'gxdev')
  args.push('./cmd/shop')
  const build = spawn(args, { cwd: shopDir })
  const buildCode = await build.exited
  if (buildCode !== 0) {
    const err = await new Response(build.stderr).text()
    throw new Error(`go build failed: ${err}`)
  }
  const port = 18000 + Math.floor(Math.random() * 2000)
  const url = `http://127.0.0.1:${port}`
  const run = [bin, '-addr', `127.0.0.1:${port}`]
  if (opts.csp) run.push('-csp')
  const server = spawn(run, {
    cwd: shopDir,
    stdout: 'ignore',
    stderr: 'ignore',
  })
  await waitFor(url + '/')
  return {
    url,
    stop: () => server.kill(),
  }
}

async function waitFor(url: string): Promise<void> {
  const deadline = Date.now() + 20000
  for (;;) {
    try {
      const res = await fetch(url)
      if (res.ok) return
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`shop did not start at ${url}`)
    await Bun.sleep(100)
  }
}

// One Chrome serves every spec file of a test process. Under bun, closing a
// Playwright browser breaks the next browser the same process launches: its
// pages never report a load, and the run hangs. So no spec file closes the
// browser; preload.ts closes it once, after the last file.
let shared: Promise<Browser> | undefined

// launchBrowser returns the Chrome of this test process.
export function launchBrowser(): Promise<Browser> {
  shared ??= chromium.launch({ channel: 'chrome', headless: true })
  return shared
}

// closeBrowser closes the Chrome of this test process, if one started.
export async function closeBrowser(): Promise<void> {
  if (!shared) return
  const browser = await shared
  shared = undefined
  try {
    await browser.close()
  } catch {
    // already closed
  }
}
