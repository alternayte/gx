// The Gx component behaviour runtime (REQ-REG-07). It covers roving
// tabindex, typeahead, focus trap, dismiss on outside click and Escape, and
// open and close. Tabs, toasts, and placed content with sub-menus and the
// context menu have their own modules: tabs.ts, toast.ts and overlay.ts. The
// app injects each module only on pages whose markup uses one of its
// markers, and `just check` fails when a module grows over 2 KB gzipped.
//
// Markup contract:
//   [data-gx-roving]      container with roving tabindex
//   [data-gx-roving-item] one focusable item of the container
//   [data-gx-trap]        element that traps Tab while it holds focus
//   [data-gx-dismiss]     overlay that closes on Escape or outside click
//   [data-gx-open]        on a trigger, selects the overlay to open; on a
//                         custom overlay, marks it open until dismiss
//   [data-gx-close]       closes the nearest overlay
//
// A roving container follows the horizontal or vertical arrow keys, Home,
// End and typeahead on printable keys. `data-gx-roving="nowrap"` stops at
// the ends. A dismiss target closes natively when it is a dialog or a
// popover, and loses `data-gx-open` otherwise; it also emits `gx:dismiss`.
//
// install runs the module for one root: the document of a page, or the
// shadow root of a widget (REQ-ISL-20). Each lookup and each listener is on
// the root, so two roots on one page do not see the elements of each other.
export const install = (r: Document | ShadowRoot): void => {
  const focusable = (root: HTMLElement): HTMLElement[] =>
    [...root.querySelectorAll<HTMLElement>(
      'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])',
    )].filter((el) => el.offsetParent !== null || el === r.activeElement)

  const isOpen = (el: HTMLElement): boolean =>
    el.matches(':popover-open') || (el instanceof HTMLDialogElement && el.open) || el.hasAttribute('data-gx-open')

  // open opens a dialog, a popover or a custom overlay. A popover keeps its
  // invoker as the source, so a click on the invoker closes it again.
  const open = (el: HTMLElement, source?: HTMLElement): void => {
    if (el instanceof HTMLDialogElement) {
      el.showModal()
      return
    }
    const popover = el as HTMLElement & { showPopover?: (options?: { source?: HTMLElement }) => void }
    if (typeof popover.showPopover === 'function' && el.hasAttribute('popover')) {
      popover.showPopover({ source })
      return
    }
    el.setAttribute('data-gx-open', '')
    el.dispatchEvent(new CustomEvent('gx:open', { bubbles: true }))
  }

  const manual = (el: HTMLElement): boolean => el.getAttribute('data-gx-dismiss') === 'manual'

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

  // rovingItems leaves out the items of a nested container, such as a menu
  // inside a menubar.
  const rovingItems = (box: HTMLElement): HTMLElement[] =>
    [...box.querySelectorAll<HTMLElement>('[data-gx-roving-item]')].filter(
      (item) => item.closest('[data-gx-roving]') === box,
    )

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
    r.querySelectorAll<HTMLElement>('[data-gx-roving]').forEach((box) => {
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
    // A checkbox or radio item takes its text from its label.
    const at = items.findIndex((item) =>
      ((item.closest('label') ?? item).textContent ?? '').trim().toLowerCase().startsWith(state.text),
    )
    if (at >= 0) setCurrent(items, at, false)
  }

  const menuOf = (trigger: HTMLElement): HTMLElement | null =>
    r.getElementById(trigger.getAttribute('popovertarget') ?? '')

  // enterMenu opens the menu of a trigger and focuses its first item.
  const enterMenu = (trigger: HTMLElement): boolean => {
    const menu = menuOf(trigger)
    if (!menu) return false
    if (!isOpen(menu)) open(menu, trigger)
    const items = rovingItems(menu)
    if (items.length > 0) setCurrent(items, 0, false)
    return true
  }

  const onKeydown = (e: KeyboardEvent): void => {
    const at = e.target as HTMLElement | null
    if (e.key === 'Escape' && !e.defaultPrevented) {
      const openList = [...r.querySelectorAll<HTMLElement>('[data-gx-dismiss]')].filter(isOpen)
      if (openList.length > 0) {
        const top = openList[openList.length - 1]
        if (!manual(top)) {
          close(top)
          e.stopPropagation()
        }
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
          if (e.shiftKey && r.activeElement === first) {
            e.preventDefault()
            last.focus()
          } else if (!e.shiftKey && r.activeElement === last) {
            e.preventDefault()
            first.focus()
          }
        }
      }
      return
    }
    // ArrowDown on a menu trigger opens the menu and enters the item list.
    if (e.key === 'ArrowDown' && at?.hasAttribute?.('popovertarget')) {
      if (enterMenu(at)) e.preventDefault()
      return
    }
    const box = at?.closest?.('[data-gx-roving]') as HTMLElement | null
    if (!box) return
    const items = rovingItems(box)
    if (items.length === 0) return
    const wrap = box.getAttribute('data-gx-roving') !== 'nowrap'
    const current = Math.max(0, items.indexOf(at as HTMLElement))
    // ArrowLeft and ArrowRight in a menu of a menubar move to the next menu.
    const side = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
    const owner = side !== 0 && box.id ? r.querySelector<HTMLElement>(`[role=menubar] [popovertarget="${box.id}"]`) : null
    const bar = owner?.closest('[data-gx-roving]') as HTMLElement | null
    if (owner && bar) {
      e.preventDefault()
      const triggers = rovingItems(bar)
      const next = triggers[(triggers.indexOf(owner) + side + triggers.length) % triggers.length]
      close(box)
      setCurrent(triggers, triggers.indexOf(next), false)
      enterMenu(next)
      return
    }
    switch (e.key) {
      case 'ArrowDown':
      case 'ArrowRight':
        e.preventDefault()
        setCurrent(items, current + 1, wrap)
        break
      case 'ArrowUp':
      case 'ArrowLeft':
        e.preventDefault()
        setCurrent(items, current - 1, wrap)
        break
      case 'Home':
        e.preventDefault()
        setCurrent(items, 0, false)
        break
      case 'End':
        e.preventDefault()
        setCurrent(items, items.length - 1, false)
        break
      case 'Enter':
        // Enter does not toggle a native checkbox or radio item.
        if (at instanceof HTMLInputElement) {
          e.preventDefault()
          at.click()
        }
        break
      default:
        // Space stays with the item: it activates a button or toggles an input.
        if (e.key.length === 1 && e.key !== ' ' && !e.ctrlKey && !e.metaKey && !e.altKey) {
          e.preventDefault()
          typeahead(box, items, e.key)
        }
    }
  }

  const onPointerdown = (e: Event): void => {
    const target = e.target as Element | null
    r.querySelectorAll<HTMLElement>('[data-gx-dismiss]').forEach((el) => {
      if (!isOpen(el) || manual(el) || el.contains(target)) return
      // The click on the invoker of a popover toggles it; closing it here
      // would let that click open it again.
      if (el.id && target?.closest?.(`[popovertarget="${el.id}"]`)) return
      close(el)
    })
  }

  // onOpenClick wires the generic overlay contract: data-gx-open points at
  // the element to open, data-gx-close closes the nearest overlay.
  const onOpenClick = (e: Event): void => {
    const at = e.target as Element | null
    const closeButton = at?.closest?.('[data-gx-close]') as HTMLElement | null
    if (closeButton) {
      const target =
        (closeButton.closest('[data-gx-dismiss]') as HTMLElement | null) ??
        (closeButton.closest('dialog[open], [popover]:popover-open') as HTMLElement | null)
      if (target) {
        e.preventDefault()
        close(target)
      }
      return
    }
    const opener = at?.closest?.('[data-gx-open]') as HTMLElement | null
    if (!opener) return
    const selector = opener.getAttribute('data-gx-open') ?? ''
    if (selector === '') return
    const target = r.querySelector<HTMLElement>(selector)
    if (!target) return
    e.preventDefault()
    open(target)
  }

  // onCancel keeps a manual dialog open when the browser fires cancel on
  // Escape (REQ-REG-07).
  const onCancel = (e: Event): void => {
    const dialog = e.target as HTMLElement | null
    if (dialog?.getAttribute?.('data-gx-dismiss') === 'manual') e.preventDefault()
  }

  const onToggle = (e: Event): void => {
    const el = e.target as HTMLElement | null
    if (!(el instanceof HTMLDialogElement) || !el.open || !el.hasAttribute('data-gx-trap')) return
    const nodes = focusable(el)
    ;(nodes[0] ?? el).focus()
  }

  {
    r.addEventListener('click', onOpenClick)
    // A range input with data-gx-behavior paints its range up to --gx-fill.
    r.addEventListener('input', (e) => {
      const el = e.target as HTMLInputElement | null
      if (!el?.matches?.('input[type=range][data-gx-behavior]')) return
      const min = +el.min
      el.style.setProperty('--gx-fill', `${((+el.value - min) / (+el.max - min || 1)) * 100}%`)
    })
    r.addEventListener('keydown', onKeydown as EventListener, true)
    r.addEventListener('pointerdown', onPointerdown, true)
    r.addEventListener('toggle', onToggle, true)
    r.addEventListener('cancel', onCancel, true)
    installRoving()
    new MutationObserver(installRoving).observe(r, { subtree: true, childList: true })
  }
}
