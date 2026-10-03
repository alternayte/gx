// The runtime CSRF wrapper (SI-03). A browser outside a secure context
// sends no Fetch Metadata, so the token must ride on every write.
import { expect, test } from 'bun:test'
import { csrfInit } from '../../runtime/js/csrf'

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
