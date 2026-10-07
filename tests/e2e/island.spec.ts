// The island loader in a real browser (REQ-ISL-04): mount, cleanup, update,
// the abort signal and the signals of the page. The pages here are plain
// files, so each test sees the loader alone, with no server of an app.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser } from './harness'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'

const repo = fileURLToPath(new URL('../..', import.meta.url))

// Each island file records what the loader gives it in window.log.
const modules: Record<string, string> = {
  // An island with a cleanup and no update export.
  '/mods/plain.js': `
    export default (el, props, ctx) => {
      el.innerHTML = '<b>' + props.label + '</b>'
      window.log.push(['mount', el.hasAttribute('data-gx-island-root'), props, typeof ctx.signal, ctx.abort.aborted])
      ctx.abort.addEventListener('abort', () => window.log.push(['abort']))
      return () => window.log.push(['cleanup', props.label])
    }`,
  // An island with an update export. It keeps a count the server cannot see.
  '/mods/updating.js': `
    export default (el, props) => {
      el.innerHTML = '<b></b><i>0</i>'
      el.querySelector('b').textContent = props.label
      el.querySelector('i').textContent = String(Number(el.dataset.clicks ?? 0))
      window.log.push(['mount', props.label])
      return () => window.log.push(['cleanup'])
    }
    export const update = (props, el, ctx) => {
      el.querySelector('b').textContent = props.label
      window.log.push(['update', props.label, el.hasAttribute('data-gx-island-root'), ctx.abort.aborted])
    }`,
  // An island that reads and writes a signal of the page.
  '/mods/signals.js': `
    export default (el, props, ctx) => {
      const qty = ctx.signal(props.qty)
      window.log.push(['get', qty.get()])
      const stop = qty.subscribe((v) => window.log.push(['seen', v]))
      el.innerHTML = '<button>more</button>'
      el.querySelector('button').onclick = () => qty.set(qty.get() + 1)
      window.stopSeeing = stop
      try {
        ctx.signal(props.label)
      } catch (err) {
        window.log.push(['not a ref', String(err.message)])
      }
    }`,
  // An island whose props hold user data with the key of a signal reference.
  '/mods/userdata.js': `
    export default (el, props, ctx) => {
      window.log.push(['props', JSON.parse(JSON.stringify(props.lists)), Object.getPrototypeOf(props.lists) === Object.prototype])
      for (const value of [props.lists, props.old]) {
        try {
          window.log.push(['signal', ctx.signal(value).get()])
        } catch (err) {
          window.log.push(['not a ref', String(err.message)])
        }
      }
    }`,
  // An island that mounts after a wait.
  '/mods/slow.js': `
    export default async (el) => {
      await new Promise((r) => setTimeout(r, 150))
      el.textContent = 'slow'
      return () => window.log.push(['slow cleanup'])
    }`,
  '/mods/nodefault.js': `export const other = 1`,
  '/mods/throws.js': `export default () => { throw new Error('boom') }`,
}

const island = (src: string, props: unknown, attrs = 'load="eager"'): string =>
  `<gx-island id="isl" name="app/x/Island" src="${src}" ${attrs} props='${JSON.stringify(props)}'><div data-gx-island-root data-ignore-morph></div></gx-island>`

const keyed = (key: string, label = key): string =>
  `<gx-island id="island-${key}" data-gx-key="${key}" name="app/x/Island" src="/mods/updating.js" load="eager" props='${JSON.stringify({ label })}'><div data-gx-island-root data-ignore-morph></div></gx-island>`

const pages: Record<string, string> = {
  '/plain': island('/mods/plain.js', { label: 'one', list: [1, 2] }),
  '/updating': island('/mods/updating.js', { label: 'one' }),
  '/signals':
    `<div data-signals='{"cart":{"qty":2}}'><span id="shown" data-text="$cart.qty"></span></div>` +
    island('/mods/signals.js', { qty: { $signal: ['cart', 'qty'], $gx: true }, label: 'x' }) +
    `<script type="module" src="/datastar.js" data-gx-adapter="datastar"></script>`,
  // The JSON of a map[string][]string prop from user data, and of a map
  // with the two keys of a reference: its values have one type.
  '/userdata':
    `<div data-signals='{"cart":{"qty":2}}'></div>` +
    island('/mods/userdata.js', {
      lists: { $signal: ['cart', 'qty'] },
      old: { $signal: ['cart', 'qty'], $gx: ['true'] },
      ref: { $signal: ['cart', 'qty'], $gx: true },
    }) +
    `<script type="module" src="/datastar.js" data-gx-adapter="datastar"></script>`,
  // Two keyed islands in a list, as a loop with key={...} renders them.
  '/list':
    `<div id="list">${keyed('a')}${keyed('b')}</div>` +
    `<script type="module" src="/datastar.js" data-gx-adapter="datastar"></script>`,
  '/slow': island('/mods/slow.js', {}),
  '/nodefault': island('/mods/nodefault.js', {}),
  '/throws': island('/mods/throws.js', {}),
  '/nosrc': `<gx-island id="isl" name="app/x/Island" load="eager" props="{}"></gx-island>`,
  '/bare': `<gx-island id="isl" name="app/x/Island" src="/mods/plain.js" load="eager" props='{"label":"bare"}'></gx-island>`,
}

let server: ReturnType<typeof Bun.serve>
let browser: Browser
let page: Page
let url: string
let errors: string[]

beforeAll(async () => {
  const js = { 'Content-Type': 'text/javascript' }
  server = Bun.serve({
    // The loopback address only: a port that a different program holds on
    // 127.0.0.1 can be free on the wildcard address, and the browser would
    // then reach that program.
    hostname: '127.0.0.1',
    port: 0,
    fetch(req) {
      const path = new URL(req.url).pathname
      if (path === '/island.js') return new Response(readFileSync(join(repo, 'runtime/js/island.js')), { headers: js })
      if (path === '/datastar.js') return new Response(readFileSync(join(repo, 'adapters/datastar/datastar.js')), { headers: js })
      if (modules[path]) return new Response(modules[path], { headers: js })
      if (pages[path]) {
        const html = `<!doctype html><html><head><script>window.log = []</script><script type="module" src="/island.js"></script></head><body>${pages[path]}</body></html>`
        return new Response(html, { headers: { 'Content-Type': 'text/html' } })
      }
      return new Response('not found', { status: 404 })
    },
  })
  url = `http://127.0.0.1:${server.port}`
  browser = await launchBrowser()
  page = await browser.newPage()
  page.on('console', (msg) => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
}, 60000)

afterAll(async () => {
  await page?.close()
  server?.stop(true)
})

const open = async (path: string): Promise<void> => {
  errors = []
  await page.goto(url + path)
}

const mounted = (): Promise<unknown> => page.waitForFunction(() => document.querySelector('#isl')?.matches(':state(mounted)'))
const log = (): Promise<unknown[]> => page.evaluate(() => (window as unknown as { log: unknown[] }).log)

test('REQ-ISL-04 mount gets the root element, the props and a context', async () => {
  await open('/plain')
  await mounted()
  expect(await log()).toEqual([['mount', true, { label: 'one', list: [1, 2] }, 'function', false]])
  expect(await page.innerHTML('#isl [data-gx-island-root]')).toBe('<b>one</b>')
})

test('REQ-ISL-04 an element that leaves the page aborts the context and runs the cleanup', async () => {
  await open('/plain')
  await mounted()
  await page.evaluate(() => document.querySelector('#isl')!.remove())
  await page.waitForFunction(() => (window as unknown as { log: unknown[] }).log.length === 3)
  expect((await log()).slice(1)).toEqual([['abort'], ['cleanup', 'one']])
})

test('REQ-ISL-04 an element that moves in the page stays mounted', async () => {
  await open('/plain')
  await mounted()
  await page.evaluate(() => {
    const el = document.querySelector('#isl')!
    const box = document.createElement('section')
    document.body.append(box)
    box.append(el)
  })
  await page.waitForTimeout(50)
  expect(await log()).toHaveLength(1)
  expect(await page.innerHTML('section #isl [data-gx-island-root]')).toBe('<b>one</b>')
})

test('REQ-ISL-04 new props call the update export and keep the DOM of the island', async () => {
  await open('/updating')
  await mounted()
  await page.evaluate(() => {
    document.querySelector('#isl i')!.textContent = '7'
    document.querySelector('#isl')!.setAttribute('props', JSON.stringify({ label: 'two' }))
  })
  expect(await log()).toEqual([
    ['mount', 'one'],
    ['update', 'two', true, false],
  ])
  expect(await page.innerHTML('#isl [data-gx-island-root]')).toBe('<b>two</b><i>7</i>')
})

test('REQ-ISL-04 new props mount an island with no update export again', async () => {
  await open('/plain')
  await mounted()
  await page.evaluate(() => document.querySelector('#isl')!.setAttribute('props', JSON.stringify({ label: 'two' })))
  await page.waitForFunction(() => (window as unknown as { log: unknown[] }).log.length === 4)
  const got = await log()
  expect(got.slice(1, 3)).toEqual([['abort'], ['cleanup', 'one']])
  expect(got[3]).toEqual(['mount', true, { label: 'two' }, 'function', false])
  expect(await page.innerHTML('#isl [data-gx-island-root]')).toBe('<b>two</b>')
})

test('REQ-ISL-04 the context reads, writes and follows a gx.SignalRef prop', async () => {
  await open('/signals')
  await mounted()
  expect(await log()).toEqual([
    ['get', 2],
    ['seen', 2],
    ['not a ref', 'the value is not a gx.SignalRef prop'],
  ])
  await page.click('#isl button')
  await page.waitForFunction(() => document.querySelector('#shown')?.textContent === '3')
  expect((await log()).at(-1)).toEqual(['seen', 3])
  // The subscription ends with the function it returned.
  await page.evaluate(() => (window as unknown as { stopSeeing: () => void }).stopSeeing())
  await page.click('#isl button')
  await page.waitForFunction(() => document.querySelector('#shown')?.textContent === '4')
  expect((await log()).at(-1)).toEqual(['seen', 3])
})

test('REQ-ISL-04 a props object with the key $signal from user data stays an object', async () => {
  await open('/userdata')
  await mounted()
  expect(await log()).toEqual([
    ['props', { $signal: ['cart', 'qty'] }, true],
    ['not a ref', 'the value is not a gx.SignalRef prop'],
    ['not a ref', 'the value is not a gx.SignalRef prop'],
  ])
})

test('REQ-ISL-04 an island that leaves during an async mount is cleaned up', async () => {
  await open('/slow')
  await page.waitForTimeout(30)
  await page.evaluate(() => document.querySelector('#isl')!.remove())
  await page.waitForFunction(() => (window as unknown as { log: unknown[] }).log.length === 1)
  expect(await log()).toEqual([['slow cleanup']])
})

test('REQ-ISL-04 a host with no root element gets one', async () => {
  await open('/bare')
  await mounted()
  expect(await page.innerHTML('#isl')).toBe('<div data-gx-island-root="" data-ignore-morph=""><b>bare</b></div>')
})

test('REQ-ISL-04 a load error is reported with the name of the island', async () => {
  for (const [path, want] of [
    ['/nodefault', 'the file has no default export'],
    ['/throws', 'boom'],
    ['/nosrc', 'the island is not in the bundle of the app'],
  ]) {
    await open(path)
    const deadline = Date.now() + 5000
    while (!errors.join('\n').includes('gx: island app/x/Island:') && Date.now() < deadline) await Bun.sleep(20)
    expect(errors.join('\n')).toContain(want)
    expect(await page.evaluate(() => document.querySelector('#isl')!.matches(':state(mounted)'))).toBe(false)
  }
})

test('REQ-ISL-06 a morph that reorders keyed islands moves each island with its state', async () => {
  await open('/list')
  await page.waitForFunction(() => document.querySelectorAll('gx-island:state(mounted)').length === 2)
  await page.evaluate(() => {
    document.querySelector('#island-a i')!.textContent = 'state of a'
    document.querySelector('#island-b i')!.textContent = 'state of b'
  })
  // The server answers with the list in the other order and a new label
  // for b. The pinned Datastar applies the patch, as it does for an action.
  const patch = `<div id="list">${keyed('b', 'b2')}${keyed('a')}</div>`
  await page.evaluate((elements) => {
    document.dispatchEvent(
      new CustomEvent('datastar-fetch', {
        detail: { type: 'datastar-patch-elements', el: document.documentElement, argsRaw: { elements } },
      }),
    )
  }, patch)
  await page.waitForFunction(() => document.querySelector('#list')!.firstElementChild!.id === 'island-b')
  await page.waitForTimeout(50)
  expect(await page.innerHTML('#island-b [data-gx-island-root]')).toBe('<b>b2</b><i>state of b</i>')
  expect(await page.innerHTML('#island-a [data-gx-island-root]')).toBe('<b>a</b><i>state of a</i>')
  // Two mounts and one update: no island mounted a second time. The two
  // islands load at the same time, so the order of their mounts is open.
  const got = await log()
  expect(got.slice(0, 2).sort()).toEqual([
    ['mount', 'a'],
    ['mount', 'b'],
  ])
  expect(got.slice(2)).toEqual([['update', 'b2', true, false]])
})
