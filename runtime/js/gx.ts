// The Gx browser runtime. `just runtime` builds this file to gx.js, which
// package gx embeds. Keep it free of bare imports: users never run a
// bundler for the core runtime.
import { installCSRF } from './csrf'
import { keep, type Kept } from './optimistic'
import { share, type Deps, type Values } from './room'
//
// It owns the parts an adapter does not: layout-aware navigation (REQ-RTE-12),
// active links (REQ-RTE-13), the dev duplicate-scope check (REQ-ACT-06) and
// the JavaScript halves of the gxc helpers (REQ-ACT-13).

export const gx = {
  // len counts Unicode code points, like Go's utf8.RuneCountInString.
  len: (s: string): number => [...s].length,
  // at returns the code point at a rune index, like gxc.At.
  at: (s: string, i: number): string => [...s][i] ?? '',
  // contains and index mirror gxc.Contains and gxc.Index.
  contains: (s: string, sub: string): boolean => s.includes(sub),
  index: (s: string, sub: string): number => {
    const i = s.indexOf(sub)
    return i < 0 ? -1 : [...s.slice(0, i)].length
  },
  // apply puts the answer of the server in the page. The tool module uses
  // it for the answer of a tool call (REQ-AI-06).
  apply: (res: Response): Promise<void> => applyAnswer(res),
  // keep saves the values of the signals that an optimistic directive
  // changes. The runtime puts them back when the action fails
  // (REQ-ACT-18).
  keep: (parts: Kept[]): void => keep(parts),
  // share gives the values of the shared signals of a component to its
  // room (REQ-ACT-21).
  share: (el: Element, values: Values): void => share(el, values, roomDeps),
}

;(globalThis as { __gx?: typeof gx }).__gx = gx

// In dev, two signal instances that share a scope are a bug: a patch would
// reach both. The compiler reports what it can see (GX2012); this catches
// the rest.
const checkInstances = (): void => {
  if (!document.querySelector('meta[name="gx-dev"]')) return
  const seen = new Set<string>()
  document.querySelectorAll('[data-gx-instance]').forEach((el) => {
    const id = el.getAttribute('data-gx-instance') ?? ''
    if (id === '') return
    if (seen.has(id)) {
      console.error(`gx: two signal instances share the scope ${id}; add key={...}`)
    }
    seen.add(id)
  })
}

// updateActive recomputes aria-current and data-active after a navigation
// (REQ-RTE-13).
const updateActive = (): void => {
  const here = location.pathname + location.search
  document.querySelectorAll<HTMLAnchorElement>('a[data-gx-active]').forEach((a) => {
    const mode = a.getAttribute('data-gx-active') ?? ''
    const target = new URL(a.getAttribute('href') ?? '', document.baseURI)
    const at = target.pathname + target.search
    const base = target.pathname.replace(/\/$/, '')
    const page = mode === 'page' && at === here
    const section = mode === 'section' && (here === at || here.startsWith(base + '/'))
    if (page || section) {
      if (page) a.setAttribute('aria-current', 'page')
      if (section) a.setAttribute('data-active', '')
    } else {
      a.removeAttribute('aria-current')
      a.removeAttribute('data-active')
    }
  })
}

// reduceMotion reports the user's motion preference (REQ-STY-10).
const reduceMotion = (): boolean => window.matchMedia('(prefers-reduced-motion: reduce)').matches

// dedupeTransitionNames keeps the first view-transition-name and clears
// duplicates before a transition (REQ-STY-08).
const dedupeTransitionNames = (): void => {
  const seen = new Set<string>()
  document.querySelectorAll<HTMLElement>('[style*="view-transition-name"]').forEach((el) => {
    const name = el.style.getPropertyValue('view-transition-name')
    if (name === '') return
    if (seen.has(name)) {
      el.style.removeProperty('view-transition-name')
      el.style.removeProperty('view-transition-class')
      if (document.querySelector('meta[name="gx-dev"]')) {
        console.warn(`gx: duplicate view-transition-name ${name}; the first element keeps it`)
      }
      return
    }
    seen.add(name)
  })
}

// withViewTransition runs update inside a view transition when the browser
// supports one and the user allows motion (REQ-STY-09, REQ-STY-10).
const withViewTransition = async (update: () => Promise<void>): Promise<void> => {
  const doc = document as Document & {
    startViewTransition?: (cb: () => Promise<void>) => { finished: Promise<void> }
  }
  if (typeof doc.startViewTransition !== 'function' || reduceMotion()) {
    await update()
    return
  }
  dedupeTransitionNames()
  const transition = doc.startViewTransition(async () => {
    await update()
  })
  try {
    await transition.finished
  } catch {
    // A skipped transition is not a page error.
  }
}

// installShell wires the docs-shell behaviours (REQ-CNT-06): theme select,
// search dialog, mobile menu and table-of-contents scroll spy.
const themeKey = 'gx-theme'

const applyTheme = (mode: string): void => {
  const root = document.documentElement
  root.classList.remove('dark', 'light')
  if (mode === 'dark') root.classList.add('dark')
  else if (mode === 'light') root.classList.add('light')
  document.querySelectorAll<HTMLElement>('[data-gx-theme]').forEach((button) => {
    // A theme control is a button for each mode, or one select.
    if (button instanceof HTMLSelectElement) {
      button.value = mode
      return
    }
    const on = button.getAttribute('data-gx-theme') === mode
    button.setAttribute('aria-pressed', on ? 'true' : 'false')
    if (on) button.setAttribute('data-active', '')
    else button.removeAttribute('data-active')
  })
}

const storedTheme = (): string => {
  try {
    return localStorage.getItem(themeKey) ?? 'auto'
  } catch {
    return 'auto'
  }
}

const openSearch = (): void => {
  const dialog = document.querySelector<HTMLDialogElement>('dialog[data-gx-search]')
  if (!dialog) return
  if (!dialog.open) dialog.showModal()
  dialog.querySelector<HTMLInputElement>('[data-gx-search-input]')?.focus()
}

// spyTOC marks the table-of-contents entry of the heading nearest the top
// of the viewport (REQ-CNT-06).
const spyTOC = (): void => {
  document.querySelectorAll('[data-gx-toc]').forEach(spyOneTOC)
}

const spyOneTOC = (toc: Element): void => {
  const links = [...toc.querySelectorAll<HTMLElement>('[data-gx-toc-target]')]
  let active = ''
  for (const link of links) {
    const id = link.getAttribute('data-gx-toc-target') ?? ''
    const heading = id === '' ? null : document.getElementById(id)
    if (heading && heading.getBoundingClientRect().top <= 96) active = id
  }
  // At the end of the page the last heading is the one being read, even
  // when it cannot reach the top of the viewport.
  if (links.length > 0 && window.innerHeight + window.scrollY >= document.documentElement.scrollHeight - 2) {
    active = links[links.length - 1].getAttribute('data-gx-toc-target') ?? active
  }
  for (const link of links) {
    const on = link.getAttribute('data-gx-toc-target') === active
    if (on) {
      link.setAttribute('aria-current', 'location')
      link.setAttribute('data-active', '')
      // A closed table of contents shows the name of the current entry.
      // The write is only for a new name: the page observer runs this
      // function again after each change of the document.
      const current = toc.querySelector('[data-gx-toc-current]')
      if (current && current.textContent !== link.textContent) current.textContent = link.textContent
    } else {
      link.removeAttribute('aria-current')
      link.removeAttribute('data-active')
    }
  }
}

const installShell = (): void => {
  // The search shortcut shows the Command key on an Apple device.
  if (/Mac|iPhone|iPod|iPad/i.test(navigator.platform)) {
    document.querySelectorAll('[data-gx-mod-key]').forEach((key) => {
      if (key.textContent !== '\u2318') key.textContent = '\u2318'
    })
  }
  applyTheme(storedTheme())
  syncSidebar()
  spyTOC()
}

// syncSidebar shows the sidebar on wide screens and when the mobile menu is
// open (REQ-CNT-06).
const syncSidebar = (): void => {
  const side = document.getElementById('gx-sidebar')
  if (!side) return
  // The value of data-gx-sidebar is the width from which the sidebar always
  // shows.
  const from = side.getAttribute('data-gx-sidebar') || '1024px'
  const wide = window.matchMedia(`(min-width: ${from})`).matches
  const open = side.hasAttribute('data-open')
  side.hidden = !wide && !open
}

// installShellEvents installs the delegated shell handlers once.
const installShellEvents = (): void => {
  // Another document of the origin changed the theme: a second tab, or the
  // page around a frame. Follow it.
  window.addEventListener('storage', (e) => {
    if (e.key === themeKey) applyTheme(e.newValue ?? 'auto')
  })
  const chooseTheme = (mode: string): void => {
    applyTheme(mode)
    try {
      localStorage.setItem(themeKey, mode)
    } catch {
      // Private mode has no storage.
    }
  }
  document.addEventListener('change', (e) => {
    const select = e.target
    if (!(select instanceof HTMLSelectElement)) return
    if (select.hasAttribute('data-gx-theme')) chooseTheme(select.value)
    // A select of site addresses, such as the versions of the docs.
    else if (select.hasAttribute('data-gx-goto') && select.value !== '') window.location.href = select.value
  })
  document.addEventListener('click', (e) => {
    const at = e.target as Element | null
    // A table of contents in a menu closes after a choice and on a click
    // outside it.
    document.querySelectorAll<HTMLDetailsElement>('details[data-gx-toc-menu][open]').forEach((menu) => {
      if (!at || !menu.contains(at) || at.closest('a')) menu.open = false
    })
    const theme = at?.closest?.('[data-gx-theme]') as HTMLElement | null
    if (theme && !(theme instanceof HTMLSelectElement)) {
      chooseTheme(theme.getAttribute('data-gx-theme') ?? 'auto')
      return
    }
    if (at?.closest?.('[data-gx-search-open]')) {
      openSearch()
      return
    }
    const close = at?.closest?.('[data-gx-search-close]')
    if (close) {
      close.closest('dialog')?.close()
      return
    }
    const menu = at?.closest?.('[data-gx-menu]') as HTMLElement | null
    if (menu) {
      const side = document.getElementById('gx-sidebar')
      if (side) {
        const open = side.hasAttribute('data-open')
        if (open) side.removeAttribute('data-open')
        else side.setAttribute('data-open', '')
        menu.setAttribute('aria-expanded', open ? 'false' : 'true')
        syncSidebar()
      }
    }
  })
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      openSearch()
    }
    if (e.key === 'Escape') {
      document.querySelectorAll<HTMLDetailsElement>('details[data-gx-toc-menu][open]').forEach((menu) => {
        const focused = menu.contains(document.activeElement)
        menu.open = false
        if (focused) menu.querySelector<HTMLElement>('summary')?.focus()
      })
    }
  })
  window.addEventListener('resize', syncSidebar)
  let queued = false
  window.addEventListener(
    'scroll',
    () => {
      if (queued) return
      queued = true
      requestAnimationFrame(() => {
        queued = false
        spyTOC()
      })
    },
    { passive: true },
  )
}

// installSearch wires the docs-shell search dialog to Pagefind
// (REQ-CNT-07). The module and index load on first use.
type PagefindResult = { url: string; excerpt: string; meta: { title?: string } }
type Pagefind = {
  init: () => Promise<void>
  search: (q: string) => Promise<{ results: { data: () => Promise<PagefindResult> }[] }>
}

let pagefindModule: Pagefind | null = null
let pagefindLoading: Promise<Pagefind | null> | null = null

const escapeHTML = (s: string): string =>
  s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

// searchExcerpt keeps Pagefind's <mark> highlights and escapes the rest.
const searchExcerpt = (s: string): string =>
  escapeHTML(s)
    .replace(/&lt;mark&gt;/g, '<mark>')
    .replace(/&lt;\/mark&gt;/g, '</mark>')

const loadPagefind = async (): Promise<Pagefind | null> => {
  if (pagefindModule) return pagefindModule
  if (!pagefindLoading) {
    const dialog = document.querySelector('[data-gx-search]')
    const src = dialog?.getAttribute('data-gx-search-src') ?? '/pagefind/pagefind.js'
    pagefindLoading = import(/* @vite-ignore */ src)
      .then(async (mod: Pagefind) => {
        await mod.init()
        pagefindModule = mod
        return mod
      })
      .catch(() => {
        pagefindLoading = null
        return null
      })
  }
  return pagefindLoading
}

const runSearch = async (query: string): Promise<void> => {
  const box = document.querySelector<HTMLElement>('[data-gx-search-results]')
  if (!box) return
  const q = query.trim()
  if (q === '') {
    box.innerHTML = ''
    return
  }
  const pf = await loadPagefind()
  if (!pf) {
    box.textContent = 'Search is not available.'
    return
  }
  const search = await pf.search(q)
  const results = await Promise.all(search.results.slice(0, 8).map((r) => r.data()))
  if (results.length === 0) {
    box.textContent = 'No results.'
    return
  }
  box.innerHTML = results
    .map(
      (r) =>
        `<a class="gx-search-result block rounded-md p-2 no-underline hover:bg-accent" href="${escapeHTML(r.url)}">` +
        `<span class="block font-medium">${escapeHTML(r.meta.title ?? r.url)}</span>` +
        `<span class="block text-muted-foreground">${searchExcerpt(r.excerpt)}</span></a>`,
    )
    .join('')
}

const installSearch = (): void => {
  let timer = 0
  document.addEventListener('input', (e) => {
    const input = (e.target as Element | null)?.closest?.('[data-gx-search-input]') as HTMLInputElement | null
    if (!input) return
    const value = input.value
    window.clearTimeout(timer)
    timer = window.setTimeout(() => void runSearch(value), 200)
  })
  document.addEventListener('submit', (e) => {
    const form = (e.target as Element | null)?.closest?.('[data-gx-search-form]')
    if (!form) return
    e.preventDefault()
    const input = form.querySelector<HTMLInputElement>('[data-gx-search-input]')
    void runSearch(input?.value ?? '')
  })
}

// installCopyButtons copies the code of a highlighted block (REQ-CNT-04).
const installCopyButtons = (): void => {
  document.addEventListener('click', (e) => {
    const button = (e.target as Element | null)?.closest?.('[data-gx-copy]') as HTMLElement | null
    if (!button) return
    const code = button.closest('figure.gx-code')?.querySelector('pre code')
    if (!code) return
    const text = code.textContent ?? ''
    const done = (): void => {
      const old = button.textContent
      button.textContent = 'Copied'
      window.setTimeout(() => {
        button.textContent = old
      }, 1000)
    }
    if (navigator.clipboard?.writeText) {
      void navigator.clipboard.writeText(text).then(done, () => undefined)
    }
  })
}

// installReducedMotionCSS stops the transition pseudo-element animations
// when the user asks for reduced motion (REQ-STY-10).
const installReducedMotionCSS = (): void => {
  const style = document.createElement('style')
  style.textContent =
    '@media (prefers-reduced-motion: reduce) { ::view-transition-group(*), ::view-transition-old(*), ::view-transition-new(*) { animation: none !important; } }'
  document.head.append(style)
}

type Frame = { event: string; data: string[] }

const parseFrame = (raw: string): Frame | null => {
  let event = ''
  const data: string[] = []
  for (const line of raw.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
  }
  if (event === '') return null
  return { event, data }
}

const mergeHead = (payload: string): void => {
  let head: {
    title?: string
    meta?: { name?: string; property?: string; content?: string }[]
    links?: { rel?: string; href?: string }[]
  }
  try {
    head = JSON.parse(payload)
  } catch {
    return
  }
  if (head.title) document.title = head.title
  for (const m of head.meta ?? []) {
    const selector = m.name ? `meta[name="${m.name}"]` : `meta[property="${m.property}"]`
    let el = document.head.querySelector(selector)
    if (!el) {
      el = document.createElement('meta')
      if (m.name) el.setAttribute('name', m.name)
      if (m.property) el.setAttribute('property', m.property)
      document.head.append(el)
    }
    el.setAttribute('content', m.content ?? '')
  }
  for (const l of head.links ?? []) {
    const selector = `link[rel="${l.rel}"]`
    let el = document.head.querySelector(selector)
    if (!el) {
      el = document.createElement('link')
      el.setAttribute('rel', l.rel ?? '')
      document.head.append(el)
    }
    el.setAttribute('href', l.href ?? '')
  }
}

// applyFrame hands a Datastar patch frame to Datastar's own watcher. The
// pinned runtime listens for datastar-fetch events on the document.
const applyFrame = (frame: Frame): void => {
  if (frame.event === 'gx-head') {
    mergeHead(frame.data.join('\n'))
    return
  }
  if (frame.event !== 'datastar-patch-elements' && frame.event !== 'datastar-patch-signals') return
  const argsRaw: Record<string, string> = {}
  for (const line of frame.data) {
    const i = line.indexOf(' ')
    const key = i < 0 ? line : line.slice(0, i)
    const value = i < 0 ? '' : line.slice(i + 1)
    argsRaw[key] = argsRaw[key] === undefined ? value : `${argsRaw[key]}\n${value}`
  }
  document.dispatchEvent(
    new CustomEvent('datastar-fetch', {
      detail: { type: frame.event, el: document.documentElement, argsRaw },
    }),
  )
}

const layoutChain = (): string[] =>
  [...document.querySelectorAll('[data-gx-slot]')].map((el) => el.getAttribute('data-gx-slot') ?? '')

// readFrames applies every SSE frame of a Gx response.
const readFrames = async (res: Response): Promise<void> => {
  if (!res.body) return
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    let at = buffer.indexOf('\n\n')
    while (at >= 0) {
      const frame = parseFrame(buffer.slice(0, at))
      buffer = buffer.slice(at + 2)
      if (frame) applyFrame(frame)
      at = buffer.indexOf('\n\n')
    }
  }
}

// An adapter that does not answer with an event stream sets __gxAdapter:
// apply puts the answer of the server in the page (REQ-ACT-09).
type AnswerAdapter = { apply: (res: Response) => Promise<void> }

const isStream = (res: Response): boolean => (res.headers.get('Content-Type') ?? '').includes('text/event-stream')

// hasPatches reports whether an answer holds patches, not a page.
const hasPatches = (res: Response): boolean => isStream(res) || res.headers.get('Gx-Answer') === 'patches'

// applyAnswer applies the patches of one answer through the adapter.
const applyAnswer = async (res: Response): Promise<void> => {
  if (isStream(res)) return readFrames(res)
  await (globalThis as { __gxAdapter?: AnswerAdapter }).__gxAdapter?.apply(res)
}

// cookie reads one cookie value.
const cookie = (name: string): string => {
  const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return m ? decodeURIComponent(m[1]) : ''
}

// restoreSignals puts the saved values of an optimistic update back into
// the signals of the page, as a signal patch of the server does
// (REQ-ACT-18).
const restoreSignals = (kept: Kept): void => patchSignals(kept)

// patchSignals puts values into the signals of the page, as a signal patch
// of the server does.
const patchSignals = (signals: Record<string, unknown>): void => {
  document.dispatchEvent(
    new CustomEvent('datastar-fetch', {
      detail: { type: 'datastar-patch-signals', el: document.documentElement, argsRaw: { signals: JSON.stringify(signals) } },
    }),
  )
}

// roomDeps connects a room to the browser (REQ-ACT-21).
const roomDeps: Deps = {
  open: (url, receive) => {
    const source = new EventSource(url)
    source.addEventListener('gx-room', (e) => receive(JSON.parse((e as MessageEvent<string>).data) as Values))
    return () => source.close()
  },
  post: (url, room, signals) =>
    fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify({ room, signals }),
    }).then(
      (res) => res.ok,
      () => false,
    ),
  patch: (scope, values) => {
    let nested: Record<string, unknown> = values
    for (let i = scope.length - 1; i >= 0; i--) nested = { [scope[i]]: nested }
    patchSignals(nested)
  },
  later: (fn) => setTimeout(fn, 50),
}

// Attach the CSRF token to every same-origin write, Datastar's fetches
// included (SI-03).
installCSRF(restoreSignals)

// adapterPresent reports whether the page loaded a hypermedia adapter.
// loadIslands loads the island loader when a patch or a morph navigation
// brings the first island into a page that started with none (REQ-ISL-04).
// A page that the server renders with an island has the loader in its head.
const loadIslands = (): void => {
  loadTools()
  if (customElements.get('gx-island') || !document.querySelector('gx-island, [data-gx-module]')) return
  void import(new URL('./island.js', import.meta.url).href)
}

// loadTools loads the tool module when a patch brings the first element
// with a tool into a page that started with none (REQ-AI-06).
let toolsLoaded = false
const loadTools = (): void => {
  if (toolsLoaded || !document.querySelector('[data-gx-tool]')) return
  toolsLoaded = true
  void import(new URL('./tool.js', import.meta.url).href)
}

const adapterPresent = (): boolean => document.querySelector('script[data-gx-adapter]') !== null

// validateTimers debounces input validation per element (REQ-FRM-06).
const validateTimers = new WeakMap<Element, number>()

// validateField posts one field value to its validation URL (REQ-FRM-06).
const validateField = async (el: HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement): Promise<void> => {
  const url = el.getAttribute('data-gx-validate-url') ?? ''
  const name = el.getAttribute('name') ?? ''
  if (url === '' || name === '') return
  const body = new URLSearchParams()
  if (el instanceof HTMLInputElement && el.type === 'checkbox') {
    if (el.checked) body.set(name, el.value === '' ? 'on' : el.value)
  } else {
    body.set(name, el.value)
  }
  const token = cookie('gx_csrf')
  if (token !== '' && !body.has('gx_csrf')) body.set('gx_csrf', token)
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
      'Datastar-Request': 'true',
      Accept: 'text/event-stream',
      'Gx-CSRF': token,
    },
    credentials: 'same-origin',
    body,
  })
  if (!res.ok || !res.body) return
  await applyAnswer(res)
}

// watchValidation wires blur and input validation (REQ-FRM-06).
const watchValidation = (): void => {
  document.addEventListener('blur', (e) => {
    const el = e.target as HTMLInputElement | null
    if (el?.getAttribute?.('data-gx-validate') !== 'blur') return
    void validateField(el)
  }, true)
  document.addEventListener('input', (e) => {
    const el = e.target as HTMLInputElement | null
    if (el?.getAttribute?.('data-gx-validate') !== 'input') return
    const timer = validateTimers.get(el)
    if (timer !== undefined) clearTimeout(timer)
    validateTimers.set(el, window.setTimeout(() => void validateField(el), 300))
  }, true)
}

// submitForm posts a Gx form through the adapter and applies the patches
// (REQ-FRM-05). Native validation has already run; this path skips only the
// browser's own form post.
const submitForm = async (form: HTMLFormElement, submitter: HTMLElement | null): Promise<void> => {
  const data = new FormData(form)
  const token = cookie('gx_csrf')
  if (token !== '' && !data.has('gx_csrf')) data.set('gx_csrf', token)
  const multipart = (form.getAttribute('enctype') ?? '').toLowerCase() === 'multipart/form-data'
  const action = submitter?.getAttribute('formaction') ?? form.action
  const headers: Record<string, string> = {
    'Datastar-Request': 'true',
    Accept: 'text/event-stream',
    'Gx-CSRF': token,
  }
  let body: URLSearchParams | FormData
  if (multipart) {
    body = data
  } else {
    body = new URLSearchParams()
    data.forEach((value, key) => {
      if (typeof value === 'string') body.append(key, value)
    })
    headers['Content-Type'] = 'application/x-www-form-urlencoded'
  }
  const res = await fetch(action, {
    method: (form.getAttribute('method') ?? 'post').toUpperCase(),
    headers,
    credentials: 'same-origin',
    body,
  })
  if (!res.ok || !res.body) {
    form.submit()
    return
  }
  await applyAnswer(res)
  // Datastar morphs the patched form a moment later. Focus the error
  // summary once it lands (REQ-FRM-11).
  for (let i = 0; i < 20; i++) {
    const summary = document.querySelector<HTMLElement>('[data-gx-error-summary]')
    if (summary) {
      summary.focus()
      break
    }
    await new Promise((resolve) => setTimeout(resolve, 16))
  }
  updateActive()
}

// navigate fetches the patch of a page and applies it. keep is for the
// reload of the dev loop: the page is the same, so the scroll position and
// the signal values stay (REQ-DEV-03).
const navigate = async (url: string, push: boolean, keep = false): Promise<void> => {
  const headers: Record<string, string> = {
    'Gx-Nav': '1',
    'Gx-Layouts': layoutChain().join(','),
    'Datastar-Request': 'true',
    Accept: 'text/event-stream',
  }
  if (keep) headers['Gx-Dev-Reload'] = '1'
  const res = await fetch(url, { headers, credentials: 'same-origin' })
  // A page with no layout slot answers HTML, not patches: load it in full.
  if (!res.ok || !res.body || !hasPatches(res) || res.headers.get('Gx-Nav') === 'full') {
    if (keep) location.reload()
    else location.href = url
    return
  }
  // The new page can have a stylesheet of its own (REQ-STY-13). It loads
  // with no effect on the page of now, and takes the place of the
  // stylesheet of now when the new page is in the document. The rules of
  // two stylesheets never apply at the same time: their order decides
  // between a class and its variant.
  const sheets = [...document.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"][href*="/_gx/"]')]
  const need = res.headers.get('Gx-Sheet') ?? ''
  let fresh: HTMLLinkElement | undefined
  if (need !== '' && !sheets.some((l) => l.getAttribute('href') === need)) {
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.media = 'not all'
    link.href = need
    await new Promise<void>((done) => {
      link.onload = link.onerror = () => done()
      document.head.append(link)
    })
    fresh = link
  }
  // A morph can make the page short for a moment, and the browser then
  // moves the scroll position to the top. keep puts it back.
  const left = window.scrollX
  const top = window.scrollY
  await withViewTransition(async () => {
    // Gx owns scroll: scroll before the morph so an on:visible element of
    // the new page never sees the old scroll position.
    if (!keep) window.scrollTo(0, 0)
    await applyAnswer(res)
    if (fresh) {
      fresh.media = 'all'
      for (const old of sheets) old.remove()
    }
    if (keep) window.scrollTo(left, top)
    if (push) {
      history.pushState({ gx: true }, '', url)
      ;(gx as typeof gx & { shownPage?: (url: string) => void }).shownPage?.(url)
    }
    updateActive()
  })
}

// The dev client morphs the page through navigate after a rebuild
// (REQ-DEV-03).
;(gx as typeof gx & { navigate?: typeof navigate }).navigate = navigate
// An adapter with an HTML answer hands the head of a navigation to the
// runtime (REQ-ACT-09).
;(gx as typeof gx & { mergeHead?: typeof mergeHead }).mergeHead = mergeHead

if (typeof document !== 'undefined') {
  document.addEventListener('click', (e) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    if (!document.querySelector('[data-gx-slot]')) return
    const anchor = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null
    if (!anchor) return
    if (anchor.target && anchor.target !== '_self') return
    if (anchor.hasAttribute('download') || anchor.dataset.gxNav === 'off') return
    const raw = anchor.getAttribute('href') ?? ''
    if (raw.startsWith('#')) return
    const url = new URL(anchor.href, document.baseURI)
    if (url.origin !== location.origin) return
    if (url.pathname === location.pathname && url.search === location.search) return
    e.preventDefault()
    void navigate(url.pathname + url.search + url.hash, true)
  })
  // The browser sends popstate for a link to a fragment of the same page
  // too. Only a change of the path or of the query is a navigation: a
  // fragment link must not load the page again.
  let shown = location.pathname + location.search
  window.addEventListener('popstate', () => {
    const next = location.pathname + location.search
    if (next === shown) return
    shown = next
    void navigate(next, false)
  })
  ;(gx as typeof gx & { shownPage?: (url: string) => void }).shownPage = (url: string) => {
    const u = new URL(url, location.href)
    shown = u.pathname + u.search
  }
  document.addEventListener('submit', (e) => {
    if (!adapterPresent()) return
    const form = (e.target as Element | null)?.closest?.('form[data-gx-form]') as HTMLFormElement | null
    if (!form) return
    e.preventDefault()
    void submitForm(form, (e as SubmitEvent).submitter as HTMLElement | null)
  }, true)
  watchValidation()
  installCopyButtons()
  installReducedMotionCSS()
  installShellEvents()
  installShell()
  installSearch()
  if ('scrollRestoration' in history) history.scrollRestoration = 'manual' 
  document.addEventListener('DOMContentLoaded', updateActive)
  updateActive()
  document.addEventListener('DOMContentLoaded', checkInstances)
  checkInstances()
  new MutationObserver(() => {
    checkInstances()
    installShell()
    loadIslands()
  }).observe(document.documentElement, {
    subtree: true,
    childList: true,
  })
}
