// The widget script of a Gx server (REQ-ISL-19). The element file of a host
// page imports it from the server, so it is always the script of the build
// that rendered the HTML. The server puts Idiomorph before this code in the
// same file.

type Answer = {
  tag?: string
  html?: string
  // The path of the stylesheet of the widget on the server.
  style?: string
}

type Mounted = {
  update: (answer: Answer) => void
  destroy: () => void
}

type MorphOptions = {
  morphStyle: 'innerHTML' | 'outerHTML'
  callbacks: {
    beforeNodeMorphed: (from: Node, to: Node) => boolean
    beforeAttributeUpdated: (name: string, node: Node, kind: string) => boolean
  }
}

declare const Idiomorph: {
  morph: (target: Node, content: string, options: MorphOptions) => void
}

// userState names, for each form field of one morph, the state that the
// server did not change in the new render. A morph keeps that state as the
// user left it: the text of a field, a check mark, the choice of a select.
// When the new render has a different value, the value of the server wins.
const userState = new WeakMap<Node, Set<string>>()

const morphOptions: MorphOptions = {
  morphStyle: 'innerHTML',
  callbacks: {
    beforeNodeMorphed: (from, to) => {
      const same = new Set<string>()
      if (from instanceof HTMLInputElement && to instanceof HTMLInputElement) {
        if (from.getAttribute('value') === to.getAttribute('value')) same.add('value')
        if (from.hasAttribute('checked') === to.hasAttribute('checked')) same.add('checked')
      } else if (from instanceof HTMLTextAreaElement && to instanceof HTMLTextAreaElement) {
        if (from.defaultValue === to.defaultValue) same.add('value')
      } else if (from instanceof HTMLOptionElement && to instanceof HTMLOptionElement) {
        if (from.hasAttribute('selected') === to.hasAttribute('selected')) same.add('selected')
      }
      if (same.size > 0) userState.set(from, same)
      else userState.delete(from)
      return true
    },
    beforeAttributeUpdated: (name, node) => !userState.get(node)?.has(name),
  },
}

// sheets holds one stylesheet object for each stylesheet URL, so each
// instance of a widget on the page shares it.
const sheets = new Map<string, Promise<CSSStyleSheet>>()

// loadSheet fetches the stylesheet of a widget and makes a constructable
// stylesheet of it (REQ-ISL-11). The rules live in the shadow root only: they
// do not reach the host page, and the rules of the host do not reach in.
const loadSheet = (url: string): Promise<CSSStyleSheet> => {
  let sheet = sheets.get(url)
  if (!sheet) {
    sheet = fetch(url, { credentials: 'omit' }).then(async (res) => {
      if (!res.ok) throw new Error('gx: the stylesheet of the widget did not load: ' + res.status)
      const css = new CSSStyleSheet()
      css.replaceSync(await res.text())
      return css
    })
    sheets.set(url, sheet)
    // A load that failed is tried again by the next widget.
    sheet.catch(() => sheets.delete(url))
  }
  return sheet
}

// mount puts the first render of a widget into its shadow root. The HTML
// comes from the Gx server of the widget, which escapes each value. server
// is the origin of that server.
export const mount = async (root: ShadowRoot, answer: Answer, server: string): Promise<Mounted> => {
  // The stylesheet is in place before the HTML, so the widget never shows
  // with no styles.
  root.adoptedStyleSheets = answer.style ? [await loadSheet(server + answer.style)] : []
  // One box holds the widget. It makes no box of its own in the layout.
  const box = document.createElement('div')
  box.setAttribute('data-gx-widget', answer.tag ?? '')
  box.style.display = 'contents'
  box.innerHTML = answer.html ?? ''
  root.replaceChildren(box)
  return {
    // update morphs the widget to a new render. Nodes that do not change
    // stay, with the value that the user typed and with the focus.
    update: (next: Answer): void => {
      Idiomorph.morph(box, next.html ?? '', morphOptions)
    },
    destroy: (): void => {
      box.remove()
      root.adoptedStyleSheets = []
    },
  }
}
