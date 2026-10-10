// Shared signals in real browsers (M25): two viewers of one room see the
// same value, a viewer of a different room does not, and the server signs
// the room key.
import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test'
import { type Browser, type BrowserContext, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let contexts: BrowserContext[] = []

beforeAll(async () => {
  shop = await startShop()
  browser = await launchBrowser()
}), 180000

afterAll(async () => {
  await Bun.sleep(400)
  shop?.stop()
})

afterEach(async () => {
  for (const context of contexts.splice(0)) {
    try {
      await context.close()
    } catch {
      // already closed
    }
  }
})

// viewer opens the basket page of a room in a browser of its own.
async function viewer(room: string): Promise<Page> {
  const context = await browser.newContext()
  contexts.push(context)
  const page = await context.newPage()
  const stream = page.waitForResponse((res) => res.url().includes('/_gx/room/basket.Note'))
  await page.goto(`${shop.url}/basket?room=${room}`)
  await page.waitForSelector('#note-input')
  // The viewer is in the room when its stream is open.
  expect((await stream).status()).toBe(200)
  return page
}

async function waitText(page: Page, selector: string, want: string, timeout = 5000): Promise<void> {
  const deadline = Date.now() + timeout
  let last = ''
  for (;;) {
    last = ((await page.textContent(selector)) ?? '').trim()
    if (last === want) return
    if (Date.now() > deadline) throw new Error(`timeout: ${selector} = ${JSON.stringify(last)}, want ${JSON.stringify(want)}`)
    await Bun.sleep(50)
  }
}

test('REQ-ACT-21 two viewers of one room see a change, and a viewer of a different room does not', async () => {
  const room = 'r' + Date.now()
  const a = await viewer(room)
  const b = await viewer(room)
  const c = await viewer(room + '-other')

  await a.fill('#note-input', 'milk and tea')
  await waitText(b, '#note-text', 'milk and tea')
  expect(await b.inputValue('#note-input')).toBe('milk and tea')
  // The second viewer changes a different shared signal.
  await b.click('#note-busy')
  await a.waitForSelector('#note-busy-mark', { state: 'visible' })
  await b.waitForSelector('#note-busy-mark', { state: 'visible' })

  await Bun.sleep(300)
  expect(((await c.textContent('#note-text')) ?? '').trim()).toBe('')
  expect(await c.isVisible('#note-busy-mark')).toBe(false)

  // A viewer that comes later gets the last values of the room.
  const late = await viewer(room)
  await waitText(late, '#note-text', 'milk and tea')
  await late.waitForSelector('#note-busy-mark', { state: 'visible' })
})

test('SI-17 a value that fails a rule reaches no viewer, and the writer gets the value of the room back', async () => {
  const room = 'rule' + Date.now()
  const a = await viewer(room)
  const b = await viewer(room)
  await a.fill('#note-input', 'short')
  await waitText(b, '#note-text', 'short')

  const refused = a.waitForResponse((res) => res.url().endsWith('/_gx/room/basket.Note') && res.request().method() === 'POST')
  await a.fill('#note-input', 'a note that is longer than twenty characters')
  expect((await refused).status()).toBe(422)
  await waitText(a, '#note-text', 'short')
  await Bun.sleep(200)
  expect(((await b.textContent('#note-text')) ?? '').trim()).toBe('short')
})

test('SI-17 a room key that the server did not sign gets 403', async () => {
  const room = 'sig' + Date.now()
  const a = await viewer(room)
  const key = (await a.getAttribute('[data-gx-room]', 'data-gx-room')) ?? ''
  expect(key).toContain('.')
  // The key of a different room with the signature of this one.
  const forged = btoa(room + '-other').replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_') + '.' + key.split('.')[1]
  const status = await a.evaluate(async (room) => {
    const write = await fetch('/_gx/room/basket.Note', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ room, signals: { note: 'x' } }),
    })
    const read = await fetch('/_gx/room/basket.Note?room=' + encodeURIComponent(room), { headers: { Accept: 'text/event-stream' } })
    return [write.status, read.status]
  }, forged)
  expect(status).toEqual([403, 403])
})
