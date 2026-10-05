// The Gx component behaviour runtime (REQ-REG-07). It covers roving
// tabindex, typeahead, focus trap, dismiss on outside click and Escape, and
// the toaster (REQ-REG-11). The app injects it only on pages whose markup
// uses one of its markers, and `just check` fails when it grows over 4 KB
// gzipped.
//
// Markup contract:
//   [data-gx-roving]      container with roving tabindex
//   [data-gx-roving-item] one focusable item of the container
//   [data-gx-trap]        element that traps Tab while it holds focus
//   [data-gx-dismiss]     overlay that closes on Escape or outside click
//   [data-gx-open]        set by a trigger; removed on dismiss
//   [data-gx-toaster]     region that holds the pushed toasts
//   [data-gx-toast]       one toast; data-duration is its time in ms, 0 stays
//   [data-closing]        set on a toast that leaves; CSS runs the exit
//
// A toast leaves after its duration, on its [data-gx-close] button or on
// Escape. The timers of a toaster stop while the pointer is over it or the
// focus is inside it. A new toast takes the place of an earlier toast with
// the same id, and a toaster shows at most three toasts.
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

// Tabs are exclusive panels with an optional sync key (REQ-CNT-05). The
// registry tabs item and the docs kit share this contract.
const tabsStorage = (key: string): string => {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

const tabsRemember = (key: string, label: string): void => {
  try {
    localStorage.setItem(key, label)
  } catch {
    // Private mode has no storage; the page selection still works.
  }
}

type Tab = { button: HTMLElement; panel: HTMLElement; label: string }

const tabsOf = (wrapper: Element): Tab[] => {
  const tabs: Tab[] = []
  wrapper.querySelectorAll<HTMLElement>('[data-gx-tab]').forEach((button) => {
    const panel = button.closest('[data-gx-tab-item]')?.querySelector<HTMLElement>('[data-gx-tab-panel]')
    if (panel) tabs.push({ button, panel, label: button.getAttribute('data-gx-tab') ?? '' })
  })
  return tabs
}

const applyTab = (wrapper: Element, label: string): void => {
  const tabs = tabsOf(wrapper)
  if (tabs.length === 0) return
  const chosen = tabs.find((tab) => tab.label === label) ?? tabs[0]
  for (const tab of tabs) {
    const selected = tab === chosen
    tab.button.setAttribute('aria-expanded', selected ? 'true' : 'false')
    tab.button.setAttribute('tabindex', selected ? '0' : '-1')
    if (selected) tab.button.setAttribute('data-selected', 'true')
    else tab.button.removeAttribute('data-selected')
    tab.panel.hidden = !selected
  }
}

const selectTab = (wrapper: Element, label: string, remember: boolean): void => {
  const tabs = tabsOf(wrapper)
  if (tabs.length === 0) return
  const chosen = tabs.find((tab) => tab.label === label) ?? tabs[0]
  applyTab(wrapper, chosen.label)
  const sync = wrapper.getAttribute('data-sync') ?? ''
  if (sync !== '') {
    document.querySelectorAll<HTMLElement>('[data-gx-tabs]').forEach((other) => {
      if (other !== wrapper && other.getAttribute('data-sync') === sync) {
        const match = tabsOf(other).find((tab) => tab.label === chosen.label)
        if (match) applyTab(other, match.label)
      }
    })
    if (remember) tabsRemember('gx-tabs:' + sync, chosen.label)
  }
}

const installTabs = (): void => {
  document.querySelectorAll<HTMLElement>('[data-gx-tabs]').forEach((wrapper) => {
    if (wrapper.getAttribute('data-gx-tabs-ready') === 'true') return
    wrapper.setAttribute('data-gx-tabs-ready', 'true')
    const tabs = tabsOf(wrapper)
    tabs.forEach((tab, i) => {
      if (!tab.button.id) tab.button.id = `gx-tab-${i}-${Math.random().toString(36).slice(2, 8)}`
      if (!tab.panel.id) tab.panel.id = `${tab.button.id}-panel`
      tab.button.setAttribute('aria-controls', tab.panel.id)
      tab.panel.setAttribute('role', 'tabpanel')
      tab.panel.setAttribute('aria-labelledby', tab.button.id)
    })
    const sync = wrapper.getAttribute('data-sync') ?? ''
    const stored = sync !== '' ? tabsStorage('gx-tabs:' + sync) : ''
    const initial = stored !== '' ? stored : wrapper.getAttribute('data-default') ?? ''
    selectTab(wrapper, initial, false)
  })
}

// Toasts arrive by DOM patch (REQ-REG-11). left is the time a toast still
// has, and at is the start of its running timer, or 0 while it is paused.
type ToastState = { left: number; at: number; timer: number }

const toastStates = new WeakMap<HTMLElement, ToastState>()

// toastReturn is the last focused element outside a toaster. Focus goes
// back to it when the focused toast leaves.
let toastReturn: HTMLElement | null = null

// toastHover is the toaster under the pointer.
let toastHover: Element | null = null

const toastsIn = (region: Element): HTMLElement[] =>
  [...region.querySelectorAll<HTMLElement>('[data-gx-toast]:not([data-closing])')]

// dismissToast starts the exit of a toast and removes it when the exit
// transition ends. The timeout covers a toast with no transition.
const dismissToast = (el: HTMLElement): void => {
  if (el.hasAttribute('data-closing')) return
  window.clearTimeout(toastStates.get(el)?.timer)
  const focused = el.contains(document.activeElement)
  el.setAttribute('data-closing', '')
  const remove = (): void => el.remove()
  el.addEventListener('transitionend', (e) => {
    if (e.target === el) remove()
  })
  const style = getComputedStyle(el)
  const exit = style.transitionProperty === 'none' ? 0 : parseFloat(style.transitionDuration) || 0
  window.setTimeout(remove, exit * 1000 + 50)
  if (focused && toastReturn?.isConnected) toastReturn.focus()
}

// syncToasts runs or pauses the timers of every toaster.
const syncToasts = (): void => {
  document.querySelectorAll('[data-gx-toaster]').forEach((region) => {
    const paused = region === toastHover || region.contains(document.activeElement)
    for (const el of toastsIn(region)) {
      const state = toastStates.get(el)
      if (!state || !(state.left > 0) || paused === !state.at) continue
      if (paused) {
        window.clearTimeout(state.timer)
        state.left = Math.max(1, state.left - (Date.now() - state.at))
        state.at = 0
      } else {
        state.at = Date.now()
        state.timer = window.setTimeout(() => dismissToast(el), state.left)
      }
    }
  })
}

const installToasts = (): void => {
  document.querySelectorAll('[data-gx-toaster]').forEach((region) => {
    region.querySelectorAll<HTMLElement>('[data-gx-toast]').forEach((el) => {
      if (toastStates.has(el)) return
      toastStates.set(el, { left: +(el.getAttribute('data-duration') ?? 0), at: 0, timer: 0 })
      const old = el.id ? region.querySelector<HTMLElement>('#' + CSS.escape(el.id)) : null
      if (!old || old === el) return
      // The new toast takes the place of the earlier one with its id. It
      // skips the enter transition, so the swap does not move.
      const focused = old.contains(document.activeElement)
      window.clearTimeout(toastStates.get(old)?.timer)
      old.replaceWith(el)
      el.style.transition = 'none'
      void el.offsetWidth
      el.style.transition = ''
      if (focused) el.querySelector<HTMLElement>('a,button')?.focus()
    })
    toastsIn(region).slice(0, -3).forEach(dismissToast)
  })
  syncToasts()
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
  document.getElementById(trigger.getAttribute('popovertarget') ?? '')

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
    const toast = at?.closest?.('[data-gx-toaster] [data-gx-toast]') as HTMLElement | null
    if (toast) {
      dismissToast(toast)
      e.stopPropagation()
      return
    }
    const openList = [...document.querySelectorAll<HTMLElement>('[data-gx-dismiss]')].filter(isOpen)
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
  // ArrowDown on a menu trigger opens the menu and enters the item list.
  if (e.key === 'ArrowDown' && at?.hasAttribute?.('popovertarget')) {
    if (enterMenu(at)) e.preventDefault()
    return
  }
  const tab = at?.closest?.('[data-gx-tab]') as HTMLElement | null
  if (tab && (e.key === 'ArrowRight' || e.key === 'ArrowLeft')) {
    const wrapper = tab.closest('[data-gx-tabs]')
    if (wrapper) {
      e.preventDefault()
      const tabs = tabsOf(wrapper)
      const i = tabs.findIndex((entry) => entry.button === tab)
      const next = (i + (e.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
      tabs[next].button.focus()
      selectTab(wrapper, tabs[next].label, true)
    }
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
  const owner = side !== 0 && box.id ? document.querySelector<HTMLElement>(`[role=menubar] [popovertarget="${box.id}"]`) : null
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
  document.querySelectorAll<HTMLElement>('[data-gx-dismiss]').forEach((el) => {
    if (!isOpen(el) || manual(el) || el.contains(target)) return
    // The click on the invoker of a popover toggles it; closing it here
    // would let that click open it again.
    if (el.id && target?.closest?.(`[popovertarget="${el.id}"]`)) return
    close(el)
  })
}

// onContextMenu opens the menu of a right-clicked area.
const onContextMenu = (e: Event): void => {
  const trigger = (e.target as Element | null)?.closest?.('[data-gx-contextmenu]') as HTMLElement | null
  if (!trigger) return
  const selector = trigger.getAttribute('data-gx-contextmenu') ?? ''
  const target = selector === '' ? null : document.querySelector<HTMLElement>(selector)
  if (!target) return
  e.preventDefault()
  const point = e as MouseEvent
  target.style.left = `${point.clientX}px`
  target.style.top = `${point.clientY}px`
  open(target)
  // The menu has no trigger to return to, so the keyboard starts inside it.
  const items = rovingItems(target)
  if (items.length > 0) setCurrent(items, 0, false)
}

// onOpenClick wires the generic overlay contract: data-gx-open points at
// the element to open, data-gx-close closes the nearest overlay.
const onOpenClick = (e: Event): void => {
  const at = e.target as Element | null
  const closeButton = at?.closest?.('[data-gx-close]') as HTMLElement | null
  if (closeButton) {
    const toast = closeButton.closest('[data-gx-toaster] [data-gx-toast]') as HTMLElement | null
    if (toast) {
      dismissToast(toast)
      return
    }
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
  const target = document.querySelector<HTMLElement>(selector)
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

const install = (): void => {
  installRoving()
  installTabs()
  installToasts()
}

if (typeof document !== 'undefined') {
  document.addEventListener('click', onOpenClick)
  document.addEventListener('contextmenu', onContextMenu)
  document.addEventListener('click', (e) => {
    const button = (e.target as Element | null)?.closest?.('[data-gx-tab]') as HTMLElement | null
    if (!button) return
    const wrapper = button.closest('[data-gx-tabs]')
    if (!wrapper) return
    e.preventDefault()
    selectTab(wrapper, button.getAttribute('data-gx-tab') ?? '', true)
  })
  document.addEventListener('keydown', onKeydown, true)
  document.addEventListener('pointerdown', onPointerdown, true)
  document.addEventListener('toggle', onToggle, true)
  document.addEventListener('cancel', onCancel, true)
  for (const type of ['pointerover', 'pointerout', 'focusin', 'focusout']) {
    document.addEventListener(type, (e) => {
      const at = e.target as HTMLElement | null
      const region = at?.closest?.('[data-gx-toaster]') ?? null
      if (type === 'focusin' && !region) toastReturn = at
      if (type === 'pointerover') toastHover = region
      // The pointer left the window.
      if (type === 'pointerout' && !(e as PointerEvent).relatedTarget) toastHover = null
      // The focus settles after the event.
      window.setTimeout(syncToasts)
    })
  }
  install()
  new MutationObserver(install).observe(document.documentElement, { subtree: true, childList: true })
}
