// The runtime half of shared signals (REQ-ACT-21): which values go to the
// room, and which values of the room go to the page.
import { expect, test } from 'bun:test'
import { share, type Deps, type Values } from '../../runtime/js/room'

const element = (attrs: Record<string, string>): Element & { isConnected: boolean } =>
  ({ isConnected: true, getAttribute: (name: string) => attrs[name] ?? null }) as unknown as Element & { isConnected: boolean }

function harness(accept = true) {
  const posts: { url: string; room: string; signals: Values }[] = []
  const patches: { scope: string[]; values: Values }[] = []
  let receive: (values: Values) => void = () => {}
  let closed = false
  let opened = ''
  const timers: (() => void)[] = []
  const deps: Deps = {
    open: (url, fn) => {
      opened = url
      receive = fn
      return () => (closed = true)
    },
    post: (url, room, signals) => {
      posts.push({ url, room, signals })
      return Promise.resolve(accept)
    },
    patch: (scope, values) => void patches.push({ scope, values }),
    later: (fn) => void timers.push(fn),
  }
  const flush = async (): Promise<void> => {
    for (const fn of timers.splice(0)) fn()
    await Promise.resolve()
    await Promise.resolve()
  }
  return { deps, posts, patches, flush, receive: (v: Values) => receive(v), closed: () => closed, opened: () => opened }
}

const el = () => element({ 'data-gx-room-url': '/_gx/room/notes.Note', 'data-gx-room': 'a b', 'data-gx-instance': 'notes.Note.k1' })

test('REQ-ACT-21 the first call joins the room and writes nothing', async () => {
  const h = harness()
  share(el(), { typing: false }, h.deps)
  await h.flush()
  expect(h.opened()).toBe('/_gx/room/notes.Note?room=a%20b')
  expect(h.posts).toEqual([])
})

test('REQ-ACT-21 a change of the page goes to the room, one request for the changes of a moment', async () => {
  const h = harness()
  const root = el()
  share(root, { typing: false, title: 'New' }, h.deps)
  share(root, { typing: true, title: 'New' }, h.deps)
  share(root, { typing: true, title: 'Plan' }, h.deps)
  await h.flush()
  expect(h.posts).toEqual([{ url: '/_gx/room/notes.Note', room: 'a b', signals: { typing: true, title: 'Plan' } }])
  // The same values again are no change.
  share(root, { typing: true, title: 'Plan' }, h.deps)
  await h.flush()
  expect(h.posts.length).toBe(1)
})

test('REQ-ACT-21 a value of the room goes to the signals of the instance, and makes no write', async () => {
  const h = harness()
  const root = el()
  share(root, { typing: false }, h.deps)
  h.receive({ typing: true })
  expect(h.patches).toEqual([{ scope: ['notes', 'Note', 'k1'], values: { typing: true } }])
  // The effect runs after the patch, with the value of the room.
  share(root, { typing: true }, h.deps)
  await h.flush()
  expect(h.posts).toEqual([])
  // A value that the page has is no patch.
  h.receive({ typing: true })
  expect(h.patches.length).toBe(1)
})

test('REQ-ACT-21 a value that the room refuses goes back to the value of the room', async () => {
  const h = harness(false)
  const root = el()
  share(root, { title: 'New' }, h.deps)
  share(root, { title: 'a title that is too long' }, h.deps)
  await h.flush()
  expect(h.posts.length).toBe(1)
  expect(h.patches).toEqual([{ scope: ['notes', 'Note', 'k1'], values: { title: 'New' } }])
})

test('REQ-ACT-21 a component that left the page leaves the room', () => {
  const h = harness()
  const root = el()
  share(root, { typing: false }, h.deps)
  root.isConnected = false
  h.receive({ typing: true })
  expect(h.closed()).toBe(true)
  expect(h.patches).toEqual([])
})
