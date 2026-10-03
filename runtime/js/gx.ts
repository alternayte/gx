// The Gx browser runtime. `just runtime` builds this file to gx.js, which
// package gx embeds. Keep it free of bare imports: users never run a
// bundler for the core runtime.
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
  if (push) history.pushState({ gx: true }, '', url)
  window.scrollTo(0, 0)
  updateActive()
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
  document.addEventListener('DOMContentLoaded', updateActive)
  updateActive()
  document.addEventListener('DOMContentLoaded', checkInstances)
  checkInstances()
  new MutationObserver(checkInstances).observe(document.documentElement, {
    subtree: true,
    childList: true,
  })
}
