// The Gx overlay module (REQ-REG-07). It keeps anchored content inside the
// viewport, runs sub-menus, opens a context menu at the pointer and lets the
// pointer move between the menus of a menubar. The app injects it only on
// pages whose markup uses one of its markers, and `just check` fails when it
// grows over 2 KB gzipped.
//
// Markup contract:
//   [data-gx-place]  popover that the module places. The value is
//                    "side align gap shift": the preferred side, the
//                    alignment (start, center or end), the gap to the anchor
//                    and the shift along the anchor edge, both in px
//   [data-side]      written by the module: the side that the content took
//   [data-gx-sub]    wrapper of one sub-menu. Its first child is the trigger
//                    and its last child is the content, a nested popover
//   [data-gx-contextmenu] area whose right click opens the menu that its
//                    value selects
//
// The anchor of a sub-menu is its trigger. The anchor of other content is
// the element whose popovertarget names it, or the point of the right click
// that opened it. Content with no anchor keeps the place its styles give it.
// Content that does not fit on its side flips to the opposite side when that
// side has more room, then shifts along the anchor edge, and stays 8px
// inside the viewport.
//
// A sub-menu opens on ArrowRight, Enter, Space or a click on its trigger,
// and after the pointer rests 100 ms on it. ArrowLeft and Escape close it and
// return focus to the trigger. It stays open while the pointer moves from
// the trigger toward it.

type Popover = HTMLElement & { showPopover: (options?: { source?: HTMLElement }) => void; hidePopover: () => void }

const hide = (el: Element): void => {
  if (el.matches(':popover-open')) (el as Popover).hidePopover()
}

// subOf returns the sub-menu wrapper of its content.
const subOf = (el: Element): Element | null => {
  const sub = el.parentElement
  return sub?.hasAttribute('data-gx-sub') ? sub : null
}

// triggerSub returns the sub-menu wrapper when the element is in its trigger.
const triggerSub = (at: Element | null): Element | null => {
  const sub = at?.closest?.('[data-gx-sub]')
  return sub?.firstElementChild!.contains(at) ? sub : null
}

// openSub opens a sub-menu. The keyboard also enters its item list.
const openSub = (sub: Element, enter?: boolean): void => {
  const menu = sub.lastElementChild as Popover
  if (!menu.matches(':popover-open')) menu.showPopover()
  if (enter) menu.querySelector<HTMLElement>('[data-gx-roving-item]')?.focus()
}

// point is the place of the last right click that opened a context menu,
// and pointMenu is that menu.
let point = { left: 0, right: 0, top: 0, bottom: 0 }
let pointMenu: Element | null = null

// onContextMenu opens the menu of a right-clicked area at the pointer. The
// menu has no trigger to return to, so the keyboard starts inside it.
const onContextMenu = (e: MouseEvent): void => {
  const area = (e.target as Element | null)?.closest?.('[data-gx-contextmenu]')
  const menu = area && document.querySelector<Popover>(area.getAttribute('data-gx-contextmenu') || 'x')
  if (!menu) return
  e.preventDefault()
  point = { left: e.clientX, right: e.clientX, top: e.clientY, bottom: e.clientY }
  pointMenu = menu
  hide(menu)
  menu.showPopover()
  menu.querySelector<HTMLElement>('[data-gx-roving-item]')?.focus()
}

// place measures the content and its anchor, and writes the position and
// the side. CSS anchor positioning cannot do the flip: WebKit 26.0 hangs on
// position-try-fallbacks when the element has a transition.
const place = (el: HTMLElement): void => {
  const [side, align, gap, shift] = el.getAttribute('data-gx-place')!.split(' ')
  const anchor = subOf(el)?.firstElementChild ?? document.querySelector(`[popovertarget="${el.id}"]`)
  const a = anchor ? anchor.getBoundingClientRect() : el === pointMenu ? point : null
  if (!a) return
  const view = document.documentElement
  const vertical = side === 'top' || side === 'bottom'
  // Each list is the start and the end of the anchor, the size of the
  // content and the size of the viewport on one axis.
  const y = [a.top, a.bottom, el.offsetHeight, view.clientHeight]
  const x = [a.left, a.right, el.offsetWidth, view.clientWidth]
  const [start, end, size, room] = vertical ? y : x
  const [crossStart, crossEnd, crossSize, crossRoom] = vertical ? x : y
  const before = start - (+gap || 0) - 8
  const after = room - end - (+gap || 0) - 8
  let first = side === 'top' || side === 'left'
  if (size > (first ? before : after) && (first ? after > before : before > after)) first = !first
  // Content that fits on no side stays inside the viewport and covers a
  // part of its anchor.
  const main = Math.round(Math.max(8, Math.min(first ? before + 8 - size : room - after - 8, room - size - 8)))
  const want =
    align === 'start' ? crossStart + (+shift || 0)
    : align === 'end' ? crossEnd - crossSize - (+shift || 0)
    : (crossStart + crossEnd - crossSize) / 2
  // Whole pixels keep the text of the content sharp.
  const cross = Math.round(Math.max(8, Math.min(want, crossRoom - crossSize - 8)))
  el.setAttribute('data-side', vertical ? (first ? 'top' : 'bottom') : first ? 'left' : 'right')
  el.style.inset = vertical ? `${main}px auto auto ${cross}px` : `${cross}px auto auto ${main}px`
  el.style.margin = '0'
  el.style.justifySelf = 'auto'
}

// onBeforeToggle places content before it shows, so the enter motion starts
// at the final place and slides from the side that the content took. The
// closed content has no box, so it gets one for the measurement, with no
// transition: the next style of the open content is then its starting style.
const onBeforeToggle = (e: Event): void => {
  const el = e.target as HTMLElement
  if (!el.hasAttribute?.('data-gx-place')) return
  const opens = (e as Event & { newState: string }).newState === 'open'
  const trigger = subOf(el)?.firstElementChild as HTMLElement | null
  if (trigger) {
    trigger.setAttribute('aria-expanded', `${opens}`)
    if (!opens && el.contains(document.activeElement)) trigger.focus()
  }
  if (!opens) return
  el.style.transition = 'none'
  el.style.display = 'block'
  place(el)
  el.style.display = ''
  void el.offsetWidth
  el.style.transition = ''
}

// onKeydown listens on the window, so the keys of a sub-menu do not reach
// the roving and dismiss handlers of the behaviour runtime.
const onKeydown = (e: KeyboardEvent): void => {
  const at = e.target as Element
  const own = at.closest?.('[data-gx-sub]')
  if (!own) return
  const onTrigger = own.firstElementChild === at
  if (onTrigger && e.key === 'ArrowRight') openSub(own, true)
  else {
    // The focus is in the content of this sub-menu.
    const sub = onTrigger ? own.parentElement!.closest('[data-gx-sub]') : own
    if (!sub || (e.key !== 'ArrowLeft' && e.key !== 'Escape')) return
    hide(sub.lastElementChild!)
  }
  e.preventDefault()
  e.stopPropagation()
}

// hover is the sub-menu whose trigger is under the pointer, and apex is the
// last place of the pointer on a trigger.
let hover: Element | null = null
let apex = [0, 0]
let openTimer = 0
let closeTimer = 0

const onPointermove = (e: PointerEvent): void => {
  const at = e.target as Element
  // With one menu of a menubar open, the pointer opens the menu of the
  // trigger under it.
  const next = at.closest?.('[role=menubar]>[popovertarget]') as HTMLElement | null
  const shown = next?.parentElement!.querySelector('[role=menu]:popover-open')
  const menu = next && (document.getElementById(next.getAttribute('popovertarget')!) as Popover | null)
  if (shown && menu && shown !== menu) {
    hide(shown)
    next!.focus()
    menu.showPopover({ source: next! })
  }
  const over = triggerSub(at)
  if (over !== hover) {
    window.clearTimeout(openTimer)
    hover = over
    if (over) openTimer = window.setTimeout(() => openSub(over), 100)
  }
  if (over) apex = [e.clientX, e.clientY]
  // The pointer is on another item of a menu with an open sub-menu. The
  // sub-menu stays while the pointer is in the triangle from the apex to
  // the near edge of the sub-menu, and closes when it rests there.
  window.clearTimeout(closeTimer)
  const parent = at.closest?.('[role=menu]')
  parent?.querySelectorAll('[data-gx-sub]>:popover-open').forEach((el) => {
    const sub = el.parentElement!
    if (sub.closest('[role=menu]') !== parent || sub.contains(at)) return
    const r = el.getBoundingClientRect()
    const edge = el.getAttribute('data-side') === 'left' ? r.right : r.left
    const t = (e.clientX - apex[0]) / (edge - apex[0])
    const inside =
      t >= 0 && t <= 1 &&
      e.clientY >= apex[1] + t * (r.top - apex[1]) - 4 &&
      e.clientY <= apex[1] + t * (r.bottom - apex[1]) + 4
    if (inside) closeTimer = window.setTimeout(() => hide(el), 300)
    else hide(el)
  })
}

if (typeof document !== 'undefined') {
  const replace = (): void => document.querySelectorAll<HTMLElement>('[data-gx-place]:popover-open').forEach(place)
  window.addEventListener('resize', replace)
  document.addEventListener('scroll', replace, true)
  // A sub-menu that opens while its menu still zooms in measures a scaled
  // trigger; it takes its final place when the transition ends.
  document.addEventListener('transitionend', replace, true)
  document.addEventListener('beforetoggle', onBeforeToggle, true)
  document.addEventListener('contextmenu', onContextMenu)
  window.addEventListener('keydown', onKeydown, true)
  document.addEventListener('pointermove', onPointermove)
  document.addEventListener('click', (e) => {
    const sub = triggerSub(e.target as Element | null)
    if (sub) openSub(sub, !e.detail)
  })
}
