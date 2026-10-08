// The behaviour modules of a widget (REQ-ISL-20, REQ-REG-07). The widget
// script loads this file from the Gx server when the HTML of a widget has a
// marker of one of the modules, and runs it for the shadow root of the
// widget. It holds the code of behavior.ts, tabs.ts, toast.ts and overlay.ts.
import { install as behavior } from './behavior-core'
import { install as overlay } from './overlay-core'
import { install as tabs } from './tabs-core'
import { install as toast } from './toast-core'

const installed = new WeakSet<ShadowRoot>()

// install runs the modules for one shadow root, one time.
export const install = (root: ShadowRoot): void => {
  if (installed.has(root)) return
  installed.add(root)
  // The toast module and the overlay module take a key before the dismiss
  // and roving handlers of the behaviour module, as on a page.
  toast(root)
  overlay(root)
  behavior(root)
  tabs(root)
  // A press on the host page, outside the widget, closes each open overlay
  // of the widget: the behaviour module sees a press that is in no overlay.
  document.addEventListener(
    'pointerdown',
    (e) => {
      if (root.isConnected && !e.composedPath().includes(root)) root.dispatchEvent(new Event('pointerdown'))
    },
    true,
  )
}
