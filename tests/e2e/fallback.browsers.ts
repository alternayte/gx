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
}
