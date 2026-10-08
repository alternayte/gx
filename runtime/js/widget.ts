// The widget script of a Gx server (REQ-ISL-19). The element file of a host
// page imports it from the server. It is the same file for each app of one
// Gx version: it holds no code of the app. The server puts Idiomorph before
// this code in the same file.
//
// The script owns one widget instance at a time, in its shadow root: the
// stylesheet, the HTML, the signals, the client expressions and the answers
// of actions. Nothing here reads the document of the host for the state of
// a widget, so two widgets and a Datastar of the host do not share signals
// (REQ-ISL-21).
import { evaluate, type Tree } from './widget-eval'

type WidgetError = { status: number; key: string; field?: string }

type Answer = {
  tag?: string
  html?: string
  // The path of the stylesheet of the widget on the server.
  style?: string
  // The build of the server that made the answer.
  build?: string
}

// One step of the answer of an action.
type Op = {
  op: 'patch' | 'signals' | 'redirect' | 'toast' | 'event'
  mode?: 'morph' | 'inner' | 'append' | 'prepend' | 'replace' | 'remove'
  target?: string
  html?: string
  scope?: string
  values?: Record<string, unknown>
  url?: string
  // The name and the detail of a domain event.
  name?: string
  detail?: unknown
}

type ActionAnswer = { build?: string; ops?: Op[]; error?: WidgetError }

// What the element gives the script.
type Host = {
  // The origin of the Gx server.
  server: string
  // request sends a request of this widget to the server.
  request: (path: string, init: RequestInit) => Promise<Response>
  // fail reports an error to the host page.
  fail: (error: WidgetError) => void
  // event dispatches an event on the element. It returns false when a
  // listener of the host stopped it.
  event: (name: string, detail: unknown) => boolean
  // reload does a fresh first render, after a new build of the server.
  reload: () => void
}

type Mounted = {
  update: (answer: Answer) => void
  destroy: () => void
}

type MorphOptions = {
  morphStyle: 'innerHTML' | 'outerHTML'
  callbacks: {
    beforeNodeMorphed: (from: Node, to: Node) => boolean
    beforeAttributeUpdated: (name: string, node: Node, kind: string) => boolean
  }
}

declare const Idiomorph: {
  morph: (target: Node, content: string, options: MorphOptions) => void
}

// userState names, for each form field of one morph, the state that the
// server did not change in the new render. A morph keeps that state as the
// user left it: the text of a field, a check mark, the choice of a select.
// When the new render has a different value, the value of the server wins.
const userState = new WeakMap<Node, Set<string>>()

const morphCallbacks: MorphOptions['callbacks'] = {
  beforeNodeMorphed: (from, to) => {
    const same = new Set<string>()
    if (from instanceof HTMLInputElement && to instanceof HTMLInputElement) {
      if (from.getAttribute('value') === to.getAttribute('value')) same.add('value')
      if (from.hasAttribute('checked') === to.hasAttribute('checked')) same.add('checked')
    } else if (from instanceof HTMLTextAreaElement && to instanceof HTMLTextAreaElement) {
      if (from.defaultValue === to.defaultValue) same.add('value')
    } else if (from instanceof HTMLOptionElement && to instanceof HTMLOptionElement) {
      if (from.hasAttribute('selected') === to.hasAttribute('selected')) same.add('selected')
    }
    if (same.size > 0) userState.set(from, same)
    else userState.delete(from)
    return true
  },
  beforeAttributeUpdated: (name, node) => !userState.get(node)?.has(name),
}

// sheets holds one stylesheet object for each stylesheet URL, so each
// instance of a widget on the page shares it.
const sheets = new Map<string, Promise<CSSStyleSheet>>()

// loadSheet fetches the stylesheet of a widget and makes a constructable
// stylesheet of it (REQ-ISL-11). The rules live in the shadow root only: they
// do not reach the host page, and the rules of the host do not reach in.
const loadSheet = (url: string): Promise<CSSStyleSheet> => {
  let sheet = sheets.get(url)
  if (!sheet) {
    sheet = fetch(url, { credentials: 'omit' }).then(async (res) => {
      if (!res.ok) throw new Error('gx: the stylesheet of the widget did not load: ' + res.status)
      const css = new CSSStyleSheet()
      css.replaceSync(await res.text())
      return css
    })
    sheets.set(url, sheet)
    // A load that failed is tried again by the next widget.
    sheet.catch(() => sheets.delete(url))
  }
  return sheet
}

// A signal store of one widget instance: a tree of values by path.
type Store = {
  root: Record<string, unknown>
  get: (path: string[]) => unknown
  set: (path: string[], value: unknown) => void
  // merge puts a tree of values into the store. With keep, a signal that
  // has a value keeps it.
  merge: (values: Record<string, unknown>, keep: boolean, at?: string[]) => void
}

const isTree = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v)

const makeStore = (changed: () => void): Store => {
  const root: Record<string, unknown> = {}
  const get = (path: string[]): unknown => {
    let node: unknown = root
    for (const part of path) {
      if (!isTree(node)) return undefined
      node = node[part]
    }
    return node
  }
  const set = (path: string[], value: unknown): void => {
    let node = root
    for (const part of path.slice(0, -1)) {
      if (!isTree(node[part])) node[part] = {}
      node = node[part] as Record<string, unknown>
    }
    const name = path[path.length - 1]
    if (name === undefined || node[name] === value) return
    node[name] = value
    changed()
  }
  const merge = (values: Record<string, unknown>, keep: boolean, at: string[] = []): void => {
    for (const [name, value] of Object.entries(values)) {
      const path = [...at, name]
      if (isTree(value)) merge(value, keep, path)
      else if (!keep || get(path) === undefined) set(path, value)
    }
  }
  return { root, get, set, merge }
}

// duration reads "300ms" or "1s" as milliseconds.
const duration = (text: string | undefined, fallback: number): number => {
  if (!text) return fallback
  const n = parseFloat(text)
  if (Number.isNaN(n)) return fallback
  return text.endsWith('ms') ? n : text.endsWith('s') ? n * 1000 : n
}

type Mods = Map<string, string>

// timed applies the delay, debounce and throttle modifiers of a handler, as
// the adapter of a page does: a debounce runs after the last call, and a
// throttle runs the first call of each period.
const timed = (run: (e?: Event) => void, mods: Mods): ((e?: Event) => void) => {
  let fn = run
  if (mods.has('delay')) {
    const inner = fn
    const wait = duration(mods.get('delay'), 0)
    fn = (e) => void setTimeout(() => inner(e), wait)
  }
  if (mods.has('debounce')) {
    const inner = fn
    const wait = duration(mods.get('debounce'), 0)
    let timer: ReturnType<typeof setTimeout> | undefined
    fn = (e) => {
      clearTimeout(timer)
      timer = setTimeout(() => inner(e), wait)
    }
  }
  if (mods.has('throttle')) {
    const inner = fn
    const wait = duration(mods.get('throttle'), 0)
    let open = true
    fn = (e) => {
      if (!open) return
      open = false
      setTimeout(() => (open = true), wait)
      inner(e)
    }
  }
  return fn
}

// A binding is one client attribute of one element, in use.
type Binding = { value: string; dispose: () => void }

// mount puts the first render of a widget into its shadow root. The HTML
// comes from the Gx server of the widget, which escapes each value.
export const mount = async (root: ShadowRoot, answer: Answer, host: Host): Promise<Mounted> => {
  // The stylesheet is in place before the HTML, so the widget never shows
  // with no styles.
  root.adoptedStyleSheets = answer.style ? [await loadSheet(host.server + answer.style)] : []
  // One box holds the widget. It makes no box of its own in the layout.
  const box = document.createElement('div')
  box.setAttribute('data-gx-widget', answer.tag ?? '')
  box.style.display = 'contents'
  const build = answer.build

  // effects are the functions that put the value of an expression into the
  // DOM. Each runs again after a signal changes.
  const effects = new Set<() => void>()
  let queued = false
  let alive = true
  const store = makeStore(() => {
    if (queued) return
    queued = true
    queueMicrotask(() => {
      queued = false
      if (alive) for (const effect of [...effects]) effect()
    })
  })
  const read = (path: string[]): unknown => store.get(path)
  const value = (text: string): unknown => evaluate(JSON.parse(text) as Tree, read)

  // run runs the statements of an on: handler.
  const run = (text: string): void => {
    const [, ...statements] = JSON.parse(text) as Tree
    for (const statement of statements as Tree[]) {
      const [op, a, b] = statement
      switch (op) {
        case '=':
          store.set(a as string[], evaluate(b as Tree, read))
          break
        case '++':
          store.set(a as string[], (store.get(a as string[]) as number) + 1)
          break
        case '--':
          store.set(a as string[], (store.get(a as string[]) as number) - 1)
          break
        case 'call':
          void call(a as string, b as string, statement[3] as string)
          break
        default:
          throw new Error('gx: the widget script does not know the statement ' + String(op))
      }
    }
  }

  // call invokes an action of the server and applies its answer (D-264).
  const call = async (method: string, url: string, scope: string): Promise<void> => {
    const headers: Record<string, string> = { 'Gx-Scope': scope, Accept: 'application/json' }
    const init: RequestInit = { method, headers }
    let path = url
    if (method === 'GET' || method === 'DELETE') {
      path += (url.includes('?') ? '&' : '?') + 'gx-signals=' + encodeURIComponent(JSON.stringify(store.root))
    } else {
      headers['Content-Type'] = 'application/json'
      init.body = JSON.stringify({ signals: store.root })
    }
    let res: Response
    try {
      res = await host.request(path, init)
    } catch {
      if (alive) host.fail({ status: 0, key: 'gx.network' })
      return
    }
    if (!alive || res.status === 204) return
    let answer: ActionAnswer = {}
    try {
      answer = (await res.json()) as ActionAnswer
    } catch {
      // An answer that is not JSON is not an answer of Gx to a widget.
    }
    if (!alive) return
    if (answer.build && build && answer.build !== build) {
      // The server has a new build. Its answer is for HTML that this
      // widget does not hold, so the widget loads again (REQ-ISL-19).
      host.reload()
      return
    }
    for (const op of answer.ops ?? []) apply(op)
    scan()
    if (answer.error) host.fail(answer.error)
    else if (!res.ok) host.fail({ status: res.status, key: 'gx.error' })
  }

  // apply does one step of the answer of an action.
  const apply = (op: Op): void => {
    switch (op.op) {
      case 'signals':
        store.merge(op.values ?? {}, false, (op.scope ?? '').split('.').filter((part) => part !== ''))
        return
      case 'event':
        // A domain event of the server: the host page gets it from the
        // element (REQ-ISL-17).
        if (op.name) host.event(op.name, op.detail ?? null)
        return
      case 'redirect':
        // The host owns its navigation: it gets the URL in an event, and
        // the page does not move.
        host.event('gx-navigate', { url: op.url ?? '' })
        return
      case 'patch': {
        const target = op.target ? box.querySelector(op.target) : null
        if (!target) return
        const html = op.html ?? ''
        switch (op.mode) {
          case 'remove':
            target.remove()
            return
          case 'append':
            target.insertAdjacentHTML('beforeend', html)
            return
          case 'prepend':
            target.insertAdjacentHTML('afterbegin', html)
            return
          case 'replace':
            target.outerHTML = html
            return
          case 'inner':
            Idiomorph.morph(target, html, { morphStyle: 'innerHTML', callbacks: morphCallbacks })
            return
          default:
            Idiomorph.morph(target, html, { morphStyle: 'outerHTML', callbacks: morphCallbacks })
        }
      }
    }
  }

  const bindings = new Map<Element, Map<string, Binding>>()
  // started holds the elements whose load handler ran, so a scan does not
  // run it a second time.
  const started = new WeakMap<Element, string>()

  // bind makes the binding of one data-gx- attribute. name has no prefix.
  const bind = (el: Element, name: string, text: string): (() => void) | undefined => {
    const effect = (fn: () => void): (() => void) => {
      effects.add(fn)
      fn()
      return () => effects.delete(fn)
    }
    const [base = '', ...rest] = name.split('__')
    const mods: Mods = new Map(
      rest.map((mod): [string, string] => {
        const dot = mod.indexOf('.')
        return dot < 0 ? [mod, ''] : [mod.slice(0, dot), mod.slice(dot + 1)]
      }),
    )
    const html = el as HTMLElement
    if (base === 'show') {
      return effect(() => {
        if (value(text)) {
          if (html.style.display === 'none') html.style.removeProperty('display')
        } else {
          html.style.setProperty('display', 'none')
        }
      })
    }
    if (base === 'text') return effect(() => void (el.textContent = `${value(text)}`))
    if (base.startsWith('class:')) {
      const classes = base.slice(6).split(/\s+/).filter(Boolean)
      return effect(() => {
        const on = Boolean(value(text))
        for (const name of classes) el.classList.toggle(name, on)
      })
    }
    if (base.startsWith('attr:')) {
      const attr = base.slice(5)
      return effect(() => {
        const v = value(text)
        if (v === '' || v === true) el.setAttribute(attr, '')
        else if (v === false || v === null || v === undefined) el.removeAttribute(attr)
        else el.setAttribute(attr, typeof v === 'string' ? v : JSON.stringify(v))
      })
    }
    if (base === 'bind') return bindField(el, text.split('.'), effect)
    if (base === 'init') {
      if (started.get(el) !== text) {
        started.set(el, text)
        timed(() => run(text), mods)()
      }
      return undefined
    }
    if (base === 'on-intersect') {
      const handler = timed(() => run(text), mods)
      const observer = new IntersectionObserver((entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          handler()
          if (mods.has('once')) observer.disconnect()
        }
      })
      observer.observe(el)
      return () => observer.disconnect()
    }
    if (base === 'on-interval') {
      const timer = setInterval(() => run(text), duration(mods.get('duration'), 1000))
      return () => clearInterval(timer)
    }
    if (base.startsWith('on:')) {
      const event = base.slice(3)
      const inner = timed(() => run(text), mods)
      const outside = mods.has('outside')
      const handler = (e: Event): void => {
        // A click of a different part of the page: the path of the event
        // does not hold the element.
        if (outside && e.composedPath().includes(el)) return
        if (mods.has('prevent')) e.preventDefault()
        if (mods.has('stop')) e.stopPropagation()
        inner(e)
      }
      const target: EventTarget = mods.has('window') ? window : outside ? document : el
      const options = { capture: mods.has('capture'), passive: mods.has('passive'), once: mods.has('once') }
      target.addEventListener(event, handler, options)
      return () => target.removeEventListener(event, handler, options)
    }
    return undefined
  }

  // bindField keeps a form field and a signal the same (bind:).
  const bindField = (el: Element, path: string[], effect: (fn: () => void) => () => void): (() => void) => {
    const field = el as HTMLInputElement
    const checkbox = field instanceof HTMLInputElement && field.type === 'checkbox'
    const fromField = (): void => {
      if (checkbox) {
        store.set(path, field.checked)
        return
      }
      const numeric = field.type === 'number' || field.type === 'range' || typeof store.get(path) === 'number'
      store.set(path, numeric ? Number(field.value) : field.value)
    }
    field.addEventListener('input', fromField)
    field.addEventListener('change', fromField)
    const stop = effect(() => {
      const v = store.get(path)
      if (checkbox) field.checked = Boolean(v)
      else if (field.value !== `${v ?? ''}`) field.value = `${v ?? ''}`
    })
    return () => {
      stop()
      field.removeEventListener('input', fromField)
      field.removeEventListener('change', fromField)
    }
  }

  // scan reads the client attributes of the widget after its HTML changed.
  // A binding whose element and text did not change stays, with its timers.
  const scan = (): void => {
    const elements = [...box.querySelectorAll('*')]
    // The first values of the signals come before the expressions that
    // read them. A signal that has a value keeps it.
    for (const el of elements) {
      const signals = el.getAttribute('data-gx-signals')
      if (signals) store.merge(JSON.parse(signals) as Record<string, unknown>, true)
    }
    const present = new Set(elements)
    for (const [el, byName] of bindings) {
      if (present.has(el)) continue
      for (const binding of byName.values()) binding.dispose()
      bindings.delete(el)
    }
    for (const el of elements) {
      const byName = bindings.get(el) ?? new Map<string, Binding>()
      const names = new Set<string>()
      for (const attr of [...el.attributes]) {
        if (!attr.name.startsWith('data-gx-')) continue
        const name = attr.name.slice(8)
        names.add(name)
        const old = byName.get(name)
        if (old && old.value === attr.value) continue
        old?.dispose()
        byName.delete(name)
        const dispose = bind(el, name, attr.value)
        if (dispose) byName.set(name, { value: attr.value, dispose })
      }
      for (const [name, binding] of byName) {
        if (names.has(name)) continue
        binding.dispose()
        byName.delete(name)
      }
      if (byName.size > 0) bindings.set(el, byName)
      else bindings.delete(el)
    }
    // A morph puts the markup of the server back into a node that stays.
    // Each expression gives the node its value again.
    for (const effect of [...effects]) effect()
  }

  box.innerHTML = answer.html ?? ''
  root.replaceChildren(box)
  scan()
  return {
    // update morphs the widget to a new render. Nodes that do not change
    // stay, with the value that the user typed and with the focus. Each
    // signal keeps its value.
    update: (next: Answer): void => {
      Idiomorph.morph(box, next.html ?? '', { morphStyle: 'innerHTML', callbacks: morphCallbacks })
      scan()
    },
    destroy: (): void => {
      alive = false
      for (const byName of bindings.values()) for (const binding of byName.values()) binding.dispose()
      bindings.clear()
      effects.clear()
      box.remove()
      root.adoptedStyleSheets = []
    },
  }
}
