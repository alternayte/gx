// The Gx dev client. The dev server injects this module into every page. It
// listens on the SSE channel for rebuild and error events (REQ-DEV-01,
// REQ-DEV-03, REQ-DEV-06).

type Overlay = {
  title: string
  text: string
  file?: string
  line?: number
  col?: number
  link?: string
}

const showOverlay = (o: Overlay): void => {
  let el = document.getElementById('gx-dev-overlay') as HTMLDivElement | null
  if (!el) {
    el = document.createElement('div')
    el.id = 'gx-dev-overlay'
    el.setAttribute('role', 'alert')
    el.style.cssText =
      'position:fixed;inset:0;z-index:2147483647;overflow:auto;background:#1e1e2e;color:#f8f8f2;font:13px/1.5 ui-monospace,SFMono-Regular,monospace;padding:24px;white-space:pre-wrap'
    document.body.append(el)
  }
  el.textContent = o.text
  if (o.link) {
    const a = document.createElement('a')
    a.href = o.link
    a.textContent = `\n${o.file ?? ''}:${o.line ?? 0}:${o.col ?? 0}`
    a.style.cssText = 'display:block;margin-top:16px;color:#89b4fa'
    el.append(a)
  }
}

const clearOverlay = (): void => {
  document.getElementById('gx-dev-overlay')?.remove()
}

const reload = (): void => {
  clearOverlay()
  const nav = (globalThis as { __gx?: { navigate?: (url: string, push: boolean) => Promise<void> } }).__gx?.navigate
  if (typeof nav === 'function') {
    void nav(location.pathname + location.search, false)
    return
  }
  location.reload()
}

const source = new EventSource('/_gx/dev')
source.addEventListener('overlay', (e) => {
  try {
    showOverlay(JSON.parse((e as MessageEvent).data) as Overlay)
  } catch {
    // ignore a malformed frame
  }
})
source.addEventListener('reload', reload)
