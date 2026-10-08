// The tools of a page for an agent in the browser (REQ-AI-06). The browser
// of the test has no WebMCP, so each test gives the page a fake
// modelContext and plays the agent.
import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop()
  browser = await launchBrowser()
}, 180000)

afterAll(async () => {
  await Bun.sleep(300)
  shop?.stop()
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

// The API of the July 2026 draft: document.modelContext, and a signal that
// takes a tool out.
const draftAPI = (): void => {
  const fake = { tools: new Map<string, any>(), log: [] as string[] }
  ;(window as any).fake = fake
  const context = {
    registerTool(tool: any, options?: { signal?: AbortSignal }) {
      fake.tools.set(tool.name, tool)
      fake.log.push('register ' + tool.name)
      options?.signal?.addEventListener('abort', () => {
        fake.tools.delete(tool.name)
        fake.log.push('unregister ' + tool.name)
      })
      return Promise.resolve()
    },
  }
  Object.defineProperty(Document.prototype, 'modelContext', { get: () => context, configurable: true })
}

// The API before that draft: navigator.modelContext, and unregisterTool.
const oldAPI = (): void => {
  const fake = { tools: new Map<string, any>(), log: [] as string[] }
  ;(window as any).fake = fake
  const context = {
    registerTool(tool: any) {
      fake.tools.set(tool.name, tool)
      fake.log.push('register ' + tool.name)
    },
    unregisterTool(name: string) {
      fake.tools.delete(name)
      fake.log.push('unregister ' + name)
    },
  }
  Object.defineProperty(Navigator.prototype, 'modelContext', { get: () => context, configurable: true })
}

const open = async (init?: () => void): Promise<void> => {
  page = await browser.newPage()
  page.on('pageerror', (err) => {
    throw err
  })
  if (init) await page.addInitScript(init)
  await page.goto(shop.url + '/')
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
}

const toolNames = (): Promise<string[]> => page.evaluate(() => [...(window as any).fake.tools.keys()].sort())

const waitTools = (count: number): Promise<unknown> =>
  page.waitForFunction((n) => (window as any).fake.tools.size === n, count)

// run plays the agent: it calls execute of one registered tool.
const run = (name: string, input: unknown): Promise<{ value?: unknown; error?: string }> =>
  page.evaluate(
    async ([n, i]) => {
      try {
        return { value: await (window as any).fake.tools.get(n).execute(i, { signal: new AbortController().signal }) }
      } catch (err) {
        return { error: String((err as Error).message) }
      }
    },
    [name, input] as const,
  )

test('REQ-AI-06 a tool is registered while an element that invokes it is in the page', async () => {
  await open(draftAPI)
  await waitTools(2)
  expect(await toolNames()).toEqual(['cart_add', 'cart_toast'])
  const add = await page.evaluate(() => {
    const t = (window as any).fake.tools.get('cart_add')
    return { description: t.description, schema: t.inputSchema, annotations: t.annotations, execute: typeof t.execute }
  })
  // The description is the doc comment of the action, and the schema comes
  // from its input.
  expect(add.description).toContain('Sets the total of the cart to ten times the quantity.')
  expect(add.schema).toEqual({ type: 'object', additionalProperties: false, properties: { qty: { type: 'integer' } } })
  expect(add.annotations).toEqual({ readOnlyHint: false, consequentialHint: false })
  expect(add.execute).toBe('function')
  // Two carts invoke the tool. It is one tool.
  expect((await page.evaluate(() => (window as any).fake.log)).filter((l: string) => l === 'register cart_add').length).toBe(1)

  // The elements leave the page: the tools leave the browser.
  await page.evaluate(() => {
    ;(window as any).kept = [...document.querySelectorAll('[data-gx-instance^="cart.Cart"]')].map((el) => ({ el, parent: el.parentNode, next: el.nextSibling }))
    for (const k of (window as any).kept) k.el.remove()
  })
  await waitTools(0)
  expect((await page.evaluate(() => (window as any).fake.log)).slice(-2).sort()).toEqual(['unregister cart_add', 'unregister cart_toast'])
  // They come back: the tools come back.
  await page.evaluate(() => {
    for (const k of (window as any).kept) k.parent.insertBefore(k.el, k.next)
  })
  await waitTools(2)
})

test('REQ-AI-06 a call of a tool runs the action, and the answer changes the page', async () => {
  await open(draftAPI)
  await waitTools(2)
  const result = await run('cart_add', { qty: 4 })
  expect(result.error).toBeUndefined()
  // The action has no typed result: the agent gets the summary of its
  // patches.
  expect(String(result.value)).toContain('morphed #cart-total-alpha')
  // The patch of the answer is in the page, in the cart of the first
  // element that invokes the tool. The other cart did not change.
  await page.waitForFunction(() => document.querySelector('#cart-total-alpha')?.textContent === '40')
  expect(await page.textContent('#cart-total-beta')).toBe('2')

  // An argument of the wrong type is an error for the agent.
  const bad = await run('cart_add', { qty: 'many' })
  expect(bad.error).toBeDefined()
  expect(await page.textContent('#cart-total-alpha')).toBe('40')
})

test('REQ-AI-06 gx.Confirm makes the browser ask the user before a tool runs', async () => {
  await open(draftAPI)
  await waitTools(2)
  const annotations = await page.evaluate(() => (window as any).fake.tools.get('cart_toast').annotations)
  expect(annotations).toEqual({ readOnlyHint: false, consequentialHint: true })
  const calls: string[] = []
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().includes('/_gx/tools/')) calls.push(r.url())
  })
  // The user says no: no request goes to the server.
  page.once('dialog', (d) => void d.dismiss())
  const refused = await run('cart_toast', {})
  expect(refused.error).toContain('did not allow')
  expect(calls).toEqual([])
  // The user says yes: the action runs, the agent gets its typed result,
  // and the toast of the answer shows.
  let asked = ''
  page.once('dialog', (d) => {
    asked = d.message()
    void d.accept()
  })
  const done = await run('cart_toast', {})
  expect(done.error).toBeUndefined()
  expect(done.value).toEqual({ saved: true })
  expect(asked).toContain('Saves the cart')
  expect(calls.length).toBe(1)
  await page.waitForSelector('[data-gx-toaster] :text("Saved")')
})

test('REQ-AI-06 the runtime falls back to navigator.modelContext', async () => {
  await open(oldAPI)
  await waitTools(2)
  expect(await toolNames()).toEqual(['cart_add', 'cart_toast'])
  const result = await run('cart_add', { qty: 2 })
  expect(result.error).toBeUndefined()
  await page.waitForFunction(() => document.querySelector('#cart-total-alpha')?.textContent === '20')
  // This API has no signal: the runtime takes a tool out by its name.
  await page.evaluate(() => {
    for (const el of document.querySelectorAll('[data-gx-instance^="cart.Cart"]')) el.remove()
  })
  await waitTools(0)
})

test('REQ-AI-06 with no modelContext the page works and asks for no tool', async () => {
  const requests: string[] = []
  await open()
  page.on('request', (r) => requests.push(r.url()))
  await page.reload()
  await page.waitForSelector('[data-gx-instance="cart.Cart.alpha"]')
  await page.click('[data-label="Alpha"] button:text-is("Add")')
  await page.waitForFunction(() => document.querySelector('#cart-total-alpha')?.textContent === '10')
  expect(requests.filter((u) => u.includes('/_gx/tools/'))).toEqual([])
})
