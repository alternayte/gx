// Accessibility of every registry fixture, audited with axe-core on the dev
// gallery (REQ-REG-09, NFR-09).
import { afterAll, afterEach, beforeAll, beforeEach, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import { type Browser, type Page } from 'playwright-core'
import { launchBrowser, startShop, type Shop } from './harness'

const axeSource = readFileSync(new URL('./node_modules/axe-core/axe.min.js', import.meta.url), 'utf8')

type AxeNode = { target: string[] }
type AxeViolation = { id: string; impact: string | null; help: string; nodes: AxeNode[] }

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
  // Audit settled styles: a running color transition would report the
  // transition start value against the new theme.
  await page.addStyleTag({ content: '*,*::before,*::after{transition:none!important;animation:none!important}' })
  await page.addScriptTag({ content: axeSource })
})

afterEach(async () => {
  try {
    await page?.close()
  } catch {
    // already closed
  }
})

// audit runs axe on every registry fixture section and returns the serious
// and critical findings. The gallery chrome and the shop's own missing
// sections stay out of the audit.
async function audit(): Promise<AxeViolation[]> {
  const violations = await page.evaluate(async () => {
    const axe = (window as unknown as { axe: { run: (ctx: Element) => Promise<{ violations: unknown[] }> } }).axe
    const sections = [...document.querySelectorAll('section.fixture:not(.missing)[data-package*="/ui/"]')]
    const out: unknown[] = []
    for (const section of sections) {
      const result = await axe.run(section)
      out.push(...result.violations)
    }
    return out
  })
  return (violations as AxeViolation[]).filter((v) => v.impact === 'serious' || v.impact === 'critical')
}

// report renders the findings with the fixture that owns each node.
function report(violations: AxeViolation[]): string {
  return violations
    .map((v) => {
      const nodes = v.nodes.map((n) => n.target.join(' ')).join('\n    ')
      return `${v.id} (${v.impact}): ${v.help}\n    ${nodes}`
    })
    .join('\n')
}

test('REQ-REG-09 the gallery has no serious axe violations in the light theme', async () => {
  const violations = await audit()
  expect(report(violations)).toBe('')
})

test('NFR-09 the gallery has no serious axe violations in the dark theme', async () => {
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  const violations = await audit()
  expect(report(violations)).toBe('')
})
