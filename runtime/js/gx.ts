// The Gx browser runtime. `just runtime` builds this file to gx.js, which
// package gx embeds. Keep it free of bare imports: users never run a
// bundler for the core runtime.
import { installCSRF } from './csrf'
//
// It owns the parts Datastar does not: layout-aware navigation (REQ-RTE-12),
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

// cookie reads one cookie value.
const cookie = (name: string): string => {
  const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return m ? decodeURIComponent(m[1]) : ''
}

// Attach the CSRF token to every same-origin write, Datastar's fetches
// included (SI-03).
installCSRF()

// adapterPresent reports whether the page loaded a hypermedia adapter.
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
  await readFrames(res)
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
  await readFrames(res)
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

const navigate = async (url: string, push: boolean): Promise<void> => {
  const res = await fetch(url, {
    headers: {
      'Gx-Nav': '1',
      'Gx-Layouts': layoutChain().join(','),
      'Datastar-Request': 'true',
      Accept: 'text/event-stream',
    },
    credentials: 'same-origin',
  })
  if (!res.ok || !res.body || res.headers.get('Gx-Nav') === 'full') {
    location.href = url
    return
  }
  await withViewTransition(async () => {
    // Gx owns scroll: scroll before the morph so an on:visible element of
    // the new page never sees the old scroll position.
    window.scrollTo(0, 0)
    await readFrames(res)
    if (push) history.pushState({ gx: true }, '', url)
    updateActive()
  })
}

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
  window.addEventListener('popstate', () => {
    void navigate(location.pathname + location.search, false)
  })
  document.addEventListener('submit', (e) => {
    if (!adapterPresent()) return
    const form = (e.target as Element | null)?.closest?.('form[data-gx-form]') as HTMLFormElement | null
    if (!form) return
    e.preventDefault()
    void submitForm(form, (e as SubmitEvent).submitter as HTMLElement | null)
  }, true)
  watchValidation()
  installReducedMotionCSS()
  if ('scrollRestoration' in history) history.scrollRestoration = 'manual' 
  document.addEventListener('DOMContentLoaded', updateActive)
  updateActive()
  document.addEventListener('DOMContentLoaded', checkInstances)
  checkInstances()
  new MutationObserver(checkInstances).observe(document.documentElement, {
    subtree: true,
    childList: true,
  })
}
