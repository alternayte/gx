// The fallback path of partly supported platform features in Firefox and
// WebKit (REQ-REG-13). Behaviour, keyboard and accessibility are checked;
// pixels are not.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { firefox, webkit, type BrowserType } from 'playwright-core'
import { startShop, type Shop } from './harness'

const axeSource = readFileSync(new URL('./node_modules/axe-core/axe.min.js', import.meta.url), 'utf8')

let shop: Shop

beforeAll(async () => {
  shop = await startShop({ gxdev: true })
}), 180000

afterAll(async () => {
  shop?.stop()
})

const engines: [string, BrowserType][] = [
  ['firefox', firefox],
  ['webkit', webkit],
]

for (const [name, engine] of engines) {
  test(`REQ-REG-13 ${name}: popover, menu and tooltip work without anchor positioning`, async () => {
    const browser = await engine.launch({ headless: true })
    const page = await browser.newPage()
    try {
      await page.goto(shop.url + '/_gx/gallery')
      await page.waitForSelector('section[data-fixture]')
      const fixture = (n: string) => page.locator(`section[data-fixture="${n}"]`)

      // Popover: native popover with the fallback placement.
      await fixture('PopoverTrigger-Default').getByRole('button', { name: 'Open popover' }).click()
      await page.waitForFunction(() => document.querySelector('#demo-popover')?.matches(':popover-open'))
      await page.keyboard.press('Escape')
      await page.waitForFunction(() => !document.querySelector('#demo-popover')?.matches(':popover-open'))

      // Dropdown menu: ArrowDown enters the menu, Escape closes it.
      const trigger = fixture('DropdownMenuTrigger-Default').getByRole('button', { name: 'Open menu' })
      await trigger.focus()
      await page.keyboard.press('ArrowDown')
      await page.waitForFunction(() => document.querySelector('#demo-dropdown')?.matches(':popover-open'))
      await page.keyboard.press('Escape')
      await page.waitForFunction(() => !document.querySelector('#demo-dropdown')?.matches(':popover-open'))

      // Tooltip is CSS only and works on every engine.
      await fixture('Tooltip-Top').getByRole('button', { name: 'Hover me' }).focus()
      await page.waitForFunction(() => {
        const el = document.querySelector('section[data-fixture="Tooltip-Top"] [role="tooltip"]')
        if (el === null) return false
        const style = getComputedStyle(el)
        return style.display !== 'none' && style.visibility === 'visible' && style.opacity === '1'
      })

      // Accessibility on the fallback path.
      await page.addScriptTag({ content: axeSource })
      const violations = await page.evaluate(async () => {
        const axe = (window as unknown as { axe: { run: (ctx: Element) => Promise<{ violations: unknown[] }> } }).axe
        const names = [
          'Popover-Content',
          'PopoverTrigger-Default',
          'DropdownMenu-Menu',
          'DropdownMenuTrigger-Default',
          'Tooltip-Top',
          'Tooltip-Right',
          'HoverCard-User',
        ]
        const out: unknown[] = []
        for (const n of names) {
          const section = document.querySelector(`section[data-fixture="${n}"]`)
          if (section) out.push(...(await axe.run(section)).violations)
        }
        return out
      })
      const serious = (violations as { impact: string | null; id: string }[]).filter(
        (v) => v.impact === 'serious' || v.impact === 'critical',
      )
      expect(JSON.stringify(serious)).toBe('[]')
    } finally {
      await browser.close()
    }
  }, 180000)

  // The measured placement replaces the CSS flip, which hangs WebKit 26.0 on
  // an element with a transition. The page must answer after each step.
  test(`REQ-REG-07 ${name}: a menu flips at the viewport edge and a sub-menu follows its keys`, async () => {
    const browser = await engine.launch({ headless: true })
    const page = await browser.newPage()
    try {
      await page.goto(shop.url + '/_gx/gallery')
      await page.waitForSelector('section[data-fixture]')
      const rect = (selector: string) =>
        page.evaluate((sel) => {
          const el = document.querySelector(sel)!
          const r = el.getBoundingClientRect()
          return { top: r.top, bottom: r.bottom, left: r.left, right: r.right, side: el.getAttribute('data-side') }
        }, selector)
      const settled = (selector: string) =>
        page.waitForFunction((sel) => {
          const el = document.querySelector(sel)
          return el?.matches(':popover-open') && getComputedStyle(el).scale === '1' && getComputedStyle(el).opacity === '1'
        }, selector)
      const focused = (text: string) =>
        page.waitForFunction((want) => (document.activeElement?.textContent ?? '').trim().startsWith(want), text)

      const name = 'section[data-fixture="DropdownMenuTrigger-Sub"] button'
      const trigger = page.locator(name)
      await trigger.evaluate((el) => window.scrollBy(0, el.getBoundingClientRect().bottom - document.documentElement.clientHeight + 12))
      await trigger.focus()
      await page.keyboard.press('ArrowDown')
      await settled('#demo-dropdown-sub')
      const at = await rect(name)
      const menu = await rect('#demo-dropdown-sub')
      expect(menu.side).toBe('top')
      expect(Math.round(at.top - menu.bottom)).toBe(4)
      expect(menu.top).toBeGreaterThanOrEqual(8)

      // The sub-menu opens beside its trigger and returns focus when it closes.
      await focused('New tab')
      await page.keyboard.press('ArrowDown')
      await focused('More tools')
      await page.keyboard.press('ArrowRight')
      const sub = '#demo-dropdown-sub [data-gx-sub] > [popover]'
      await settled(sub)
      await focused('Save page')
      const item = await rect('#demo-dropdown-sub [data-gx-sub] > button')
      const content = await rect(sub)
      expect(content.side).toBe('right')
      expect(Math.round(content.left)).toBe(Math.round(item.right))
      await page.keyboard.press('ArrowLeft')
      await page.waitForFunction((sel) => !document.querySelector(sel)?.matches(':popover-open'), sub)
      await focused('More tools')
      await page.keyboard.press('ArrowRight')
      await focused('Save page')
      await page.keyboard.press('Enter')
      await page.waitForFunction(() => !document.querySelector('#demo-dropdown-sub')?.matches(':popover-open'))
      await focused('Open with a sub-menu')
      // The closed menu leaves the rendering, and the page still answers.
      await page.waitForFunction(() => getComputedStyle(document.querySelector('#demo-dropdown-sub')!).display === 'none')
      expect(await page.evaluate(() => 1 + 1)).toBe(2)
    } finally {
      await browser.close()
    }
  }, 180000)
}
