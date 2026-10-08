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
  // The path of the bundle of the behaviour modules on the server.
  behaviors?: string
  // The path of the island loader on the server.
  islands?: string
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
    beforeNodeAdded: (node: Node) => boolean
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

// absolute makes the path of a link or of a file an absolute URL of the Gx
// server (REQ-ISL-20). A path of the widget HTML starts at the origin of the
// server; on a host page the same text starts at the origin of the host.
const absolute = (el: Element, server: string): void => {
  for (const name of ['href', 'src']) {
    const value = el.getAttribute(name)
    if (value && value.startsWith('/') && !value.startsWith('//')) el.setAttribute(name, server + value)
  }
  const set = el.getAttribute('srcset')
  if (set) {
    const next = set
      .split(',')
      .map((part) => {
        const text = part.trim()
        return text.startsWith('/') && !text.startsWith('//') ? server + text : text
      })
      .join(', ')
    if (next !== set) el.setAttribute('srcset', next)
  }
}

const absoluteAll = (root: ParentNode, server: string): void => {
  for (const el of root.querySelectorAll('[href],[src],[srcset]')) absolute(el, server)
}

// morphCallbacks are the rules of a morph in a widget. The new node gets
// absolute URLs before the morph compares it, so a link that did not change
// is not written two times.
const morphCallbacks = (server: string): MorphOptions['callbacks'] => ({
  // A new node has its absolute URLs before it is in the page, so an island
  // loads its file from the server of its widget.
  beforeNodeAdded: (node) => {
    if (node instanceof Element) {
      absolute(node, server)
      absoluteAll(node, server)
    }
    return true
  },
  beforeNodeMorphed: (from, to) => {
    // The root of an island holds what the island drew. A morph leaves it
    // and its children alone, as on a page (REQ-ISL-06).
    if (from instanceof Element && from.hasAttribute('data-ignore-morph')) return false
    if (to instanceof Element) absolute(to, server)
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
})

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

// A node of the store has no prototype. A part of a path is a value of the
// app: the key of a component instance can be "__proto__" or "constructor".
// On a plain object such a part reaches Object.prototype of the host page;
// on a node with no prototype it is one more key (REQ-ISL-21).
const node = (): Record<string, unknown> => Object.create(null) as Record<string, unknown>

const makeStore = (changed: () => void): Store => {
  const root = node()
  const get = (path: string[]): unknown => {
    let at: unknown = root
    for (const part of path) {
      if (!isTree(at) || !Object.hasOwn(at, part)) return undefined
      at = at[part]
    }
    return at
  }
  const set = (path: string[], value: unknown): void => {
    let at = root
    for (const part of path.slice(0, -1)) {
      if (!Object.hasOwn(at, part) || !isTree(at[part])) at[part] = node()
      at = at[part] as Record<string, unknown>
    }
    const name = path[path.length - 1]
    if (name === undefined || (Object.hasOwn(at, name) && at[name] === value)) return
    at[name] = value
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

// behaviorMarkers are the attributes of the registry components that need a
// behaviour module (REQ-REG-07).
const behaviorMarkers =
  '[data-gx-behavior],[data-gx-roving],[data-gx-trap],[data-gx-dismiss],[data-gx-open],[data-gx-close],[data-gx-tabs],[data-gx-toaster],[data-gx-place],[data-gx-sub],[data-gx-contextmenu]'

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
  const callbacks = morphCallbacks(host.server)
  // life ends the listeners of this mount.
  const life = new AbortController()

  // behaviors loads the behaviour modules of the server for this shadow
  // root, one time: the dialogs, menus, tabs and toasts of the registry.
  let behaviorsLoad: Promise<void> | undefined
  const behaviors = (): Promise<void> =>
    (behaviorsLoad ??= answer.behaviors
      ? (import(host.server + answer.behaviors) as Promise<{ install: (root: ShadowRoot) => void }>).then((m) => m.install(root))
      : Promise.resolve())

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

  // send sends one request of the widget and applies the answer (D-264).
  const send = async (path: string, init: RequestInit): Promise<void> => {
    let res: Response
    try {
      res = await host.request(path, init)
    } catch {
      if (alive) host.fail({ status: 0, key: 'gx.network' })
      return
    }
    if (!alive) return
    // Each answer names the build of the server in a header, also an
    // answer with no body (REQ-ISL-19).
    const seen = res.headers.get('Gx-Build')
    if (seen && build && seen !== build) {
      host.reload()
      return
    }
    if (res.status === 204) return
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

  // call invokes an action of the server with the signals of the widget.
  const call = (method: string, url: string, scope: string): Promise<void> => {
    const headers: Record<string, string> = { 'Gx-Scope': scope, Accept: 'application/json' }
    const init: RequestInit = { method, headers }
    let path = url
    if (method === 'GET' || method === 'DELETE') {
      path += (url.includes('?') ? '&' : '?') + 'gx-signals=' + encodeURIComponent(JSON.stringify(store.root))
    } else {
      headers['Content-Type'] = 'application/json'
      init.body = JSON.stringify({ signals: store.root })
    }
    return send(path, init)
  }

  // submit sends a form of the widget to its action (REQ-ISL-20). The
  // answer morphs the form with its errors, or is a step for the host.
  const submit = async (form: HTMLFormElement, submitter: HTMLElement | null): Promise<void> => {
    const data = new FormData(form)
    const path = submitter?.getAttribute('formaction') ?? form.getAttribute('action') ?? ''
    const init: RequestInit = { method: (form.getAttribute('method') ?? 'post').toUpperCase(), headers: { Accept: 'application/json' } }
    if (form.enctype === 'multipart/form-data') {
      // The browser writes the content type with its boundary.
      init.body = data
    } else {
      const params = new URLSearchParams()
      for (const [name, value] of data) if (typeof value === 'string') params.append(name, value)
      init.body = params
    }
    await send(path, init)
    // A form that came back with errors has a summary: it takes the focus.
    root.querySelector<HTMLElement>('[data-gx-error-summary]')?.focus()
  }

  // validate checks one field on the server and patches its error.
  const validate = (el: HTMLInputElement): Promise<void> | undefined => {
    const path = el.getAttribute('data-gx-validate-url')
    if (!path) return undefined
    const value = el.type === 'checkbox' ? (el.checked ? el.value || 'on' : '') : el.value
    return send(path, { method: 'POST', headers: { Accept: 'application/json' }, body: new URLSearchParams([[el.name, value]]) })
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
      case 'toast': {
        // A toast of the server shows inside the widget, in the toaster
        // region of the view. A view with no region gets a plain one.
        let region = root.querySelector('[data-gx-toaster]')
        if (!region) {
          region = document.createElement('div')
          region.id = 'gx-toaster'
          region.setAttribute('role', 'region')
          region.setAttribute('aria-label', 'Notifications')
          region.setAttribute('aria-live', 'polite')
          region.setAttribute('data-gx-toaster', '')
          root.append(region)
        }
        region.insertAdjacentHTML('beforeend', op.html ?? '')
        return
      }
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
            Idiomorph.morph(target, html, { morphStyle: 'innerHTML', callbacks })
            return
          default:
            Idiomorph.morph(target, html, { morphStyle: 'outerHTML', callbacks })
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
    // A radio button keeps its own value. The signal holds the value of
    // the button of the group that is checked.
    const radio = field instanceof HTMLInputElement && field.type === 'radio'
    const fromField = (): void => {
      if (checkbox) {
        store.set(path, field.checked)
        return
      }
      if (radio) {
        if (field.checked) store.set(path, typeof store.get(path) === 'number' ? Number(field.value) : field.value)
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
      else if (radio) field.checked = field.value === `${v ?? ''}`
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
    absoluteAll(root, host.server)
    // The toaster region of a widget with no region of its own is beside
    // the box, so the scan reads the whole shadow root.
    const elements = [...root.querySelectorAll('*')]
    if (root.querySelector(behaviorMarkers)) void behaviors()
    // An island is a custom element of the page. The island loader of the
    // server defines it, one time for the page.
    if (answer.islands && root.querySelector('gx-island,[data-gx-module]')) void import(host.server + answer.islands)
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

  // A submit does not leave a shadow root, so the widget takes it here.
  root.addEventListener(
    'submit',
    (e) => {
      const form = (e.target as Element | null)?.closest?.('form[data-gx-form]') as HTMLFormElement | null
      if (!form) return
      e.preventDefault()
      void submit(form, (e as SubmitEvent).submitter)
    },
    { signal: life.signal },
  )
  // Live validation of a field (REQ-FRM-06): at blur, or after the user
  // stops typing.
  const typing = new WeakMap<Element, ReturnType<typeof setTimeout>>()
  root.addEventListener(
    'blur',
    (e) => {
      const el = e.target as HTMLInputElement | null
      if (el?.getAttribute?.('data-gx-validate') === 'blur') void validate(el)
    },
    { capture: true, signal: life.signal },
  )
  root.addEventListener(
    'input',
    (e) => {
      const el = e.target as HTMLInputElement | null
      if (el?.getAttribute?.('data-gx-validate') !== 'input') return
      clearTimeout(typing.get(el))
      typing.set(el, setTimeout(() => void validate(el), 300))
    },
    { capture: true, signal: life.signal },
  )

  // An island of the widget reads and writes the signals of the widget
  // through this store, as an island of a page uses the adapter.
  ;(root as unknown as { gxSignals: unknown }).gxSignals = {
    getPath: (path: string): unknown => store.get(path.split('.')),
    mergePatch: (patch: Record<string, unknown>): void => store.merge(patch, false),
    effect: (fn: () => void): (() => void) => {
      effects.add(fn)
      fn()
      return () => effects.delete(fn)
    },
  }

  box.innerHTML = answer.html ?? ''
  // The URLs are absolute before the HTML is in the page.
  absoluteAll(box, host.server)
  root.replaceChildren(box)
  scan()
  return {
    // update morphs the widget to a new render. Nodes that do not change
    // stay, with the value that the user typed and with the focus. Each
    // signal keeps its value.
    update: (next: Answer): void => {
      Idiomorph.morph(box, next.html ?? '', { morphStyle: 'innerHTML', callbacks })
      scan()
    },
    destroy: (): void => {
      alive = false
      life.abort()
      // The toaster region that this mount made is beside the box.
      for (const el of [...root.children]) if (el !== box) el.remove()
      for (const byName of bindings.values()) for (const binding of byName.values()) binding.dispose()
      bindings.clear()
      effects.clear()
      box.remove()
      root.adoptedStyleSheets = []
    },
  }
}
