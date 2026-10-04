// The docs kit in a real browser: synced tabs, a remembered choice and
// build-time code frames (REQ-CNT-05).
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { spawn, type Subprocess } from 'bun'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))
const kit = join(repo, 'registry', 'docs')

let dir: string
let app: Subprocess
let browser: Browser
let page: Page
let url: string

// run runs one command in a directory.
async function run(argv: string[], cwd: string): Promise<void> {
  const proc = spawn(argv, { cwd, env: { ...process.env, GOFLAGS: '-mod=mod' }, stdout: 'pipe', stderr: 'pipe' })
  const code = await proc.exited
  if (code !== 0) {
    const err = await new Response(proc.stderr).text()
    throw new Error(`${argv.join(' ')} failed: ${err}`)
  }
}

// waitForUrl polls until the url answers.
async function waitForUrl(target: string, timeout = 30000): Promise<void> {
  const deadline = Date.now() + timeout
  for (;;) {
    try {
      const res = await fetch(target)
      if (res.ok) return
    } catch {
      // not up yet
    }
    if (Date.now() > deadline) throw new Error(`app did not start at ${target}`)
    await Bun.sleep(100)
  }
}

const kitPage = `package pages

import "kitapp/ui/docs"

<gx.Head title="Kit" />
<h1 id="kit-title">Docs kit</h1>
<docs.Aside kind={docs.Tip} title="Tip">A tip.</docs.Aside>
<docs.Tabs sync="db">
  <docs.TabItem label="Postgres">Postgres body</docs.TabItem>
  <docs.TabItem label="SQL Server">SQL Server body</docs.TabItem>
</docs.Tabs>
<docs.Tabs sync="db">
  <docs.TabItem label="Postgres">Postgres body two</docs.TabItem>
  <docs.TabItem label="SQL Server">SQL Server body two</docs.TabItem>
</docs.Tabs>
<docs.Code code={gx.CodeFile("fixture.txt", "2-3")} title="fixture.txt" />
<docs.CardGrid>
  <docs.Card title="One" description="First card." />
  <docs.Card title="Two" description="Second card." />
</docs.CardGrid>
<docs.FileTree items={tree} />
<docs.Badge label="New" />
<docs.LinkButton href={gx.URL("/about")}>About</docs.LinkButton>
`

const treeGo = `package pages

import "kitapp/ui/docs"

var tree = []docs.FileTreeItem{
	{Name: "app", Dir: true, Children: []docs.FileTreeItem{
		{Name: "main.go", Comment: "the app entry"},
	}},
	{Name: "go.mod"},
}
`

const mainGo = `package main

import (
	"net/http"
	"os"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"kitapp/pages"
)

type Kit struct {
	gx.Route ` + "`GET /`" + `
}

var KitPage = gx.Page(func(c *gx.Ctx, in Kit) (pages.KitProps, error) { return pages.KitProps{}, nil }, pages.Kit)

func main() {
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", gx.Collect(KitPage))
	_ = http.ListenAndServe("127.0.0.1:"+os.Getenv("PORT"), app)
}
`

beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), 'gx-kit-'))
  writeFileSync(
    join(dir, 'go.mod'),
    `module kitapp\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => ${repo}\n`,
  )
  mkdirSync(join(dir, 'ui', 'docs'), { recursive: true })
  for (const entry of readdirSync(kit, { withFileTypes: true })) {
    const name = entry.name
    if (!entry.isFile()) continue
    if (name.endsWith('_gx.go') || !(name.endsWith('.gx') || name.endsWith('.go'))) continue
    writeFileSync(join(dir, 'ui', 'docs', name), await Bun.file(join(kit, name)).text())
  }
  writeFileSync(join(dir, 'fixture.txt'), 'first line\nsecond line\nthird line\n')
  mkdirSync(join(dir, 'pages'), { recursive: true })
  writeFileSync(join(dir, 'pages', 'tree.go'), treeGo)
  writeFileSync(join(dir, 'pages', 'Kit.gx'), kitPage)
  writeFileSync(join(dir, 'main.go'), mainGo)
  await run(['go', 'mod', 'tidy'], dir)
  await run(['go', 'run', './cmd/gx', 'generate', dir], repo)
  await run(['go', 'build', '-o', join(dir, 'kitapp'), '.'], dir)
  const port = 19000 + Math.floor(Math.random() * 1500)
  url = `http://127.0.0.1:${port}`
  app = spawn([join(dir, 'kitapp')], {
    cwd: dir,
    env: { ...process.env, PORT: String(port) },
    stdout: 'ignore',
    stderr: 'ignore',
  })
  await waitForUrl(url + '/')
  browser = await chromium.launch({ channel: 'chrome', headless: true })
}), 240000

afterAll(async () => {
  await Bun.sleep(200)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
  app?.kill()
  await Bun.sleep(200)
  if (dir) rmSync(dir, { recursive: true, force: true })
})

test('REQ-CNT-05 the docs kit renders every component', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('#kit-title')
  expect(await page.textContent('#kit-title')).toBe('Docs kit')
  expect(await page.$('.gx-aside')).not.toBeNull()
  expect(await page.$('.gx-card-grid .gx-card')).not.toBeNull()
  expect(await page.$('.gx-file-tree')).not.toBeNull()
  expect(await page.textContent('.gx-link-button')).toBe('About')
  expect(await page.$('.gx-code')).not.toBeNull()
})

test('REQ-CNT-05 the code frame carries the resolved repository lines', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('figure.gx-code')
  const text = (await page.textContent('figure.gx-code')) ?? ''
  expect(text).toContain('fixture.txt')
  expect(text).toContain('second line')
  expect(text).toContain('third line')
  expect(text).not.toContain('first line')
})

test('REQ-CNT-05 tabs sync across the page and remember the choice', async () => {
  page = await browser.newPage()
  await page.goto(url + '/')
  await page.waitForSelector('[data-gx-tabs][data-sync="db"]')
  const groups = await page.$$('[data-gx-tabs][data-sync="db"]')
  expect(groups.length).toBe(2)
  const panels = async (i: number) => groups[i].$$('[data-gx-tab-panel]')
  // The first tab starts open in both groups.
  expect(await (await panels(0))[0].isVisible()).toBe(true)
  expect(await (await panels(0))[1].isVisible()).toBe(false)
  // Clicking the second tab switches both groups.
  const buttons = await groups[0].$$('[data-gx-tab]')
  await buttons[1].click()
  expect(await (await panels(0))[0].isVisible()).toBe(false)
  expect(await (await panels(0))[1].isVisible()).toBe(true)
  expect(await (await panels(1))[0].isVisible()).toBe(false)
  expect(await (await panels(1))[1].isVisible()).toBe(true)
  // The choice survives a reload.
  await page.reload()
  await page.waitForSelector('[data-gx-tabs][data-sync="db"]')
  const after = await page.$$('[data-gx-tabs][data-sync="db"]')
  const afterPanels = await after[0].$$('[data-gx-tab-panel]')
  expect(await afterPanels[0].isVisible()).toBe(false)
  expect(await afterPanels[1].isVisible()).toBe(true)
})
