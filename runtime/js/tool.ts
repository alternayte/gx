// The tools of a page for an agent in the browser (REQ-AI-06). An element
// that invokes an action or a form with .Tool() has the attribute
// data-gx-tool. While such an element is in the document, this module has
// the tool registered with the WebMCP API of the browser. A call of the tool
// runs the action on the server as the element does, and the answer of the
// server changes the page.
//
// The WebMCP draft moved the API from navigator.modelContext to
// document.modelContext. This module is the one place that knows the API.

type Meta = {
  name: string
  description: string
  inputSchema: object
  // confirm is true for a tool with gx.Confirm.
  confirm?: boolean
  // readOnly is true for a GET action.
  readOnly?: boolean
}

type Answer = { text: string; structured?: unknown; isError?: boolean }

type ModelContext = {
  registerTool: (tool: object, options?: { signal: AbortSignal }) => unknown
  // The API before the July 2026 draft took a tool out by its name.
  unregisterTool?: (name: string) => unknown
}

const modelContext = (): ModelContext | undefined =>
  (document as { modelContext?: ModelContext }).modelContext ?? (navigator as { modelContext?: ModelContext }).modelContext

// The address of the tool routes of the app, next to this file.
const base = new URL('./tools/', import.meta.url).href

// live holds each registered tool, with the signal that takes it out.
const live = new Map<string, AbortController>()

const cookie = (name: string): string => {
  for (const part of document.cookie.split('; ')) {
    const [key, ...rest] = part.split('=')
    if (key === name) return decodeURIComponent(rest.join('='))
  }
  return ''
}

// call runs one tool on the server. The request names the scope of the
// element that invokes the tool, so a patch of the answer finds the instance
// of its component.
const call = async (meta: Meta, input: unknown): Promise<unknown> => {
  const el = document.querySelector(`[data-gx-tool="${CSS.escape(meta.name)}"]`)
  if (!el) throw new Error('The page does not have this tool now.')
  // gx.Confirm: the user says yes before the agent runs the tool.
  if (meta.confirm && !window.confirm(meta.description)) throw new Error('The user did not allow the call.')
  const res = await fetch(base + encodeURIComponent(meta.name), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Datastar-Request': 'true',
      Accept: 'text/event-stream',
      'Gx-CSRF': cookie('gx_csrf'),
      'Gx-Scope': el.getAttribute('data-gx-tool-scope') ?? '',
    },
    credentials: 'same-origin',
    body: JSON.stringify(input ?? {}),
  })
  const raw = res.headers.get('Gx-Tool-Answer')
  const answer: Answer = raw
    ? (JSON.parse(decodeURIComponent(raw)) as Answer)
    : { text: `The app refused the call with status ${res.status}.`, isError: true }
  // The answer of the server changes the page, as it does after a click.
  const runtime = (globalThis as { __gx?: { apply?: (res: Response) => Promise<void> } }).__gx
  if (res.ok && res.status !== 204) await runtime?.apply?.(res)
  if (answer.isError) throw new Error(answer.text)
  return answer.structured ?? answer.text
}

const register = async (context: ModelContext, name: string): Promise<void> => {
  const abort = new AbortController()
  live.set(name, abort)
  const res = await fetch(base + encodeURIComponent(name), { credentials: 'same-origin' })
  // The element can leave the page while the description loads.
  if (!res.ok || abort.signal.aborted) {
    if (live.get(name) === abort) live.delete(name)
    return
  }
  const meta = (await res.json()) as Meta
  if (abort.signal.aborted) return
  try {
    await context.registerTool(
      {
        name: meta.name,
        description: meta.description,
        inputSchema: meta.inputSchema,
        annotations: { readOnlyHint: meta.readOnly === true, consequentialHint: meta.confirm === true },
        execute: (input: unknown) => call(meta, input),
      },
      { signal: abort.signal },
    )
  } catch {
    // The browser refused the tool. The page works with no tool.
    if (live.get(name) === abort) live.delete(name)
  }
}

// sync makes the registered tools equal to the tools of the document.
const sync = (context: ModelContext): void => {
  const names = new Set<string>()
  for (const el of document.querySelectorAll('[data-gx-tool]')) names.add(el.getAttribute('data-gx-tool') ?? '')
  names.delete('')
  for (const [name, abort] of live) {
    if (names.has(name)) continue
    live.delete(name)
    abort.abort()
    void context.unregisterTool?.(name)
  }
  for (const name of names) {
    if (!live.has(name)) void register(context, name)
  }
}

const context = modelContext()
// With no WebMCP in the browser this module does nothing.
if (context) {
  let queued = false
  const later = (): void => {
    if (queued) return
    queued = true
    queueMicrotask(() => {
      queued = false
      sync(context)
    })
  }
  new MutationObserver(later).observe(document.documentElement, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ['data-gx-tool'],
  })
  sync(context)
}
