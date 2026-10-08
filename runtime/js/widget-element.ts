// The element file of one widget (REQ-ISL-19). A host page loads this file.
// It defines the custom element, fetches the first render from the Gx server
// and gives the answer to the widget script of that server. It holds no
// runtime, so a new build of the server needs no new copy of this file.
//
// `gx wc build` writes the file, and the Gx server serves it at
// /_gx/widgets/<tag>.js: each puts the configuration of one widget in the
// place of __GX_WIDGET_CONFIG__.

type Config = {
  // The element name, for example "acme-cart".
  tag: string
  // The attributes that the element sends to the server.
  attrs: string[]
  // The origin of the Gx server. Empty for the origin of the page.
  server: string
  // The path of the GET route of the widget.
  path: string
  // True for a file that the Gx server serves: the origin of the server is
  // the origin of the URL of this file.
  self?: boolean
}

type WidgetError = { status: number; key: string; field?: string }

type Answer = {
  tag?: string
  html?: string
  script?: string
  build?: string
  error?: WidgetError
}

// The token of the host for this widget: text, or a function that gives the
// text now. A function gives a fresh token after the old one stops.
type Token = string | (() => string | Promise<string>) | undefined

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
const server = config.self ? new URL(import.meta.url).origin : config.server || location.origin

// The children of the element are its fallback content. They show through
// this slot until the first render is in the shadow root (REQ-ISL-11).
const fallback = '<slot></slot>'

class WidgetElement extends HTMLElement {
  // A browser writes the name of an attribute of an HTML element in lower
  // case, and calls attributeChangedCallback by that name.
  static observedAttributes = config.attrs.map((name) => name.toLowerCase())

  #root: ShadowRoot
  #mounted: Mounted | undefined
  // #mounting is the mount that runs now, and #build the build of the
  // server that made the mounted render.
  #mounting: Promise<Mounted> | undefined
  #build: string | undefined
  // #life numbers the lives of the element in a page: a mount of an
  // earlier life is dropped.
  #life = 0
  #abort: AbortController | undefined
  // #turn numbers the loads, so the answer of an old load is dropped.
  #turn = 0
  #queued = false
  #ready = false
  #token: Token

  constructor() {
    super()
    this.#root = this.attachShadow({ mode: 'open' })
    this.#root.innerHTML = fallback
  }

  connectedCallback(): void {
    // A host can set the token before this file defines the element. The
    // value is then a plain property of the node: take it, so the setter of
    // the class gets it.
    if (Object.prototype.hasOwnProperty.call(this, 'token')) {
      const early = (this as unknown as { token: Token }).token
      delete (this as unknown as { token?: Token }).token
      this.#token = early
    }
    this.#queue()
  }

  // token is the token of the host for this widget (REQ-ISL-18). The element
  // sends it as a bearer header on each request to the Gx server. It is a
  // property and never an attribute, so it is in no markup.
  get token(): Token {
    return this.#token
  }

  set token(value: Token) {
    if (value === this.#token) return
    this.#token = value
    // The widget of a different user is a different render.
    if (this.isConnected) this.#queue()
  }

  // #send sends one request of this widget to the Gx server.
  async #send(url: string, init: RequestInit): Promise<Response> {
    const once = async (): Promise<Response> => {
      const token = typeof this.#token === 'function' ? await this.#token() : this.#token
      const headers: Record<string, string> = { ...(init.headers as Record<string, string>), 'Gx-Widget': config.tag }
      if (token) headers.Authorization = 'Bearer ' + token
      return fetch(url, { ...init, headers, credentials: this.#credentials() })
    }
    const res = await once()
    // The token stopped. A token function gives a fresh one, one time.
    if (res.status === 401 && typeof this.#token === 'function') return once()
    return res
  }

  disconnectedCallback(): void {
    this.#turn++
    this.#life++
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
      request: (path, init) => this.#send(server + path, init),
      fail: (error) => this.#fail(error),
      event: (name, detail) => this.dispatchEvent(new CustomEvent(name, { bubbles: true, composed: true, cancelable: true, detail })),
      reload: () => this.#fresh(),
    }
  }

  // #fresh drops the render of an old build and loads the widget again.
  #fresh(): void {
    this.#life++
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
      const res = await this.#send(this.#url(), { headers: { Accept: 'application/json' }, signal: abort.signal })
      let answer: Answer = {}
      try {
        answer = (await res.json()) as Answer
      } catch {
        // An answer that is not JSON is not an answer of a widget route.
      }
      if (turn !== this.#turn) return
      if (!res.ok || answer.error || typeof answer.html !== 'string' || typeof answer.script !== 'string') {
        this.#fail(answer.error ?? { status: res.ok ? 0 : res.status, key: res.status === 401 ? 'gx.unauthorized' : 'gx.error' })
        return
      }
      const runtime = (await import(server + answer.script)) as Runtime
      if (turn !== this.#turn) return
      // One mount at a time. A load that comes while the first render
      // mounts waits for that mount and then updates it: a second mount
      // into the same shadow root, and the end of the first, would leave
      // the root empty (REQ-ISL-16).
      if (!this.#mounted && this.#mounting) {
        await this.#mounting.catch(() => undefined)
        if (turn !== this.#turn) return
      }
      if (this.#mounted && answer.build && this.#build && answer.build !== this.#build) {
        // The server has a new build. Its HTML needs its own stylesheet
        // and its own script, so the widget mounts again (REQ-ISL-19).
        this.#mounted.destroy()
        this.#mounted = undefined
        this.#root.innerHTML = fallback
      }
      if (this.#mounted) {
        this.#mounted.update(answer)
      } else {
        const life = this.#life
        const pending = (this.#mounting = runtime.mount(this.#root, answer, this.#host()))
        let mounted: Mounted
        try {
          mounted = await pending
        } finally {
          if (this.#mounting === pending) this.#mounting = undefined
        }
        if (life !== this.#life) {
          // The element left the page, or loads again from the start.
          mounted.destroy()
          return
        }
        this.#mounted = mounted
        this.#build = answer.build
        // A newer load waits for this mount. It updates the mount and
        // sets the state.
        if (turn !== this.#turn) return
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

// The type file of a widget declares the class of the element. gx puts the
// class name of the widget in the place of this export name, so a host that
// imports the class gets it.
export { WidgetElement as __GX_WIDGET_CLASS__ }
