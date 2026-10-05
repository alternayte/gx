// Keyboard behaviour of every interactive registry component, driven on the
// dev gallery in a real browser (REQ-REG-07).
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { chromium, type Browser, type Page } from 'playwright-core'
import { startShop, type Shop } from './harness'

let shop: Shop
let browser: Browser
let page: Page

beforeAll(async () => {
  shop = await startShop({ gxdev: true })
  browser = await chromium.launch({ channel: 'chrome', headless: true })
}), 180000

afterAll(async () => {
  await Bun.sleep(300)
  try {
    await browser?.close()
  } catch {
    // already closed
  }
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
  const account = fixture('Tabs-Two').getByRole('button', { name: 'Account' })
  await account.focus()
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => {
    const tab = document.querySelector('[data-fixture="Tabs-Two"] [data-gx-tab="Password"]')
    return tab?.getAttribute('aria-expanded') === 'true'
  })
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
