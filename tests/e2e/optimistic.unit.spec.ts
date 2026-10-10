// The runtime half of optimistic updates (REQ-ACT-18): the saved values and
// the answers that put them back.
import { expect, test } from 'bun:test'
import { failed, keep, mergeKept, take, watch, type Kept } from '../../runtime/js/optimistic'

const answer = (status: number, headers: Record<string, string> = {}): Response => new Response(null, { status, headers })

test('REQ-ACT-18 keep saves the first value of each signal for the request of the task', () => {
  let end = (): void => {}
  keep([{ cart: { count: 1 } }, { cart: { busy: false } }], (fn) => (end = fn))
  // A second directive of the same task does not replace a saved value.
  keep([{ cart: { count: 2 } }], () => {})
  expect(take()).toEqual({ cart: { count: 1, busy: false } })
  expect(take()).toBeUndefined()

  // With no request in the task, the values go.
  keep([{ cart: { count: 1 } }], (fn) => (end = fn))
  end()
  expect(take()).toBeUndefined()
})

test('REQ-ACT-18 mergeKept keeps a value that is saved', () => {
  expect(mergeKept({ a: { b: 1 } }, { a: { b: 2, c: 3 } })).toEqual({ a: { b: 1, c: 3 } })
})

test('REQ-ACT-18 an error status and the mark of an action error are failures', () => {
  expect(failed(answer(200))).toBe(false)
  expect(failed(answer(204))).toBe(false)
  expect(failed(answer(422))).toBe(true)
  expect(failed(answer(500))).toBe(true)
  expect(failed(answer(200, { 'Gx-Error': '1' }))).toBe(true)
})

test('REQ-ACT-18 watch puts the values back only when the request fails', async () => {
  const kept: Kept = { cart: { count: 1 } }
  const restored: Kept[] = []
  const restore = (k: Kept): void => void restored.push(k)

  await watch(Promise.resolve(answer(200)), kept, restore)
  expect(restored).toEqual([])
  await watch(Promise.resolve(answer(422)), kept, restore)
  expect(restored).toEqual([kept])
  await watch(Promise.resolve(answer(200, { 'Gx-Error': '1' })), kept, restore)
  expect(restored.length).toBe(2)
  // No network: the values go back, and the caller still gets the error.
  await expect(watch(Promise.reject(new Error('offline')), kept, restore)).rejects.toThrow('offline')
  expect(restored.length).toBe(3)
  // A request with no saved values is not watched.
  await watch(Promise.resolve(answer(500)), undefined, restore)
  expect(restored.length).toBe(3)
})
