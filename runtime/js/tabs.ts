// The Gx tabs module (REQ-REG-07, REQ-CNT-05). The app injects it only on
// pages whose markup uses one of its markers, and `just check` fails when it
// grows over 2 KB gzipped.
//
// Markup contract:
//   [data-gx-tabs]       wrapper of one tab set; data-sync names a sync key
//   [data-gx-tab]        one tab button; the value is its label
//   [data-gx-tab-item]   docs kit: holds one tab and its panel
//   [data-gx-tab-panel]  one panel; in a tab list the value is its label
//
// A tab list follows the arrow keys of its orientation, Home and End.
// Wrappers with the same sync key select the same label, and the choice
// stays in localStorage.

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

const onKeydown = (e: KeyboardEvent): void => {
  const at = e.target as HTMLElement | null
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
}

if (typeof document !== 'undefined') {
  document.addEventListener('click', (e) => {
    const button = (e.target as Element | null)?.closest?.('[data-gx-tab]') as HTMLElement | null
    if (!button) return
    const wrapper = button.closest('[data-gx-tabs]')
    if (!wrapper) return
    e.preventDefault()
    selectTab(wrapper, button.getAttribute('data-gx-tab') ?? '', true)
  })
  document.addEventListener('keydown', onKeydown, true)
  installTabs()
  new MutationObserver(installTabs).observe(document.documentElement, { subtree: true, childList: true })
}
