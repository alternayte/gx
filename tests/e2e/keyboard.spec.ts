// Keyboard behaviour of every interactive registry component, driven on the
// dev gallery in a real browser (REQ-REG-07).
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop({ gxdev: true })
  browser = await launchBrowser()
}), 180000

afterAll(async () => {
  await Bun.sleep(300)
  await Bun.sleep(100)
  shop?.stop()
})

beforeEach(async () => {
  page = await browser.newPage()
  await page.goto(shop.url + '/_gx/gallery')
  await page.waitForSelector('section[data-fixture]')
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

// fixture returns one gallery section by its component-fixture name.
function fixture(name: string) {
  return page.locator(`section[data-fixture="${name}"]`)
}

// open checks that an element carries the open attribute or the popover
// state, and close is the opposite.
function waitOpen(selector: string): Promise<unknown> {
  return page.waitForFunction((sel) => {
    const el = document.querySelector(sel)
    return el?.hasAttribute('open') || el?.matches(':popover-open')
  }, selector)
}

function waitClosed(selector: string): Promise<unknown> {
  return page.waitForFunction((sel) => {
    const el = document.querySelector(sel)
    return !el?.hasAttribute('open') && !el?.matches(':popover-open')
  }, selector)
}

// waitFocus waits until the focused element, or the label of a focused
// checkbox or radio item, starts with the text.
function waitFocus(text: string): Promise<unknown> {
  return page.waitForFunction((want) => {
    const at = document.activeElement
    return ((at?.closest('label') ?? at)?.textContent ?? '').trim().startsWith(want)
  }, text)
}

// waitShown waits until hover or focus content is visible and opaque.
function waitShown(selector: string): Promise<unknown> {
  return page.waitForFunction((sel) => {
    const el = document.querySelector(sel)
    if (el === null) return false
    const style = getComputedStyle(el)
    return style.display !== 'none' && style.visibility === 'visible' && style.opacity === '1'
  }, selector)
}

test('REQ-REG-07 dialog opens from the trigger and closes on Escape', async () => {
  await fixture('Dialog-Default').getByRole('button', { name: 'Open dialog' }).click()
  await waitOpen('#demo-dialog')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-dialog')
})

test('REQ-REG-07 alert dialog ignores Escape and closes from Cancel', async () => {
  await fixture('AlertDialog-Default').getByRole('button', { name: 'Delete' }).first().click()
  await waitOpen('#demo-alert')
  await page.keyboard.press('Escape')
  await Bun.sleep(150)
  expect(await page.locator('#demo-alert').getAttribute('open')).not.toBeNull()
  await page.locator('#demo-alert').getByRole('button', { name: 'Cancel' }).click()
  await waitClosed('#demo-alert')
})

test('REQ-REG-07 sheet and drawer close on Escape', async () => {
  await fixture('Sheet-Right').getByRole('button', { name: 'Open sheet' }).click()
  await waitOpen('#demo-sheet')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-sheet')

  await fixture('Drawer-Default').getByRole('button', { name: 'Open drawer' }).click()
  await waitOpen('#demo-drawer')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-drawer')

  await fixture('Sheet-Bottom').getByRole('button', { name: 'Open bottom sheet' }).click()
  await waitOpen('#demo-sheet-bottom')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-sheet-bottom')
})

test('REQ-REG-07 popover toggles from the trigger and closes on Escape', async () => {
  const trigger = fixture('PopoverTrigger-Default').getByRole('button', { name: 'Open popover' })
  await trigger.click()
  await waitOpen('#demo-popover')
  // A second click on the trigger closes the popover and leaves it closed.
  await trigger.click()
  await waitClosed('#demo-popover')
  await Bun.sleep(300)
  expect(await page.locator('#demo-popover').evaluate((el) => el.matches(':popover-open'))).toBe(false)
  await trigger.click()
  await waitOpen('#demo-popover')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-popover')
})

test('REQ-REG-07 dropdown menu opens with ArrowDown and closes on Escape', async () => {
  const trigger = fixture('DropdownMenuTrigger-Default').getByRole('button', { name: 'Open menu' })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await waitOpen('#demo-dropdown')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'Profile')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-dropdown')
})

test('REQ-REG-07 dropdown menu moves one item per arrow key and jumps with Home, End and typeahead', async () => {
  const trigger = fixture('DropdownMenuTrigger-Default').getByRole('button', { name: 'Open menu' })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await waitFocus('Profile')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Settings')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Documentation')
  await page.keyboard.press('ArrowUp')
  await waitFocus('Settings')
  await page.keyboard.press('End')
  await waitFocus('Sign out')
  await page.keyboard.press('Home')
  await waitFocus('Profile')
  await page.keyboard.press('d')
  await waitFocus('Documentation')
})

test('REQ-REG-07 dropdown menu checkbox and radio items change with Space and Enter', async () => {
  const trigger = fixture('DropdownMenuTrigger-Default').getByRole('button', { name: 'Open menu' })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await waitFocus('Profile')
  // Typeahead reads the label of a checkbox item.
  await page.keyboard.type('pa')
  await waitFocus('Panel')
  const panel = page.locator('#demo-dropdown input[name="panel"]')
  expect(await panel.isChecked()).toBe(false)
  await page.keyboard.press('Space')
  expect(await panel.isChecked()).toBe(true)
  await page.keyboard.press('Enter')
  expect(await panel.isChecked()).toBe(false)
  await page.keyboard.press('ArrowDown')
  await waitFocus('Top')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Bottom')
  // The arrow keys move focus only; Space selects the radio item.
  const bottom = page.locator('#demo-dropdown input[value="bottom"]')
  expect(await bottom.isChecked()).toBe(false)
  await page.keyboard.press('Space')
  expect(await bottom.isChecked()).toBe(true)
  // A checkbox or radio item keeps the menu open.
  expect(await page.locator('#demo-dropdown').evaluate((el) => el.matches(':popover-open'))).toBe(true)
})

test('REQ-REG-07 dropdown menu passes a disabled item', async () => {
  const trigger = fixture('DropdownMenuTrigger-Ghost').getByRole('button', { name: 'Open at the end' })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await waitOpen('#demo-dropdown-end')
  await waitFocus('Rename')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Duplicate')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Delete')
  // Enter runs the item and closes the menu.
  await page.keyboard.press('Enter')
  await waitClosed('#demo-dropdown-end')
})

test('REQ-REG-07 context menu opens on a right click and closes on Escape', async () => {
  // Playwright's headless Chrome does not turn a synthetic right click into
  // a contextmenu event; dispatch the event the browser fires.
  await fixture('ContextMenuTrigger-Default').locator('[data-gx-contextmenu]').dispatchEvent('contextmenu', { button: 2 })
  await waitOpen('#demo-context')
  // The keyboard starts inside the menu.
  await waitFocus('Copy')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Cut')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-context')
})

test('REQ-REG-07 menubar moves focus with the arrow keys', async () => {
  const first = fixture('Menubar-Default').getByRole('menuitem', { name: 'File', exact: true })
  await first.focus()
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'View')
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'Profiles')
  await page.keyboard.press('Home')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'File')
})

test('REQ-REG-07 menubar opens a menu with ArrowDown and moves between menus', async () => {
  const view = fixture('Menubar-Default').getByRole('menuitem', { name: 'View' })
  await view.focus()
  await page.keyboard.press('ArrowDown')
  await waitOpen('#demo-menubar-view')
  await waitFocus('Always show bookmarks bar')
  // ArrowDown passes the disabled item and stops at the end.
  await page.keyboard.press('End')
  await waitFocus('Reload')
  // ArrowRight closes this menu and opens the next one.
  await page.keyboard.press('ArrowRight')
  await waitOpen('#demo-menubar-profiles')
  await waitClosed('#demo-menubar-view')
  await waitFocus('Ada')
  await page.keyboard.press('ArrowLeft')
  await waitOpen('#demo-menubar-view')
  await waitClosed('#demo-menubar-profiles')
  await waitFocus('Always show bookmarks bar')
  // Escape closes the menu and returns focus to its trigger.
  await page.keyboard.press('Escape')
  await waitClosed('#demo-menubar-view')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'View')
})

// subOpen waits for the state of the sub-menu content inside a menu.
function waitSub(menu: string, open: boolean): Promise<unknown> {
  return page.waitForFunction(
    ([sel, want]) => document.querySelector(`${sel} [data-gx-sub] > [popover]`)?.matches(':popover-open') === want,
    [menu, open] as const,
  )
}

test('REQ-REG-07 dropdown sub-menu opens with ArrowRight or Enter, closes with ArrowLeft or Escape, and a selection closes the tree', async () => {
  const trigger = fixture('DropdownMenuTrigger-Sub').getByRole('button', { name: 'Open with a sub-menu' })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await waitFocus('New tab')
  await page.keyboard.press('ArrowDown')
  await waitFocus('More tools')
  const subTrigger = page.locator('#demo-dropdown-sub [data-gx-sub] > button')
  expect(await subTrigger.getAttribute('aria-expanded')).toBe('false')
  // ArrowRight opens the sub-menu and moves to its first item.
  await page.keyboard.press('ArrowRight')
  await waitSub('#demo-dropdown-sub', true)
  await waitFocus('Save page')
  expect(await subTrigger.getAttribute('aria-expanded')).toBe('true')
  // The arrow keys move inside the sub-menu.
  await page.keyboard.press('ArrowDown')
  await waitFocus('Create shortcut')
  // ArrowLeft closes the sub-menu only and returns focus to its trigger.
  await page.keyboard.press('ArrowLeft')
  await waitSub('#demo-dropdown-sub', false)
  await waitFocus('More tools')
  expect(await subTrigger.getAttribute('aria-expanded')).toBe('false')
  expect(await page.locator('#demo-dropdown-sub').evaluate((el) => el.matches(':popover-open'))).toBe(true)
  // Enter opens it again, and Escape closes the sub-menu only.
  await page.keyboard.press('Enter')
  await waitSub('#demo-dropdown-sub', true)
  await waitFocus('Save page')
  await page.keyboard.press('Escape')
  await waitSub('#demo-dropdown-sub', false)
  await waitFocus('More tools')
  await Bun.sleep(150)
  expect(await page.locator('#demo-dropdown-sub').evaluate((el) => el.matches(':popover-open'))).toBe(true)
  // A selection in the sub-menu closes the whole tree; focus goes to the
  // first trigger.
  await page.keyboard.press('ArrowRight')
  await waitFocus('Save page')
  await page.keyboard.press('Enter')
  await waitClosed('#demo-dropdown-sub')
  await waitSub('#demo-dropdown-sub', false)
  await waitFocus('Open with a sub-menu')
})

test('REQ-REG-07 context menu sub-menu follows the sub-menu keys', async () => {
  await fixture('ContextMenuTrigger-Sub').locator('[data-gx-contextmenu]').dispatchEvent('contextmenu', { button: 2 })
  await waitOpen('#demo-context-sub')
  await waitFocus('Back')
  await page.keyboard.press('ArrowDown')
  await waitFocus('More tools')
  await page.keyboard.press('ArrowRight')
  await waitSub('#demo-context-sub', true)
  await waitFocus('Save page')
  await page.keyboard.press('ArrowLeft')
  await waitSub('#demo-context-sub', false)
  await waitFocus('More tools')
  expect(await page.locator('#demo-context-sub').evaluate((el) => el.matches(':popover-open'))).toBe(true)
  await page.keyboard.press('ArrowRight')
  await waitFocus('Save page')
  await page.keyboard.press('End')
  await waitFocus('Developer tools')
  await page.keyboard.press('Enter')
  await waitClosed('#demo-context-sub')
  await waitSub('#demo-context-sub', false)
})

test('REQ-REG-07 menubar sub-menu takes ArrowRight and ArrowLeft before the bar does', async () => {
  const file = fixture('Menubar-Sub').getByRole('menuitem', { name: 'File', exact: true })
  await file.focus()
  await page.keyboard.press('ArrowDown')
  await waitOpen('#demo-menubar-sub-file')
  await waitFocus('New tab')
  await page.keyboard.press('ArrowDown')
  await waitFocus('Share')
  // On a sub-menu trigger ArrowRight opens the sub-menu, not the next menu.
  await page.keyboard.press('ArrowRight')
  await waitSub('#demo-menubar-sub-file', true)
  await waitFocus('Email link')
  expect(await page.locator('#demo-menubar-sub-edit').evaluate((el) => el.matches(':popover-open'))).toBe(false)
  // ArrowLeft closes the sub-menu and keeps the menu.
  await page.keyboard.press('ArrowLeft')
  await waitSub('#demo-menubar-sub-file', false)
  await waitFocus('Share')
  expect(await page.locator('#demo-menubar-sub-file').evaluate((el) => el.matches(':popover-open'))).toBe(true)
  await page.keyboard.press('ArrowRight')
  await waitFocus('Email link')
  await page.keyboard.press('Enter')
  await waitClosed('#demo-menubar-sub-file')
  await waitSub('#demo-menubar-sub-file', false)
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'File')
})

// box returns the client rectangle of one element and the size of the
// viewport without its scrollbars.
function box(selector: string): Promise<{ left: number; top: number; right: number; bottom: number; width: number; height: number; side: string | null }> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel)!
    const r = el.getBoundingClientRect()
    return {
      left: r.left,
      top: r.top,
      right: r.right,
      bottom: r.bottom,
      width: document.documentElement.clientWidth,
      height: document.documentElement.clientHeight,
      side: el.getAttribute('data-side'),
    }
  }, selector)
}

// settled waits until the enter transition of an open popover is over, so
// its rectangle has no scale.
function settled(selector: string): Promise<unknown> {
  return page.waitForFunction((sel) => {
    const el = document.querySelector(sel)
    return el?.matches(':popover-open') && getComputedStyle(el).scale === '1' && getComputedStyle(el).opacity === '1'
  }, selector)
}

test('REQ-REG-07 a menu flips at the bottom edge and shifts at the right edge of the viewport', async () => {
  const name = 'section[data-fixture="DropdownMenuTrigger-Sub"] button'
  const trigger = page.locator(name)
  // In the middle of the viewport the menu opens below its trigger.
  await trigger.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().top - 200))
  await trigger.click()
  await settled('#demo-dropdown-sub')
  let at = await box(name)
  let menu = await box('#demo-dropdown-sub')
  expect(menu.side).toBe('bottom')
  expect(Math.round(menu.top - at.bottom)).toBe(4)
  // The menu is centred on the trigger.
  expect(Math.abs((menu.left + menu.right) / 2 - (at.left + at.right) / 2)).toBeLessThan(1)
  await page.keyboard.press('Escape')
  await waitClosed('#demo-dropdown-sub')

  // The trigger sits at the bottom edge: the menu opens upward.
  await trigger.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().bottom - document.documentElement.clientHeight + 12))
  await trigger.click()
  await settled('#demo-dropdown-sub')
  at = await box(name)
  menu = await box('#demo-dropdown-sub')
  expect(menu.side).toBe('top')
  expect(Math.round(at.top - menu.bottom)).toBe(4)
  expect(menu.top).toBeGreaterThanOrEqual(8)
  expect(menu.bottom).toBeLessThanOrEqual(menu.height - 8)
  await page.keyboard.press('Escape')
  await waitClosed('#demo-dropdown-sub')

  // The trigger sits at the right edge: the menu shifts left and stays 8px
  // inside. The sub-menu has no room on the right and opens on the left of
  // its trigger.
  await trigger.evaluate((el) => {
    el.style.position = 'fixed'
    el.style.right = '12px'
    el.style.top = '200px'
  })
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  await settled('#demo-dropdown-sub')
  at = await box(name)
  menu = await box('#demo-dropdown-sub')
  expect(menu.side).toBe('bottom')
  expect(Math.round(menu.width - menu.right)).toBe(8)
  expect(menu.left).toBeGreaterThanOrEqual(8)
  // Unshifted, the centred menu would end right of the viewport.
  expect((at.left + at.right) / 2 + (menu.right - menu.left) / 2).toBeGreaterThan(menu.width)
  await page.keyboard.press('ArrowDown')
  await waitFocus('More tools')
  await page.keyboard.press('ArrowRight')
  const sub = '#demo-dropdown-sub [data-gx-sub] > [popover]'
  await settled(sub)
  const subTrigger = await box('#demo-dropdown-sub [data-gx-sub] > button')
  const content = await box(sub)
  expect(content.side).toBe('left')
  expect(Math.round(content.right)).toBe(Math.round(subTrigger.left))
  expect(content.left).toBeGreaterThanOrEqual(8)
})

test('REQ-REG-07 a popover and a context menu stay inside the viewport', async () => {
  // A popover at the bottom edge opens upward.
  const name = 'section[data-fixture="PopoverTrigger-Default"] button'
  const trigger = page.locator(name)
  await trigger.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().bottom - document.documentElement.clientHeight + 12))
  await trigger.click()
  await settled('#demo-popover')
  const at = await box(name)
  const popover = await box('#demo-popover')
  expect(popover.side).toBe('top')
  expect(Math.round(at.top - popover.bottom)).toBe(4)
  expect(popover.top).toBeGreaterThanOrEqual(8)
  await page.keyboard.press('Escape')
  await waitClosed('#demo-popover')

  // A right click near the right and bottom edges: the context menu opens
  // on the left of the pointer and shifts up.
  await page.setViewportSize({ width: 360, height: 400 })
  const area = fixture('ContextMenuTrigger-Default').locator('[data-gx-contextmenu]')
  await area.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().bottom - document.documentElement.clientHeight + 4))
  const r = await box('section[data-fixture="ContextMenuTrigger-Default"] [data-gx-contextmenu]')
  const x = Math.round(r.right - 10)
  const y = Math.round(r.bottom - 10)
  // The event needs its pointer position, so the page builds a mouse event.
  await area.evaluate(
    (el, [clientX, clientY]) => el.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2, clientX, clientY })),
    [x, y],
  )
  await settled('#demo-context')
  const menu = await box('#demo-context')
  expect(menu.side).toBe('left')
  expect(Math.round(menu.right)).toBe(x - 2)
  expect(menu.left).toBeGreaterThanOrEqual(8)
  expect(Math.round(menu.height - menu.bottom)).toBe(8)
  expect(menu.top).toBeGreaterThanOrEqual(8)
})

test('REQ-REG-07 tooltip shows on keyboard focus', async () => {
  const tip = 'section[data-fixture="Tooltip-Top"] [role="tooltip"]'
  expect(await page.locator(tip).evaluate((el) => getComputedStyle(el).visibility)).toBe('hidden')
  await fixture('Tooltip-Top').getByRole('button', { name: 'Hover me' }).focus()
  await waitShown(tip)
})

test('REQ-REG-07 hover card shows on keyboard focus', async () => {
  const card = 'section[data-fixture="HoverCard-User"] .fixture-body > span > span:last-child'
  expect(await page.locator(card).evaluate((el) => getComputedStyle(el).visibility)).toBe('hidden')
  await fixture('HoverCard-User').getByRole('button', { name: '@ada' }).focus()
  await waitShown(card)
  expect(await page.locator(card).textContent()).toContain('Ada Lovelace')
})

test('REQ-REG-07 accordion toggles with Space', async () => {
  const summary = fixture('Accordion-Two').locator('summary').first()
  await summary.focus()
  await page.keyboard.press('Space')
  await page.waitForFunction(() => document.querySelector('section[data-fixture="Accordion-Two"] details')?.hasAttribute('open') === false)
  await page.keyboard.press('Space')
  await page.waitForFunction(() => document.querySelector('section[data-fixture="Accordion-Two"] details')?.hasAttribute('open') === true)
})

test('REQ-REG-07 collapsible toggles with Space', async () => {
  const summary = fixture('Collapsible-Closed').locator('summary')
  await summary.focus()
  await page.keyboard.press('Space')
  await page.waitForFunction(() => document.querySelector('section[data-fixture="Collapsible-Closed"] details')?.hasAttribute('open') === true)
})

test('REQ-REG-07 tabs switch with the arrow keys', async () => {
  const account = fixture('Tabs-Two').getByRole('tab', { name: 'Account' })
  await account.focus()
  await page.keyboard.press('ArrowRight')
  // The triggers sit in one tab list; the selected tab shows its panel and
  // hides the other one.
  await page.waitForFunction(() => {
    const root = document.querySelector('[data-fixture="Tabs-Two"]')
    const tab = root?.querySelector('[role="tablist"] [data-gx-tab="Password"]')
    const shown = [...(root?.querySelectorAll<HTMLElement>('[role="tabpanel"]') ?? [])].filter((panel) => !panel.hidden)
    return (
      tab?.getAttribute('aria-selected') === 'true' &&
      document.activeElement === tab &&
      shown.length === 1 &&
      shown[0].getAttribute('aria-labelledby') === tab.id
    )
  })
})

test('REQ-REG-07 vertical tabs follow the vertical arrow keys and skip a disabled tab', async () => {
  const selected = (name: string, label: string) =>
    page.waitForFunction(
      ([f, l]) => document.querySelector(`[data-fixture="${f}"] [data-gx-tab="${l}"]`)?.getAttribute('aria-selected') === 'true',
      [name, label],
    )
  await fixture('Tabs-Vertical').getByRole('tab', { name: 'Account' }).focus()
  // ArrowRight belongs to a horizontal list: it must not move a vertical one.
  await page.keyboard.press('ArrowRight')
  await page.keyboard.press('ArrowDown')
  await selected('Tabs-Vertical', 'Password')
  await page.keyboard.press('ArrowUp')
  await selected('Tabs-Vertical', 'Account')

  await fixture('Tabs-Disabled').getByRole('tab', { name: 'Account' }).focus()
  await page.keyboard.press('ArrowRight')
  await selected('Tabs-Disabled', 'Password')
  await page.keyboard.press('Home')
  await selected('Tabs-Disabled', 'Account')
})

test('REQ-REG-07 toggle switches with Space', async () => {
  const input = fixture('Toggle-Off').locator('input[type="checkbox"]')
  await input.focus()
  await page.keyboard.press('Space')
  expect(await input.isChecked()).toBe(true)
})

test('REQ-REG-07 toggle group moves the choice with the arrow keys', async () => {
  const checked = fixture('ToggleGroup-Three').locator('input:checked')
  await checked.focus()
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => {
    const section = document.querySelector('section[data-fixture="ToggleGroup-Three"]')
    const inputs = [...(section?.querySelectorAll('input') ?? [])]
    return inputs[1].checked
  })
})

test('REQ-REG-07 select changes with typeahead', async () => {
  const select = fixture('Select-Plan').locator('select')
  await select.focus()
  // Typeahead selects the option that starts with the typed letter.
  await page.keyboard.press('p')
  expect(await select.inputValue()).toBe('pro')
})

test('REQ-REG-07 slider changes with the arrow keys', async () => {
  const slider = fixture('Slider-Half').locator('input[type="range"]')
  await slider.focus()
  await page.keyboard.press('ArrowRight')
  expect(await slider.inputValue()).toBe('51')
})

test('REQ-REG-07 navigation menu shows its content on keyboard focus', async () => {
  const content = 'section[data-fixture="NavigationMenu-Default"] li > div'
  expect(await page.locator(content).evaluate((el) => getComputedStyle(el).visibility)).toBe('hidden')
  await fixture('NavigationMenu-Default').getByRole('button', { name: 'Products' }).focus()
  await waitShown(content)
  // Tab moves into the content and the content stays.
  await page.keyboard.press('Tab')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'All products')
  await waitShown(content)
})

test('REQ-REG-07 navigation menu and sidebar links take focus', async () => {
  const home = fixture('NavigationMenu-Default').getByRole('link', { name: 'Home' })
  expect(await home.getAttribute('tabindex')).not.toBe('-1')
  const sidebar = fixture('Sidebar-Full').getByRole('link', { name: 'Home' })
  expect(await sidebar.getAttribute('tabindex')).not.toBe('-1')
})

test('REQ-REG-07 scroll area takes focus and the toast stays out', async () => {
  const area = fixture('ScrollArea-Vertical').locator('.fixture-body > div')
  expect(await area.getAttribute('tabindex')).toBe('0')
  const live = fixture('Toaster-WithToast').locator('#gx-toaster')
  expect(await live.getAttribute('aria-live')).toBe('polite')
})

// The tier 3 components are islands (REQ-REG-14). A test waits until the
// island of its fixture runs.
async function islandReady(name: string): Promise<void> {
  const section = fixture(name)
  await section.scrollIntoViewIfNeeded()
  await page.waitForFunction(
    (sel) => [...document.querySelectorAll(sel + ' gx-island')].every((el) => el.matches(':state(mounted)')),
    `section[data-fixture="${name}"]`,
  )
}

test('REQ-REG-14 input-otp shows the typed digits in its slots', async () => {
  await islandReady('InputOTP-Empty')
  const section = fixture('InputOTP-Empty')
  const input = section.locator('#otp-empty')
  const slots = section.locator('[data-otp-slot]')
  expect(await slots.count()).toBe(6)
  await input.focus()
  // The first slot is active and shows the caret.
  expect(await slots.nth(0).getAttribute('data-active')).toBe('true')
  await page.keyboard.type('12a3')
  // A letter is not part of a code.
  expect(await input.inputValue()).toBe('123')
  expect(await slots.allTextContents()).toEqual(['1', '2', '3', '', '', ''])
  expect(await slots.nth(3).getAttribute('data-active')).toBe('true')
  await page.keyboard.press('Backspace')
  expect(await slots.allTextContents()).toEqual(['1', '2', '', '', '', ''])
  expect(await slots.nth(2).getAttribute('data-active')).toBe('true')
  await page.keyboard.press('ArrowLeft')
  await page.waitForFunction(() => document.querySelector('#otp-empty')!.parentElement!.querySelector('[data-otp-slot="1"]')!.getAttribute('data-active') === 'true')
})

test('REQ-REG-14 input-otp takes a paste and stops at its length', async () => {
  await islandReady('InputOTP-Four')
  const section = fixture('InputOTP-Four')
  const input = section.locator('#otp-four')
  await input.focus()
  await page.evaluate(() => {
    const el = document.querySelector('#otp-four') as HTMLInputElement
    const data = new DataTransfer()
    data.setData('text/plain', '98-76 54')
    // The island takes the paste: the browser alone cuts the text at four
    // characters and loses a digit.
    el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }))
  })
  expect(await input.inputValue()).toBe('9876')
  expect(await section.locator('[data-otp-slot]').allTextContents()).toEqual(['9', '8', '7', '6'])
})

test('REQ-REG-14 input-otp groups its slots and keeps a value from the server', async () => {
  await islandReady('InputOTP-Grouped')
  expect(await fixture('InputOTP-Grouped').locator('[data-otp-separator]').count()).toBe(1)
  await islandReady('InputOTP-Filled')
  expect(await fixture('InputOTP-Filled').locator('[data-otp-slot]').allTextContents()).toEqual(['1', '2', '3', '', '', ''])
  // The input is the control: a form sends its value.
  expect(await fixture('InputOTP-Filled').locator('#otp-filled').getAttribute('name')).toBe('code')
})

// dayButton is the button of one day of a calendar fixture.
const dayButton = (name: string, iso: string) => fixture(name).locator(`button[data-day="${iso}"]`)

// focusedDay is the day that has the focus.
const focusedDay = (): Promise<string | null> => page.evaluate(() => (document.activeElement as HTMLElement | null)?.dataset.day ?? null)

test('REQ-REG-14 calendar moves by day, week, month and year with the keyboard', async () => {
  await islandReady('Calendar-Chosen')
  const section = fixture('Calendar-Chosen')
  expect(await section.locator('[aria-live]').textContent()).toBe('October 2026')
  // The chosen day is the one stop of the grid for the Tab key.
  expect(await section.locator('tbody button[tabindex="0"]').getAttribute('data-day')).toBe('2026-10-14')
  expect(await section.locator('td[aria-selected="true"] button').getAttribute('data-day')).toBe('2026-10-14')
  await dayButton('Calendar-Chosen', '2026-10-14').focus()
  const steps: [string, string][] = [
    ['ArrowRight', '2026-10-15'],
    ['ArrowDown', '2026-10-22'],
    ['ArrowLeft', '2026-10-21'],
    ['ArrowUp', '2026-10-14'],
    // The week starts on Sunday: 11 to 17 October.
    ['Home', '2026-10-11'],
    ['End', '2026-10-17'],
    ['PageDown', '2026-11-17'],
    ['PageUp', '2026-10-17'],
    ['Shift+PageDown', '2027-10-17'],
    ['Shift+PageUp', '2026-10-17'],
  ]
  for (const [key, want] of steps) {
    await page.keyboard.press(key)
    expect(await focusedDay()).toBe(want)
  }
  // An arrow key past the end of the month shows the next month.
  await dayButton('Calendar-Chosen', '2026-10-17').focus()
  for (let i = 0; i < 2; i++) await page.keyboard.press('ArrowDown')
  expect(await focusedDay()).toBe('2026-10-31')
  await page.keyboard.press('ArrowRight')
  expect(await focusedDay()).toBe('2026-11-01')
  expect(await section.locator('[aria-live]').textContent()).toBe('November 2026')
})

test('REQ-REG-14 calendar writes the chosen day into its date input', async () => {
  await islandReady('Calendar-Empty')
  const section = fixture('Calendar-Empty')
  const input = section.locator('#cal-empty')
  expect(await input.inputValue()).toBe('')
  expect(await section.locator('[aria-live]').textContent()).toBe('February 2026')
  await page.evaluate(() => {
    const w = window as unknown as { changes: string[] }
    w.changes = []
    document.querySelector('#cal-empty')!.addEventListener('change', (e) => w.changes.push((e.target as HTMLInputElement).value))
  })
  await dayButton('Calendar-Empty', '2026-02-01').focus()
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  expect(await input.inputValue()).toBe('2026-02-08')
  expect(await page.evaluate(() => (window as unknown as { changes: string[] }).changes)).toEqual(['2026-02-08'])
  expect(await section.locator('td[aria-selected="true"] button').getAttribute('data-day')).toBe('2026-02-08')
  // The focus stays on the chosen day, and the input is the form control.
  expect(await focusedDay()).toBe('2026-02-08')
  expect(await input.getAttribute('name')).toBe('day')
  // The month buttons change the month and keep the choice.
  await section.getByRole('button', { name: 'Next month' }).click()
  expect(await section.locator('[aria-live]').textContent()).toBe('March 2026')
  expect(await input.inputValue()).toBe('2026-02-08')
})

test('REQ-REG-14 calendar follows the week start, the limits and the language', async () => {
  await islandReady('Calendar-Monday')
  expect(await fixture('Calendar-Monday').locator('thead th').first().getAttribute('abbr')).toBe('Monday')
  await islandReady('Calendar-Limited')
  expect(await dayButton('Calendar-Limited', '2026-10-09').isDisabled()).toBe(true)
  expect(await dayButton('Calendar-Limited', '2026-10-10').isDisabled()).toBe(false)
  expect(await dayButton('Calendar-Limited', '2026-10-21').isDisabled()).toBe(true)
  await islandReady('Calendar-German')
  expect(await fixture('Calendar-German').locator('[aria-live]').textContent()).toBe('Oktober 2026')
})

test('REQ-REG-14 date-picker opens a calendar, takes a day and closes', async () => {
  await islandReady('DatePicker-Empty')
  const section = fixture('DatePicker-Empty')
  const trigger = section.getByRole('button', { name: 'Pick a date' })
  await trigger.focus()
  await page.keyboard.press('Enter')
  await waitOpen('#date-empty-popover')
  // The focus is in the grid, on the day of today.
  await page.waitForFunction(() => (document.activeElement as HTMLElement | null)?.dataset.day !== undefined)
  const first = await focusedDay()
  await page.keyboard.press('ArrowRight')
  const next = await focusedDay()
  expect(next).not.toBe(first)
  await page.keyboard.press('Enter')
  await waitClosed('#date-empty-popover')
  expect(await section.locator('#date-empty').inputValue()).toBe(next!)
  // The button shows the day as text and takes the focus back.
  const text = await section.locator('#date-empty-display').textContent()
  expect(text).not.toBe('Pick a date')
  expect(text).toContain(String(Number(next!.slice(8))))
  expect(await section.locator('#date-empty-display').getAttribute('data-empty')).toBe('false')
  await page.waitForFunction(() => document.activeElement?.getAttribute('popovertarget') === 'date-empty-popover')
})

test('REQ-REG-14 date-picker closes on Escape and shows the day of the server', async () => {
  await islandReady('DatePicker-Chosen')
  const section = fixture('DatePicker-Chosen')
  expect(await section.locator('#date-chosen-display').textContent()).toBe('Wednesday, October 14, 2026')
  await section.locator('button[popovertarget="date-chosen-popover"]').click()
  await waitOpen('#date-chosen-popover')
  await page.waitForFunction(() => (document.activeElement as HTMLElement | null)?.dataset.day === '2026-10-14')
  await page.keyboard.press('Escape')
  await waitClosed('#date-chosen-popover')
  expect(await section.locator('#date-chosen').inputValue()).toBe('2026-10-14')
})

// activeOption is the text of the option that a combobox input points at.
const activeOption = (inputSelector: string): Promise<string | null> =>
  page.evaluate((sel) => {
    const id = document.querySelector(sel)?.getAttribute('aria-activedescendant')
    return id ? (document.getElementById(id)?.querySelector('span')?.textContent ?? document.getElementById(id)?.textContent ?? null) : null
  }, inputSelector)

test('REQ-REG-14 combobox filters, moves with the arrow keys and chooses with Enter', async () => {
  await islandReady('Combobox-Empty')
  const section = fixture('Combobox-Empty')
  const input = section.getByRole('combobox', { name: 'Framework' })
  const select = section.locator('#combo-empty')
  expect(await input.getAttribute('placeholder')).toBe('Select a framework')
  expect(await input.getAttribute('aria-expanded')).toBe('false')
  await input.focus()
  await page.keyboard.press('ArrowDown')
  expect(await input.getAttribute('aria-expanded')).toBe('true')
  expect(await section.getByRole('option').count()).toBe(6)
  expect(await activeOption('[data-fixture="Combobox-Empty"] input')).toBe('Gx')
  // Rails is disabled: the arrow keys go past it.
  for (let i = 0; i < 3; i++) await page.keyboard.press('ArrowDown')
  expect(await activeOption('[data-fixture="Combobox-Empty"] input')).toBe('Next.js')
  await page.keyboard.press('ArrowDown')
  expect(await activeOption('[data-fixture="Combobox-Empty"] input')).toBe('SvelteKit')
  await page.keyboard.press('Home')
  expect(await activeOption('[data-fixture="Combobox-Empty"] input')).toBe('Gx')
  // Typing filters the list by the label.
  await page.keyboard.type('mp')
  expect(await section.getByRole('option').allTextContents()).toEqual(['templ', 'gomponents'])
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  expect(await input.getAttribute('aria-expanded')).toBe('false')
  expect(await input.inputValue()).toBe('gomponents')
  // The select is the form control and holds the value.
  expect(await select.inputValue()).toBe('gomponents')
  expect(await select.getAttribute('name')).toBe('framework')
})

test('REQ-REG-14 combobox closes on Escape and keeps the chosen option', async () => {
  await islandReady('Combobox-Chosen')
  const section = fixture('Combobox-Chosen')
  const input = section.getByRole('combobox', { name: 'Framework' })
  expect(await input.inputValue()).toBe('templ')
  await input.focus()
  await page.keyboard.press('ArrowDown')
  // The chosen option is in the list, and the list shows every option.
  expect(await section.locator('[role=option][aria-selected="true"]').textContent()).toBe('templ')
  expect(await section.getByRole('option').count()).toBe(6)
  await input.fill('zzz')
  expect(await section.getByText('No option found.').isVisible()).toBe(true)
  await page.keyboard.press('Escape')
  expect(await input.getAttribute('aria-expanded')).toBe('false')
  expect(await input.inputValue()).toBe('templ')
  expect(await section.locator('#combo-chosen').inputValue()).toBe('templ')
  // A click chooses too.
  await input.click()
  await section.getByRole('option', { name: 'Next.js' }).click()
  expect(await section.locator('#combo-chosen').inputValue()).toBe('next')
  expect(await input.inputValue()).toBe('Next.js')
})

test('REQ-REG-14 command filters its items and runs the active one', async () => {
  await islandReady('Command-Default')
  const section = fixture('Command-Default')
  const input = section.getByRole('combobox', { name: 'Commands' })
  const active = '[data-fixture="Command-Default"] input'
  await input.focus()
  expect(await activeOption(active)).toBe('Calendar')
  await page.keyboard.press('ArrowDown')
  expect(await activeOption(active)).toBe('Search emoji')
  // Calculator is disabled: the next item is Profile.
  await page.keyboard.press('ArrowDown')
  expect(await activeOption(active)).toBe('Profile')
  await page.keyboard.press('End')
  expect(await activeOption(active)).toBe('Settings')
  await page.keyboard.press('Home')
  expect(await activeOption(active)).toBe('Calendar')
  // The search reads the keywords: "smile" finds Search emoji.
  await page.keyboard.type('smile')
  expect(await section.locator('[data-command-item]:visible > span').allTextContents()).toEqual(['Search emoji'])
  expect(await section.locator('[data-command-group]:visible').count()).toBe(1)
  await input.fill('zzz')
  expect(await section.getByText('No results found.').isVisible()).toBe(true)
  expect(await activeOption(active)).toBe(null)
  // Enter runs the active item: a button sends command-select with its value.
  await input.fill('sett')
  await page.evaluate(() => {
    const w = window as unknown as { ran: string[] }
    w.ran = []
    document.querySelector('#command-default')!.addEventListener('command-select', (e) => w.ran.push((e as CustomEvent).detail.value))
  })
  await page.keyboard.press('Enter')
  expect(await page.evaluate(() => (window as unknown as { ran: string[] }).ran)).toEqual(['settings'])
  // A link item goes to its address.
  await input.fill('bill')
  await page.keyboard.press('Enter')
  await page.waitForFunction(() => location.hash === '#billing')
})

test('REQ-REG-14 carousel moves one slide with its buttons and the arrow keys', async () => {
  await islandReady('Carousel-Default')
  const section = fixture('Carousel-Default')
  const viewport = section.locator('#carousel-default-viewport')
  const status = section.locator('[aria-live]')
  const prev = section.getByRole('button', { name: 'Previous slide' })
  const next = section.getByRole('button', { name: 'Next slide' })
  const current = (): Promise<string | null> => section.locator('[data-current="true"]').textContent()
  expect(await status.textContent()).toBe('Slide 1 of 3')
  expect(await prev.isDisabled()).toBe(true)
  await next.click()
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Default"] [aria-live]')?.textContent === 'Slide 2 of 3')
  expect((await current())?.trim()).toBe('2')
  expect(await prev.isDisabled()).toBe(false)
  // The arrow keys on the slides move one slide.
  await viewport.focus()
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Default"] [aria-live]')?.textContent === 'Slide 3 of 3')
  expect(await next.isDisabled()).toBe(true)
  await page.keyboard.press('Home')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Default"] [aria-live]')?.textContent === 'Slide 1 of 3')
  await page.keyboard.press('End')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Default"] [aria-live]')?.textContent === 'Slide 3 of 3')
  await page.keyboard.press('ArrowLeft')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Default"] [aria-live]')?.textContent === 'Slide 2 of 3')
  // The slides are a named group, and each slide says what it is.
  expect(await section.locator('section[aria-roledescription="carousel"]').getAttribute('aria-label')).toBe('Numbers')
  expect(await section.locator('[aria-roledescription="slide"]').count()).toBe(3)
})

test('REQ-REG-14 a vertical carousel uses the up and down keys', async () => {
  await islandReady('Carousel-Vertical')
  const section = fixture('Carousel-Vertical')
  await section.locator('#carousel-vertical-viewport').focus()
  await page.keyboard.press('ArrowDown')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Vertical"] [aria-live]')?.textContent === 'Slide 2 of 3')
  await page.keyboard.press('ArrowUp')
  await page.waitForFunction(() => document.querySelector('[data-fixture="Carousel-Vertical"] [aria-live]')?.textContent === 'Slide 1 of 3')
})

test('REQ-REG-14 resizable changes the panel sizes with the arrow keys inside the limits', async () => {
  await islandReady('ResizablePanelGroup-Horizontal')
  const section = fixture('ResizablePanelGroup-Horizontal')
  const handle = section.locator('#resize-h')
  const widths = (): Promise<number[]> =>
    section.locator('[data-slot="resizable-panel"]').evaluateAll((els) => {
      // A size is a part of the room that the panels share; the handle
      // takes none of it.
      const total = els.reduce((sum, el) => sum + el.getBoundingClientRect().width, 0)
      return els.map((el) => Math.round((el.getBoundingClientRect().width / total) * 100))
    })
  expect(await handle.getAttribute('role')).toBe('separator')
  expect(await handle.getAttribute('aria-valuenow')).toBe('40')
  expect(await widths()).toEqual([40, 60])
  await handle.focus()
  await page.keyboard.press('ArrowRight')
  expect(await handle.getAttribute('aria-valuenow')).toBe('45')
  expect(await widths()).toEqual([45, 55])
  await page.keyboard.press('ArrowLeft')
  await page.keyboard.press('ArrowLeft')
  expect(await widths()).toEqual([35, 65])
  // The first panel has a minimum of 20, and the second one of 30.
  await page.keyboard.press('Home')
  expect(await widths()).toEqual([20, 80])
  expect(await handle.getAttribute('aria-valuemin')).toBe('20')
  await page.keyboard.press('End')
  expect(await widths()).toEqual([70, 30])
  expect(await handle.getAttribute('aria-valuemax')).toBe('70')
})

test('REQ-REG-14 resizable follows a drag of its handle', async () => {
  await islandReady('ResizablePanelGroup-Vertical')
  const section = fixture('ResizablePanelGroup-Vertical')
  const handle = section.locator('#resize-v')
  const box = (await handle.boundingBox())!
  const group = (await section.locator('[data-slot="resizable-panel-group"]').boundingBox())!
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
  await page.mouse.down()
  await page.mouse.move(box.x + box.width / 2, group.y + group.height * 0.25, { steps: 5 })
  await page.mouse.up()
  const now = Number(await handle.getAttribute('aria-valuenow'))
  expect(now).toBeGreaterThan(20)
  expect(now).toBeLessThan(30)
  // The arrow keys of a vertical group are up and down.
  await handle.focus()
  await page.keyboard.press('ArrowDown')
  // One step is 5 percent; the value in the attribute is a whole number.
  const stepped = Number(await handle.getAttribute('aria-valuenow'))
  expect(stepped).toBeGreaterThanOrEqual(now + 4)
  expect(stepped).toBeLessThanOrEqual(now + 6)
})

test('REQ-REG-14 chart draws the data and shows the values of a category with the arrow keys', async () => {
  await islandReady('Chart-Bar')
  const section = fixture('Chart-Bar')
  // Two series of six values: twelve bars, and a legend with both names.
  expect(await section.locator('svg rect[data-series]').count()).toBe(12)
  expect(await section.locator('ul li').allTextContents()).toEqual(['Desktop', 'Mobile'])
  // The largest value, 305, has the tallest bar.
  const heights = await section.locator('svg rect[data-series="Desktop"]').evaluateAll((els) => els.map((el) => Number(el.getAttribute('height'))))
  expect(heights.indexOf(Math.max(...heights))).toBe(1)
  // The data table stays in the page for a screen reader.
  expect(await section.locator('table#chart-bar-data tbody tr').count()).toBe(2)
  expect(await section.locator('table#chart-bar-data td').first().textContent()).toBe('186')
  const figure = section.getByRole('img')
  expect(await figure.getAttribute('aria-label')).toContain('Visitors')
  const tooltip = section.getByRole('status')
  expect(await tooltip.isHidden()).toBe(true)
  await figure.focus()
  await page.keyboard.press('ArrowRight')
  expect(await tooltip.innerText()).toBe('Jan\nDesktop: 186\nMobile: 80')
  await page.keyboard.press('ArrowRight')
  expect(await tooltip.innerText()).toBe('Feb\nDesktop: 305\nMobile: 200')
  await page.keyboard.press('End')
  expect(await tooltip.innerText()).toBe('Jun\nDesktop: 214\nMobile: 140')
  await page.keyboard.press('Home')
  expect(await tooltip.innerText()).toContain('Jan')
  await page.keyboard.press('Escape')
  expect(await tooltip.isHidden()).toBe(true)
})

test('REQ-REG-14 chart draws a line and an area', async () => {
  await islandReady('Chart-Line')
  expect(await fixture('Chart-Line').locator('svg polyline').count()).toBe(2)
  expect(await fixture('Chart-Line').locator('svg circle').count()).toBe(12)
  await islandReady('Chart-Area')
  expect(await fixture('Chart-Area').locator('svg polygon').count()).toBe(1)
})
