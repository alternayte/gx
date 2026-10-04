// The Gx component behaviour runtime (REQ-REG-07). It covers roving
// tabindex, typeahead, focus trap, dismiss on outside click and Escape. The
// app injects it only on pages whose markup uses one of its markers, and
// `just check` fails when it grows over 4 KB gzipped.
//
// Markup contract:
//   [data-gx-roving]      container with roving tabindex
//   [data-gx-roving-item] one focusable item of the container
//   [data-gx-trap]        element that traps Tab while it holds focus
//   [data-gx-dismiss]     overlay that closes on Escape or outside click
//   [data-gx-open]        set by a trigger; removed on dismiss
//
// A roving container follows the horizontal or vertical arrow keys, Home,
// End and typeahead on printable keys. `data-gx-roving="nowrap"` stops at
// the ends. A dismiss target closes natively when it is a dialog or a
// popover, and loses `data-gx-open` otherwise; it also emits `gx:dismiss`.

const focusable = (root: HTMLElement): HTMLElement[] =>
  [...root.querySelectorAll<HTMLElement>(
    'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])',
  )].filter((el) => el.offsetParent !== null || el === document.activeElement)

const isOpen = (el: HTMLElement): boolean =>
  el.matches(':popover-open') || (el instanceof HTMLDialogElement && el.open) || el.hasAttribute('data-gx-open')

const close = (el: HTMLElement): void => {
  if (el instanceof HTMLDialogElement && el.open) {
    el.close()
    return
  }
  if (el.matches(':popover-open')) {
    ;(el as HTMLElement & { hidePopover: () => void }).hidePopover()
    return
  }
  el.removeAttribute('data-gx-open')
  el.dispatchEvent(new CustomEvent('gx:dismiss', { bubbles: true }))
}

const rovingItems = (box: HTMLElement): HTMLElement[] =>
  [...box.querySelectorAll<HTMLElement>('[data-gx-roving-item]')]

const rovingAt = (items: HTMLElement[]): number => {
  const index = items.findIndex(
    (item) => item.getAttribute('aria-selected') === 'true' || item.hasAttribute('data-selected') || item.hasAttribute('data-active'),
  )
  return index < 0 ? 0 : index
}

const setCurrent = (items: HTMLElement[], at: number, wrap: boolean): void => {
  if (items.length === 0) return
  let index = at
  if (wrap) index = (index + items.length) % items.length
  else index = Math.max(0, Math.min(index, items.length - 1))
  items.forEach((item, i) => item.setAttribute('tabindex', i === index ? '0' : '-1'))
  const item = items[index]
  item.focus()
  // Selection follows focus for the ARIA tab and listbox patterns.
  const role = item.getAttribute('role')
  if (role === 'tab' || role === 'option') item.click()
}

const installRoving = (): void => {
  document.querySelectorAll<HTMLElement>('[data-gx-roving]').forEach((box) => {
    if (box.hasAttribute('data-gx-roving-ready')) return
    box.setAttribute('data-gx-roving-ready', '')
    const items = rovingItems(box)
    const current = rovingAt(items)
    items.forEach((item, i) => {
      if (!item.hasAttribute('tabindex')) item.setAttribute('tabindex', i === current ? '0' : '-1')
    })
  })
}

// typeahead accumulates printable keys per container for half a second.
const typed = new WeakMap<HTMLElement, { text: string; timer: number }>()

const typeahead = (box: HTMLElement, items: HTMLElement[], key: string): void => {
  const state = typed.get(box) ?? { text: '', timer: 0 }
  window.clearTimeout(state.timer)
  state.text += key.toLowerCase()
  state.timer = window.setTimeout(() => typed.delete(box), 500)
  typed.set(box, state)
  const at = items.findIndex((item) => (item.textContent ?? '').trim().toLowerCase().startsWith(state.text))
  if (at >= 0) setCurrent(items, at, false)
}

const onKeydown = (e: KeyboardEvent): void => {
  const at = e.target as HTMLElement | null
  if (e.key === 'Escape' && !e.defaultPrevented) {
    const open = [...document.querySelectorAll<HTMLElement>('[data-gx-dismiss]')].filter(isOpen)
    if (open.length > 0) {
      close(open[open.length - 1])
      e.stopPropagation()
      return
    }
  }
  if (e.key === 'Tab' && !e.defaultPrevented) {
    const trap = at?.closest?.('[data-gx-trap]') as HTMLElement | null
    if (trap) {
      const nodes = focusable(trap)
      const first = nodes[0]
      const last = nodes[nodes.length - 1]
      if (first && last) {
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }
    return
  }
  const box = at?.closest?.('[data-gx-roving]') as HTMLElement | null
  if (!box) return
  const items = rovingItems(box)
  if (items.length === 0) return
  const wrap = box.getAttribute('data-gx-roving') !== 'nowrap'
  const current = Math.max(0, items.indexOf(at as HTMLElement))
  switch (e.key) {
    case 'ArrowDown':
    case 'ArrowRight':
      e.preventDefault()
      setCurrent(items, current + 1, wrap)
    case 'ArrowUp':
    case 'ArrowLeft':
      e.preventDefault()
      setCurrent(items, current - 1, wrap)
    case 'Home':
      e.preventDefault()
      setCurrent(items, 0, false)
    case 'End':
      e.preventDefault()
      setCurrent(items, items.length - 1, false)
    default:
      if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault()
        typeahead(box, items, e.key)
      }
  }
}

const onPointerdown = (e: Event): void => {
  const target = e.target as Node | null
  document.querySelectorAll<HTMLElement>('[data-gx-dismiss]').forEach((el) => {
    if (!isOpen(el) || el.contains(target)) return
    close(el)
  })
}

const onToggle = (e: Event): void => {
  const el = e.target as HTMLElement | null
  if (!(el instanceof HTMLDialogElement) || !el.open || !el.hasAttribute('data-gx-trap')) return
  const nodes = focusable(el)
  ;(nodes[0] ?? el).focus()
}

const install = (): void => {
  installRoving()
}

if (typeof document !== 'undefined') {
  document.addEventListener('keydown', onKeydown, true)
  document.addEventListener('pointerdown', onPointerdown, true)
  document.addEventListener('toggle', onToggle, true)
  install()
  new MutationObserver(install).observe(document.documentElement, { subtree: true, childList: true })
}
