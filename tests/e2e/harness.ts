// The gx e2e harness. It builds the example shop and starts it on a free
// port. Bun and Playwright are for the repo's own tests only (G3).
import { spawn } from 'bun'
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

// startShop builds the example app and starts it.
export async function startShop(): Promise<Shop> {
  const dir = mkdtempSync(join(tmpdir(), 'gx-shop-'))
  const bin = join(dir, process.platform === 'win32' ? 'shop.exe' : 'shop')
  const build = spawn(['go', 'build', '-o', bin, './cmd/shop'], { cwd: shopDir })
  const buildCode = await build.exited
  if (buildCode !== 0) {
    const err = await new Response(build.stderr).text()
    throw new Error(`go build failed: ${err}`)
  }
  const port = 18000 + Math.floor(Math.random() * 2000)
  const url = `http://127.0.0.1:${port}`
  const server = spawn([bin, '-addr', `127.0.0.1:${port}`], {
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
