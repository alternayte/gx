// The CSRF fetch wrapper of the Gx runtime (SI-03). A browser outside a
// secure context sends no Fetch Metadata, so the server needs the
// gx_csrf token on every same-origin write. Datastar's own fetches do not
// carry it, so the runtime adds it here.

import { take, watch, type Kept } from './optimistic'

// cookieValue reads one cookie value.
export const cookieValue = (name: string): string => {
  const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return m ? decodeURIComponent(m[1]) : ''
}

// csrfInit returns init with the Gx-CSRF header for a same-origin non-GET
// request. It returns init unchanged when the token is empty, the request
// is a read or the request is cross-origin.
export const csrfInit = (
  input: RequestInfo | URL,
  init: RequestInit | undefined,
  origin: string,
  token: string,
): RequestInit | undefined => {
  if (token === '') return init
  try {
    const raw = typeof input === 'string' || input instanceof URL ? String(input) : input.url
    const url = new URL(raw, origin)
    if (url.origin !== origin) return init
    let method = init?.method ?? (typeof input === 'object' ? input.method : '')
    method = (method || 'GET').toUpperCase()
    if (method === 'GET' || method === 'HEAD') return init
    const headers = new Headers(init?.headers ?? (typeof input === 'object' ? input.headers : undefined))
    headers.set('Gx-CSRF', token)
    return { ...init, headers }
  } catch {
    return init
  }
}

// fragmentsLimit is the largest Gx-Fragments header that the runtime sends.
// A page with more fragments sends none, and the server then answers with
// each fragment of an update.
const fragmentsLimit = 4096

// fragmentHashes returns the hash of each fragment under root as
// "id=hash,id=hash" (REQ-ACT-15). The server compares them with the hashes
// of its new render and sends only the fragments that differ (REQ-ACT-16).
export const fragmentHashes = (root: ParentNode): string => {
  const parts: string[] = []
  let size = 0
  for (const el of root.querySelectorAll('[data-gx-h][id]')) {
    const part = `${el.id}=${el.getAttribute('data-gx-h') ?? ''}`
    size += part.length + 1
    if (size > fragmentsLimit) return ''
    parts.push(part)
  }
  return parts.join(',')
}

// fragmentsInit returns init with the Gx-Fragments header for a same-origin
// non-GET request. It returns init unchanged when the page has no fragment,
// the request is a read or the request is cross-origin.
export const fragmentsInit = (
  input: RequestInfo | URL,
  init: RequestInit | undefined,
  origin: string,
  hashes: string,
): RequestInit | undefined => {
  if (hashes === '') return init
  try {
    const raw = typeof input === 'string' || input instanceof URL ? String(input) : input.url
    if (new URL(raw, origin).origin !== origin) return init
    let method = init?.method ?? (typeof input === 'object' ? input.method : '')
    method = (method || 'GET').toUpperCase()
    if (method === 'GET' || method === 'HEAD') return init
    const headers = new Headers(init?.headers ?? (typeof input === 'object' ? input.headers : undefined))
    headers.set('Gx-Fragments', hashes)
    return { ...init, headers }
  } catch {
    return init
  }
}

// installCSRF wraps fetch so every same-origin write carries the token and
// the hashes of the fragments of the page. restore gets the values of an
// optimistic update when its request fails (REQ-ACT-18).
export const installCSRF = (restore: (kept: Kept) => void = () => {}): void => {
  const native = globalThis.fetch.bind(globalThis)
  globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> =>
    watch(
      native(
        input,
        fragmentsInit(
          input,
          csrfInit(input, init, location.origin, cookieValue('gx_csrf')),
          location.origin,
          fragmentHashes(document),
        ),
      ),
      take(),
      restore,
    )
}
