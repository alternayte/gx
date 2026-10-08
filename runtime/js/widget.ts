// The widget script of a Gx server (REQ-ISL-19). The element file of a host
// page imports it from the server, so it is always the script of the build
// that rendered the HTML. The server puts Idiomorph before this code in the
// same file.

type Answer = {
  tag?: string
  html?: string
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

// mount puts the first render of a widget into its shadow root. The HTML
// comes from the Gx server of the widget, which escapes each value.
export const mount = (root: ShadowRoot, answer: Answer): Mounted => {
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
    },
  }
}
