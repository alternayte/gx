// The CSRF fetch wrapper of the Gx runtime (SI-03). A browser outside a
// secure context sends no Fetch Metadata, so the server needs the
// gx_csrf token on every same-origin write. Datastar's own fetches do not
// carry it, so the runtime adds it here.

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

// installCSRF wraps fetch so every same-origin write carries the token.
export const installCSRF = (): void => {
  const native = globalThis.fetch.bind(globalThis)
  globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> =>
    native(input, csrfInit(input, init, location.origin, cookieValue('gx_csrf')))
}
