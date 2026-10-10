// The JSON wire of the shop (M24): the TypeScript client that gx api wrote
// calls the actions with .API() of a running shop.
import { afterAll, beforeAll, expect, test } from 'bun:test'
import { createClient, GxAPIError } from '../../examples/shop/api/client'
import { startShop, type Shop } from './harness'

let shop: Shop
let client: ReturnType<typeof createClient>

beforeAll(async () => {
  shop = await startShop()
  client = createClient({ baseURL: shop.url })
}), 180000

afterAll(() => {
  shop?.stop()
})

test('REQ-ACT-20 the client calls an action and gets the typed result', async () => {
  const before = await client.basketCount()
  expect(before.lines.map((l) => l.sku)).toEqual(['tea', 'milk', 'rice'])
  const after = await client.basketSetQty({ sku: 'milk', qty: 7 })
  expect(after.lines.find((l) => l.sku === 'milk')?.qty).toBe(7)
  const milkBefore = before.lines.find((l) => l.sku === 'milk')?.qty ?? 0
  expect(after.items).toBe(before.items - milkBefore + 7)
  // The page of a browser shows the same data.
  const page = await (await fetch(shop.url + '/basket')).text()
  expect(page).toContain('<b>7</b>')
})

test('REQ-ACT-19 a rule failure is status 422 with the field and the key', async () => {
  let err: unknown
  try {
    await client.basketSetQty({ sku: 'milk', qty: 500 })
  } catch (e) {
    err = e
  }
  expect(err).toBeInstanceOf(GxAPIError)
  const failed = err as GxAPIError
  expect(failed.status).toBe(422)
  expect(failed.errors.length).toBe(1)
  expect(failed.errors[0].field).toBe('qty')
  expect(failed.errors[0].key).not.toBe('')
})

test('REQ-ACT-19 an error of the handler has its status', async () => {
  let err: unknown
  try {
    await client.basketSetQty({ sku: 'no-such-line', qty: 2 })
  } catch (e) {
    err = e
  }
  expect((err as GxAPIError).status).toBe(404)
})

test('REQ-ACT-19 an action with no .API() answers 406 to a JSON call', async () => {
  const res = await fetch(shop.url + '/basket/reset', { method: 'POST', headers: { Accept: 'application/json' } })
  expect(res.status).toBe(406)
  const body = (await res.json()) as { errors: { key: string }[] }
  expect(body.errors[0].key).toBe('gx.not_api')
})

test('SI-16 a JSON call with no cookie needs no token, and one with a cookie does', async () => {
  // This client has no cookie and no token; a browser with no Fetch
  // Metadata names its origin.
  const call = (headers: Record<string, string>): Promise<Response> =>
    fetch(shop.url + '/basket/qty/tea', {
      method: 'POST',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json', Origin: shop.url, ...headers },
      body: JSON.stringify({ qty: 2 }),
    })
  expect((await call({})).status).toBe(200)
  expect((await call({ Cookie: 'gx_csrf=abc' })).status).toBe(403)
  expect((await call({ Cookie: 'gx_csrf=abc', 'Gx-CSRF': 'abc' })).status).toBe(200)
})
