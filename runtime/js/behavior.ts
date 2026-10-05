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

// open opens a dialog, a popover or a custom overlay.
const open = (el: HTMLElement): void => {
  if (el instanceof HTMLDialogElement) {
    el.showModal()
    return
  }
  const popover = el as HTMLElement & { showPopover?: () => void }
  if (typeof popover.showPopover === 'function' && el.hasAttribute('popover')) {
    popover.showPopover()
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

// A docs tab holds its panel in its [data-gx-tab-item]. A registry tab sits
// in a tab list and names its panel: [data-gx-tab-panel] carries the label.
const tabsOf = (wrapper: Element): Tab[] => {
  const tabs: Tab[] = []
  wrapper.querySelectorAll<HTMLElement>('[data-gx-tab]').forEach((button) => {
    if (button.closest('[data-gx-tabs]') !== wrapper) return
    const label = button.getAttribute('data-gx-tab') ?? ''
    const panel =
      button.closest('[data-gx-tab-item]')?.querySelector<HTMLElement>('[data-gx-tab-panel]') ??
      [...wrapper.querySelectorAll<HTMLElement>('[data-gx-tab-panel]')].find(
        (el) => el.getAttribute('data-gx-tab-panel') === label && el.closest('[data-gx-tabs]') === wrapper,
      )
    if (panel) tabs.push({ button, panel, label })
  })
  return tabs
}

const applyTab = (wrapper: Element, label: string): void => {
  const tabs = tabsOf(wrapper)
  if (tabs.length === 0) return
  const chosen = tabs.find((tab) => tab.label === label) ?? tabs[0]
  for (const tab of tabs) {
    const selected = tab === chosen
    // A button with the tab role states aria-selected; a plain button
    // states aria-expanded.
    const state = tab.button.getAttribute('role') === 'tab' ? 'aria-selected' : 'aria-expanded'
    tab.button.setAttribute(state, selected ? 'true' : 'false')
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
    const orientation = wrapper.getAttribute('data-orientation')
    if (orientation) wrapper.querySelector('[role=tablist]')?.setAttribute('aria-orientation', orientation)
    const sync = wrapper.getAttribute('data-sync') ?? ''
    const stored = sync !== '' ? tabsStorage('gx-tabs:' + sync) : ''
    const initial = stored !== '' ? stored : wrapper.getAttribute('data-default') ?? ''
    selectTab(wrapper, initial, false)
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
    const menu = document.getElementById(at.getAttribute('popovertarget') ?? '')
    if (menu) {
      e.preventDefault()
      if (!isOpen(menu)) open(menu)
      const items = rovingItems(menu)
      if (items.length > 0) setCurrent(items, 0, false)
    }
    return
  }
  // A tab list follows the arrow keys of its orientation, Home and End, and
  // skips a disabled tab.
  const tab = at?.closest?.('[data-gx-tab]') as HTMLElement | null
  const wrapper = tab?.closest('[data-gx-tabs]')
  if (tab && wrapper) {
    const vertical = wrapper.getAttribute('data-orientation') === 'vertical'
    const tabs = tabsOf(wrapper).filter((entry) => !entry.button.hasAttribute('disabled'))
    const i = tabs.findIndex((entry) => entry.button === tab)
    const next =
      e.key === (vertical ? 'ArrowDown' : 'ArrowRight') ? i + 1
      : e.key === (vertical ? 'ArrowUp' : 'ArrowLeft') ? i - 1
      : e.key === 'Home' ? 0
      : e.key === 'End' ? -1
      : NaN
    if (!isNaN(next)) {
      e.preventDefault()
      const to = tabs[(next + tabs.length) % tabs.length]
      to.button.focus()
      selectTab(wrapper, to.label, true)
      return
    }
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
    if (!isOpen(el) || manual(el) || el.contains(target)) return
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
  // A range input with data-gx-behavior paints its range up to --gx-fill.
  document.addEventListener('input', (e) => {
    const el = e.target as HTMLInputElement | null
    if (!el?.matches?.('input[type=range][data-gx-behavior]')) return
    const min = +el.min
    el.style.setProperty('--gx-fill', `${((+el.value - min) / (+el.max - min || 1)) * 100}%`)
  })
  document.addEventListener('keydown', onKeydown, true)
  document.addEventListener('pointerdown', onPointerdown, true)
  document.addEventListener('toggle', onToggle, true)
  document.addEventListener('cancel', onCancel, true)
  install()
  new MutationObserver(install).observe(document.documentElement, { subtree: true, childList: true })
}
