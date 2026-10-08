// The element file of one widget (REQ-ISL-19). A host page loads this file.
// It defines the custom element, fetches the first render from the Gx server
// and gives the answer to the widget script of that server. It holds no
// runtime, so a new build of the server needs no new copy of this file.
//
// `gx wc build` writes the file: it puts the configuration of one widget in
// the place of __GX_WIDGET_CONFIG__.

type Config = {
  // The element name, for example "acme-cart".
  tag: string
  // The attributes that the element sends to the server.
  attrs: string[]
  // The origin of the Gx server. Empty for the origin of the page.
  server: string
  // The path of the GET route of the widget.
  path: string
}

type WidgetError = { status: number; key: string; field?: string }

type Answer = {
  tag?: string
  html?: string
  script?: string
  build?: string
  error?: WidgetError
}

type Mounted = {
  update: (answer: Answer) => void
  destroy: () => void
}

// What the element gives the widget script.
type Host = {
  server: string
  request: (path: string, init: RequestInit) => Promise<Response>
  fail: (error: WidgetError) => void
  event: (name: string, detail: unknown) => boolean
  reload: () => void
}

type Runtime = {
  mount: (root: ShadowRoot, answer: Answer, host: Host) => Promise<Mounted>
}

declare const __GX_WIDGET_CONFIG__: Config

const config: Config = __GX_WIDGET_CONFIG__
const server = config.server || location.origin

// The children of the element are its fallback content. They show through
// this slot until the first render is in the shadow root (REQ-ISL-11).
const fallback = '<slot></slot>'

class WidgetElement extends HTMLElement {
  static observedAttributes = config.attrs

  #root: ShadowRoot
  #mounted: Mounted | undefined
  #abort: AbortController | undefined
  // #turn numbers the loads, so the answer of an old load is dropped.
  #turn = 0
  #queued = false
  #ready = false

  constructor() {
    super()
    this.#root = this.attachShadow({ mode: 'open' })
    this.#root.innerHTML = fallback
  }

  connectedCallback(): void {
    this.#queue()
  }

  disconnectedCallback(): void {
    this.#turn++
    this.#abort?.abort()
    this.#mounted?.destroy()
    this.#mounted = undefined
    this.#root.innerHTML = fallback
  }

  attributeChangedCallback(_name: string, before: string | null, after: string | null): void {
    if (before !== after && this.isConnected) this.#queue()
  }

  // #queue starts one load for the attribute changes of one task.
  #queue(): void {
    if (this.#queued) return
    this.#queued = true
    queueMicrotask(() => {
      this.#queued = false
      if (this.isConnected) void this.#load()
    })
  }

  // #host is what the widget script gets from the element: the requests of
  // the widget, and the events to the host page.
  #host(): Host {
    return {
      server,
      request: (path, init) =>
        fetch(server + path, {
          ...init,
          headers: { ...(init.headers as Record<string, string>), 'Gx-Widget': config.tag },
          credentials: this.#credentials(),
        }),
      fail: (error) => this.#fail(error),
      event: (name, detail) => this.dispatchEvent(new CustomEvent(name, { bubbles: true, composed: true, cancelable: true, detail })),
      reload: () => this.#fresh(),
    }
  }

  // #fresh drops the render of an old build and loads the widget again.
  #fresh(): void {
    this.#mounted?.destroy()
    this.#mounted = undefined
    this.#root.innerHTML = fallback
    this.#queue()
  }

  #url(): string {
    const query = new URLSearchParams()
    for (const name of config.attrs) {
      const value = this.getAttribute(name)
      // An attribute with no value is a bool that is true.
      if (value !== null) query.set(name, value === '' ? 'true' : value)
    }
    const text = query.toString()
    return server + config.path + (text ? '?' + text : '')
  }

  // #credentials is the cookie rule of the element (SI-14). A request to a
  // different origin carries no cookie, unless the host sets gx-credentials:
  // the server then must list the origin of the host in AllowCredentials.
  #credentials(): RequestCredentials {
    if (this.hasAttribute('gx-credentials')) return 'include'
    return new URL(server, location.href).origin === location.origin ? 'same-origin' : 'omit'
  }

  async #load(): Promise<void> {
    const turn = ++this.#turn
    this.#abort?.abort()
    const abort = (this.#abort = new AbortController())
    if (!this.#mounted) this.#state('loading')
    try {
      const res = await fetch(this.#url(), {
        headers: { 'Gx-Widget': config.tag, Accept: 'application/json' },
        credentials: this.#credentials(),
        signal: abort.signal,
      })
      let answer: Answer = {}
      try {
        answer = (await res.json()) as Answer
      } catch {
        // An answer that is not JSON is not an answer of a widget route.
      }
      if (turn !== this.#turn) return
      if (!res.ok || answer.error || typeof answer.html !== 'string' || typeof answer.script !== 'string') {
        this.#fail(answer.error ?? { status: res.ok ? 0 : res.status, key: 'gx.error' })
        return
      }
      const runtime = (await import(server + answer.script)) as Runtime
      if (turn !== this.#turn) return
      if (this.#mounted) {
        this.#mounted.update(answer)
      } else {
        const mounted = await runtime.mount(this.#root, answer, this.#host())
        if (turn !== this.#turn) {
          mounted.destroy()
          return
        }
        this.#mounted = mounted
      }
      this.#state('ready')
      if (!this.#ready) {
        this.#ready = true
        this.dispatchEvent(new CustomEvent('gx-ready', { bubbles: true, composed: true }))
      }
    } catch {
      if (turn !== this.#turn || abort.signal.aborted) return
      // The server did not answer, or its script did not load.
      this.#fail({ status: 0, key: 'gx.network' })
    }
  }

  #state(state: 'loading' | 'ready' | 'error'): void {
    if (this.getAttribute('data-gx-state') !== state) this.setAttribute('data-gx-state', state)
  }

  // #fail reports an error to the host. The detail holds a status and a
  // message key, and no text of the server (REQ-ISL-16).
  #fail(error: WidgetError): void {
    this.#state('error')
    const detail: WidgetError = { status: error.status, key: error.key }
    if (error.field) detail.field = error.field
    this.dispatchEvent(new CustomEvent('gx-error', { bubbles: true, composed: true, detail }))
  }
}

if (!customElements.get(config.tag)) customElements.define(config.tag, WidgetElement)
