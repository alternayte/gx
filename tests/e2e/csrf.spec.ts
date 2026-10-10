// The runtime CSRF wrapper (SI-03). A browser outside a secure context
// sends no Fetch Metadata, so the token must ride on every write.
import { expect, test } from 'bun:test'
import { csrfInit, fragmentHashes, fragmentsInit } from '../../runtime/js/csrf'

test('SI-03 csrfInit adds the token to a same-origin POST', () => {
  const init = csrfInit('http://host/cart/add', { method: 'POST' }, 'http://host', 'tok')
  expect(new Headers(init?.headers).get('Gx-CSRF')).toBe('tok')
})

test('SI-03 csrfInit leaves reads and cross-origin writes alone', () => {
  const get = csrfInit('/x', { method: 'GET' }, 'http://host', 'tok')
  expect(new Headers(get?.headers).has('Gx-CSRF')).toBe(false)
  const cross = csrfInit('http://evil.example/x', { method: 'POST' }, 'http://host', 'tok')
  expect(new Headers(cross?.headers).has('Gx-CSRF')).toBe(false)
})

test('SI-03 csrfInit without a token changes nothing', () => {
  const init = { method: 'POST' }
  expect(csrfInit('/x', init, 'http://host', '')).toBe(init)
})

// The fragment hashes of a write (REQ-ACT-15).

const fakeRoot = (els: { id: string; hash: string }[]): ParentNode =>
  ({
    querySelectorAll: () => els.map((e) => ({ id: e.id, getAttribute: () => e.hash })),
  }) as unknown as ParentNode

test('REQ-ACT-15 fragmentHashes lists each fragment of the page by id', () => {
  expect(fragmentHashes(fakeRoot([{ id: 'cart-total', hash: '0a1b2c3d' }, { id: 'cart-row-7', hash: 'ffffffff' }]))).toBe(
    'cart-total=0a1b2c3d,cart-row-7=ffffffff',
  )
  expect(fragmentHashes(fakeRoot([]))).toBe('')
})

test('REQ-ACT-15 fragmentHashes sends nothing for a page over the header limit', () => {
  const many = Array.from({ length: 400 }, (_, i) => ({ id: `cart-row-${i}`, hash: '0a1b2c3d' }))
  expect(fragmentHashes(fakeRoot(many))).toBe('')
})

test('REQ-ACT-15 fragmentsInit adds the hashes to a same-origin write only', () => {
  const post = fragmentsInit('http://host/cart/add', { method: 'POST' }, 'http://host', 'a=1')
  expect(new Headers(post?.headers).get('Gx-Fragments')).toBe('a=1')
  const get = fragmentsInit('/x', { method: 'GET' }, 'http://host', 'a=1')
  expect(new Headers(get?.headers).has('Gx-Fragments')).toBe(false)
  const cross = fragmentsInit('http://other.example/x', { method: 'POST' }, 'http://host', 'a=1')
  expect(new Headers(cross?.headers).has('Gx-Fragments')).toBe(false)
  const init = { method: 'POST' }
  expect(fragmentsInit('/x', init, 'http://host', '')).toBe(init)
})
