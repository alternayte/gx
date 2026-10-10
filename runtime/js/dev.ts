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
  const nav = (globalThis as { __gx?: { navigate?: (url: string, push: boolean, keep: boolean) => Promise<void> } }).__gx
    ?.navigate
  if (typeof nav === 'function') {
    // keep: the page is the same, so its state stays (REQ-DEV-03).
    void nav(location.pathname + location.search, false, true)
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

// The capture of a fixture (REQ-AI-12): a button opens a dialog with the
// components of the current page; Save writes the props of the last render
// of one component into its <Name>.fixtures.go file.

type Kept = { component: string; package: string }

const json = async (res: Response): Promise<Record<string, unknown>> => {
  try {
    return (await res.json()) as Record<string, unknown>
  } catch {
    return { error: `the dev server answers ${res.status}` }
  }
}

// pageComponents renders the current page again and returns the components
// that the render used.
const pageComponents = async (): Promise<Kept[]> => {
  const before = await json(await fetch('/_gx/dev/props?since=18446744073709551615'))
  await (await fetch(location.pathname + location.search, { headers: { Accept: 'text/html' } })).text()
  const after = await json(await fetch(`/_gx/dev/props?since=${String(before.mark ?? 0)}`))
  return (after.components as Kept[] | undefined) ?? []
}

const el = <K extends keyof HTMLElementTagNameMap>(tag: K, css: string, text = ''): HTMLElementTagNameMap[K] => {
  const node = document.createElement(tag)
  node.style.cssText = css
  node.textContent = text
  return node
}

const field = 'display:block;width:100%;margin:4px 0 12px;padding:6px 8px;font:inherit;color:inherit;background:#313244;border:1px solid #585b70;border-radius:4px'
const control = 'padding:6px 12px;font:inherit;color:#1e1e2e;background:#89b4fa;border:0;border-radius:4px;cursor:pointer'

const openCapture = async (): Promise<void> => {
  document.getElementById('gx-dev-capture-dialog')?.remove()
  const dialog = el('dialog', 'width:min(420px,90vw);padding:16px;color:#f8f8f2;background:#1e1e2e;border:1px solid #585b70;border-radius:8px;font:13px/1.5 ui-monospace,SFMono-Regular,monospace')
  dialog.id = 'gx-dev-capture-dialog'
  dialog.setAttribute('aria-label', 'Save a fixture')
  const form = el('form', 'margin:0')
  form.method = 'dialog'
  const pickLabel = el('label', 'display:block', 'Component')
  const pick = el('select', field)
  pick.name = 'component'
  pickLabel.append(pick)
  const nameLabel = el('label', 'display:block', 'Fixture name')
  const name = el('input', field)
  name.name = 'name'
  name.required = true
  name.pattern = '[A-Za-z][A-Za-z0-9]*'
  name.autocomplete = 'off'
  nameLabel.append(name)
  const status = el('p', 'margin:0 0 12px;min-height:1.5em;white-space:pre-wrap', 'Reading the components of the page...')
  status.setAttribute('role', 'status')
  const save = el('button', control, 'Save')
  save.type = 'submit'
  save.value = 'save'
  save.disabled = true
  const close = el('button', control + ';margin-left:8px;background:#585b70;color:#f8f8f2', 'Close')
  close.type = 'button'
  close.addEventListener('click', () => dialog.close())
  form.append(pickLabel, nameLabel, status, save, close)
  dialog.append(form)
  document.body.append(dialog)
  dialog.showModal()

  let kept: Kept[] = []
  try {
    kept = await pageComponents()
  } catch (err) {
    status.textContent = `The dev server does not answer: ${String(err)}`
    return
  }
  for (const [i, k] of kept.entries()) {
    const option = el('option', '', `${k.component} (${k.package})`)
    option.value = String(i)
    pick.append(option)
  }
  status.textContent = kept.length ? '' : 'This page renders no component of the app.'
  save.disabled = kept.length === 0

  form.addEventListener('submit', (e) => {
    e.preventDefault()
    const chosen = kept[Number(pick.value)]
    if (!chosen) return
    save.disabled = true
    status.textContent = 'Saving...'
    void (async () => {
      const res = await fetch('/_gx/dev/capture', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ component: chosen.component, package: chosen.package, name: name.value }),
      })
      const out = await json(res)
      status.dataset.state = res.ok ? 'saved' : 'error'
      status.textContent = res.ok
        ? `Saved the fixture ${String(out.name)} in ${String(out.file)}.`
        : `Not saved: ${String(out.error)}`
      save.disabled = false
    })()
  })
}

const captureButton = (): void => {
  if (document.getElementById('gx-dev-capture') || !document.body) return
  const button = el(
    'button',
    'position:fixed;left:8px;bottom:8px;z-index:2147483646;padding:2px 8px;font:11px/1.5 ui-monospace,SFMono-Regular,monospace;color:#f8f8f2;background:#1e1e2e;border:1px solid #585b70;border-radius:4px;opacity:.7;cursor:pointer',
    'fixture',
  )
  button.id = 'gx-dev-capture'
  button.type = 'button'
  button.title = 'Save the props of a component of this page as a fixture'
  button.addEventListener('click', () => void openCapture())
  document.body.append(button)
}

// The gallery shows the fixtures; it has no page component to capture.
if (!location.pathname.startsWith('/_gx/')) {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', captureButton)
  } else {
    captureButton()
  }
}
