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
  await fixture('PopoverTrigger-Default').getByRole('button', { name: 'Open popover' }).click()
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

test('REQ-REG-07 context menu opens on a right click and closes on Escape', async () => {
  // Playwright's headless Chrome does not turn a synthetic right click into
  // a contextmenu event; dispatch the event the browser fires.
  await fixture('ContextMenuTrigger-Default').locator('[data-gx-contextmenu]').dispatchEvent('contextmenu', { button: 2 })
  await waitOpen('#demo-context')
  await page.keyboard.press('Escape')
  await waitClosed('#demo-context')
})

test('REQ-REG-07 menubar moves focus with the arrow keys', async () => {
  const first = fixture('Menubar-Default').getByRole('menuitem', { name: 'Home' })
  await first.focus()
  await page.keyboard.press('ArrowRight')
  await page.waitForFunction(() => document.activeElement?.textContent?.trim() === 'Docs')
})

test('REQ-REG-07 tooltip shows on keyboard focus', async () => {
  await fixture('Tooltip-Top').getByRole('button', { name: 'Hover me' }).focus()
  await page.waitForFunction(() => {
    const el = document.querySelector('section[data-fixture="Tooltip-Top"] [role="tooltip"]')
    return el !== null && getComputedStyle(el).display !== 'none'
  })
})

test('REQ-REG-07 hover card shows on keyboard focus', async () => {
  await fixture('HoverCard-User').getByRole('button', { name: '@ada' }).focus()
  await page.waitForFunction(() => {
    const section = document.querySelector('section[data-fixture="HoverCard-User"]')
    return section?.textContent?.includes('Ada Lovelace') === true
  })
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
  await page.waitForFunction(() => {
    const tab = document.querySelector('[data-fixture="Tabs-Two"] [data-gx-tab="Password"]')
    return tab?.getAttribute('aria-selected') === 'true'
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

test('REQ-REG-07 navigation menu and sidebar links take focus', async () => {
  const home = fixture('NavigationMenu-Default').getByRole('link', { name: 'Home' })
  expect(await home.getAttribute('tabindex')).not.toBe('-1')
  const sidebar = fixture('Sidebar-Full').getByRole('link', { name: 'Home' })
  expect(await sidebar.getAttribute('tabindex')).not.toBe('-1')
})

test('REQ-REG-07 scroll area and toast stay out of the tab order', async () => {
  const area = fixture('ScrollArea-Vertical').locator('div').first()
  expect(await area.getAttribute('tabindex')).toBeNull()
  const live = fixture('Toaster-WithToast').locator('#gx-toaster')
  expect(await live.getAttribute('aria-live')).toBe('polite')
})
