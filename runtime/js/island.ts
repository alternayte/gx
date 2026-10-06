// The island loader (REQ-ISL-04 to REQ-ISL-06). The server renders
//
//   <gx-island name="app/dash/Chart" src="/_gx/islands/…js" props="{…}">
//     <div data-gx-island-root data-ignore-morph></div>
//   </gx-island>
//
// and this module loads the file, calls its default export with the root
// element, the props and a context, and keeps the cleanup. A page loads this
// module only when it holds an island.

// A signal of the page, as an island reads and writes it.
type Signal = {
  get(): unknown
  set(value: unknown): void
  subscribe(fn: (value: unknown) => void): () => void
}

// The context of one mounted island.
type Ctx = {
  signal(ref: unknown): Signal
  abort: AbortSignal
}

type Cleanup = void | (() => void)

// The exports of an island file.
type IslandModule = {
  default?: (el: HTMLElement, props: unknown, ctx: Ctx) => Cleanup | Promise<Cleanup>
  update?: (props: unknown, el: HTMLElement, ctx: Ctx) => void
}

// The exports of the adapter runtime that hold the signals of the page.
type SignalStore = {
  getPath(path: string): unknown
  mergePatch(patch: Record<string, unknown>): void
  effect(fn: () => void): () => void
}

// The server writes a gx.SignalRef prop as {"$signal": ["cart", "Cart", "qty"]}:
// the parts of the signal path.
const refKey = '$signal'

class SignalRef {
  constructor(readonly path: string[]) {}
}

let store: Promise<SignalStore> | undefined

// signalStore loads the adapter runtime of the page. The page already runs
// that module, so the import gives the same instance and its signals.
const signalStore = (): Promise<SignalStore> => {
  if (store) return store
  const script = document.querySelector<HTMLScriptElement>('script[data-gx-adapter]')
  if (!script) return Promise.reject(new Error('the page has no adapter with signals'))
  // The adapter reads the data-signals attributes of the page in a timer
  // that it starts when its module runs. A timer that starts here runs
  // after that one, so the signals hold their first values by then.
  return (store = import(script.src).then(
    (module: SignalStore) => new Promise<SignalStore>((resolve) => setTimeout(() => resolve(module))),
  ))
}

// patchFor returns the nested object that sets one signal path.
const patchFor = (parts: string[], value: unknown): Record<string, unknown> => {
  const patch: Record<string, unknown> = {}
  let at = patch
  parts.forEach((part, i) => {
    if (i === parts.length - 1) {
      at[part] = value
      return
    }
    at = at[part] = {} as Record<string, unknown>
  })
  return patch
}

// parseProps reads the props attribute. It reports whether a prop is a
// signal, because the island then needs the signals before it mounts.
const parseProps = (text: string | null): { props: unknown; signals: boolean } => {
  let signals = false
  const props = JSON.parse(text || '{}', (_key, value) => {
    if (value && typeof value === 'object' && Array.isArray(value[refKey]) && Object.keys(value).length === 1) {
      signals = true
      return new SignalRef(value[refKey])
    }
    return value
  })
  return { props, signals }
}

// whenReady calls start at the time the load attribute names
// (REQ-ISL-05): eager at once, idle when the browser is idle, visible (the
// default) when the element comes near the viewport, media when the query
// in the media attribute matches. It returns a function that cancels the
// wait.
const whenReady = (el: HTMLElement, start: () => void): (() => void) => {
  switch (el.getAttribute('load') ?? 'visible') {
    case 'eager':
      start()
      return () => {}
    case 'idle': {
      if ('requestIdleCallback' in window) {
        const id = requestIdleCallback(start)
        return () => cancelIdleCallback(id)
      }
      const id = setTimeout(start, 1)
      return () => clearTimeout(id)
    }
    case 'media': {
      const query = matchMedia(el.getAttribute('media') ?? 'all')
      if (query.matches) {
        start()
        return () => {}
      }
      const onChange = () => {
        if (!query.matches) return
        query.removeEventListener('change', onChange)
        start()
      }
      query.addEventListener('change', onChange)
      return () => query.removeEventListener('change', onChange)
    }
    default: {
      const observer = new IntersectionObserver(
        (entries) => {
          if (!entries.some((e) => e.isIntersecting)) return
          observer.disconnect()
          start()
        },
        { rootMargin: '200px' },
      )
      observer.observe(el)
      return () => observer.disconnect()
    }
  }
}

// Mounted is the state of one mounted island.
type Mounted = {
  module: IslandModule
  ctx: Ctx
  controller: AbortController
  cleanup: Cleanup
  stops: Array<() => void>
}

class GxIsland extends HTMLElement {
  static observedAttributes = ['props', 'src']

  // The element matches :state(mounted) while its island runs. A morph
  // cannot remove a state, as it removes an attribute the server did not
  // write.
  #internals = this.attachInternals()
  #cancelWait: (() => void) | undefined
  #mounted: Mounted | undefined
  // #run numbers the mounts, so that a load that ends after the element left
  // the page, or after a newer load began, mounts nothing.
  #run = 0

  connectedCallback(): void {
    if (this.#mounted || this.#cancelWait) return
    this.#cancelWait = whenReady(this, () => {
      this.#cancelWait = undefined
      void this.#mount()
    })
  }

  disconnectedCallback(): void {
    // A morph can move the element: it leaves the page and comes back in
    // the same task. Only an element that stays out unmounts.
    queueMicrotask(() => {
      if (this.isConnected) return
      this.#cancelWait?.()
      this.#cancelWait = undefined
      this.#unmount()
    })
  }

  attributeChangedCallback(name: string, before: string | null, after: string | null): void {
    if (before === after || !this.#mounted) return
    const { module, ctx } = this.#mounted
    if (name === 'props' && module.update) {
      // A morph changed the props (REQ-ISL-06): the island keeps its DOM
      // and its state and gets the new values.
      try {
        module.update(parseProps(after).props, this.#root(), ctx)
      } catch (err) {
        this.#fail(err)
      }
      return
    }
    this.#unmount()
    void this.#mount()
  }

  // #root returns the element the island renders into. The server writes
  // it with data-ignore-morph, so a morph never touches what the island
  // put there. A host with no server markup gets one here.
  #root(): HTMLElement {
    for (const child of this.children) {
      if (child instanceof HTMLElement && child.hasAttribute('data-gx-island-root')) return child
    }
    const root = document.createElement('div')
    root.setAttribute('data-gx-island-root', '')
    root.setAttribute('data-ignore-morph', '')
    this.append(root)
    return root
  }

  async #mount(): Promise<void> {
    const run = ++this.#run
    const src = this.getAttribute('src')
    try {
      if (!src) throw new Error('the element has no src; the island is not in the bundle of the app')
      const { props, signals } = parseProps(this.getAttribute('props'))
      const [module, signalsOfPage] = await Promise.all([
        import(src) as Promise<IslandModule>,
        signals ? signalStore() : undefined,
      ])
      if (run !== this.#run || !this.isConnected) return
      if (typeof module.default !== 'function') throw new Error('the file has no default export')
      const controller = new AbortController()
      const stops: Array<() => void> = []
      const ctx: Ctx = {
        abort: controller.signal,
        signal: (ref) => {
          if (!(ref instanceof SignalRef) || !signalsOfPage) throw new Error('the value is not a gx.SignalRef prop')
          const path = ref.path.join('.')
          return {
            get: () => signalsOfPage.getPath(path),
            set: (value) => signalsOfPage.mergePatch(patchFor(ref.path, value)),
            subscribe: (fn) => {
              const stop = signalsOfPage.effect(() => fn(signalsOfPage.getPath(path)))
              stops.push(stop)
              return stop
            },
          }
        },
      }
      const mounted: Mounted = { module, ctx, controller, cleanup: undefined, stops }
      this.#mounted = mounted
      const cleanup = await module.default(this.#root(), props, ctx)
      if (this.#mounted !== mounted) {
        // The element left the page during an async mount.
        if (typeof cleanup === 'function') cleanup()
        return
      }
      mounted.cleanup = cleanup
      this.#internals.states.add('mounted')
      this.dispatchEvent(new CustomEvent('gx:island', { bubbles: true, detail: { name: this.getAttribute('name') } }))
    } catch (err) {
      if (run !== this.#run) return
      // A mount that failed holds no island.
      this.#mounted = undefined
      this.#fail(err)
    }
  }

  #unmount(): void {
    this.#run++
    const mounted = this.#mounted
    if (!mounted) return
    this.#mounted = undefined
    this.#internals.states.delete('mounted')
    mounted.controller.abort()
    for (const stop of mounted.stops) stop()
    try {
      if (typeof mounted.cleanup === 'function') mounted.cleanup()
    } catch (err) {
      this.#fail(err)
    }
    this.#root().replaceChildren()
  }

  #fail(err: unknown): void {
    console.error(`gx: island ${this.getAttribute('name') ?? ''}:`, err)
    this.dispatchEvent(new CustomEvent('gx:island-error', { bubbles: true, detail: { error: err } }))
  }
}

// An imported web component carries the URL of the module that defines it
// (REQ-ISL-09). The loader imports each module one time, for the elements
// of the first page and for the ones a patch brings in.
const modules = new Set<string>()

const loadElementModules = (): void => {
  for (const el of document.querySelectorAll('[data-gx-module]')) {
    const src = el.getAttribute('data-gx-module')
    if (!src || modules.has(src)) continue
    modules.add(src)
    import(src).catch((err) => console.error(`gx: web component <${el.localName}>:`, err))
  }
}

if (!customElements.get('gx-island')) {
  customElements.define('gx-island', GxIsland)
  loadElementModules()
  new MutationObserver(loadElementModules).observe(document.documentElement, { subtree: true, childList: true })
}

export {}
