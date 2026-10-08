// The Gx toast module (REQ-REG-11). The app injects it only on pages that
// hold the toaster region, and `just check` fails when it grows over 2 KB
// gzipped.
//
// Markup contract:
//   [data-gx-toaster]     region that holds the pushed toasts
//   [data-gx-toast]       one toast; data-duration is its time in ms, 0 stays
//   [data-gx-close]       button of a toast that closes it
//   [data-closing]        set on a toast that leaves; CSS runs the exit
//
// A toast leaves after its duration, on its [data-gx-close] button or on
// Escape. The timers of a toaster stop while the pointer is over it or the
// focus is inside it. A new toast takes the place of an earlier toast with
// the same id, and a toaster shows at most three toasts.
//
// install runs the module for one root: the document of a page, or the
// shadow root of a widget (REQ-ISL-20). Each lookup and each listener is on
// the root, so two roots on one page do not see the elements of each other.
export const install = (r: Document | ShadowRoot): void => {
  // The keys that a module takes before the others: the window of a page,
  // or the root itself.
  const top: EventTarget = (r as Document).defaultView ?? r

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
    const focused = el.contains(r.activeElement)
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
    r.querySelectorAll('[data-gx-toaster]').forEach((region) => {
      const paused = region === toastHover || region.contains(r.activeElement)
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
    r.querySelectorAll('[data-gx-toaster]').forEach((region) => {
      region.querySelectorAll<HTMLElement>('[data-gx-toast]').forEach((el) => {
        if (toastStates.has(el)) return
        toastStates.set(el, { left: +(el.getAttribute('data-duration') ?? 0), at: 0, timer: 0 })
        const old = el.id ? region.querySelector<HTMLElement>('#' + CSS.escape(el.id)) : null
        if (!old || old === el) return
        // The new toast takes the place of the earlier one with its id. It
        // skips the enter transition, so the swap does not move.
        const focused = old.contains(r.activeElement)
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

  // onKeydown listens on the window, so Escape on a toast does not reach the
  // dismiss handler of the behaviour runtime.
  const onKeydown = (e: KeyboardEvent): void => {
    const at = e.target as HTMLElement | null
    if (e.key === 'Escape' && !e.defaultPrevented) {
      const toast = at?.closest?.('[data-gx-toaster] [data-gx-toast]') as HTMLElement | null
      if (toast) {
        dismissToast(toast)
        e.stopPropagation()
        return
      }
    }
  }

  const onClick = (e: Event): void => {
    const at = e.target as Element | null
    const closeButton = at?.closest?.('[data-gx-close]') as HTMLElement | null
    if (closeButton) {
      const toast = closeButton.closest('[data-gx-toaster] [data-gx-toast]') as HTMLElement | null
      if (toast) {
        dismissToast(toast)
        return
      }
    }
  }

  {
    r.addEventListener('click', onClick)
    top.addEventListener('keydown', onKeydown as EventListener, true)
    for (const type of ['pointerover', 'pointerout', 'focusin', 'focusout']) {
      r.addEventListener(type, (e) => {
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
    installToasts()
    new MutationObserver(installToasts).observe(r, { subtree: true, childList: true })
  }
}
