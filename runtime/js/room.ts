// Shared signals (REQ-ACT-21). The root of a component with shared signals
// has an effect that calls share with the value of each shared signal, at
// the start and after each change. The runtime writes a changed value to the
// room of the component, and the stream of the room gives each viewer the
// values of the room, the writer too.

export type Values = Record<string, unknown>

// Deps are the parts of the browser that a room uses. A test gives its own.
export interface Deps {
  // open reads the stream of a room and returns the function that ends it.
  open: (url: string, receive: (values: Values) => void) => () => void
  // post writes values to the room; it resolves to false for a refusal.
  post: (url: string, room: string, signals: Values) => Promise<boolean>
  // patch puts values into the signals of the page, under a scope.
  patch: (scope: string[], values: Values) => void
  // later runs fn after the writes of this moment.
  later: (fn: () => void) => void
}

interface Room {
  url: string
  room: string
  scope: string[]
  // seen holds the JSON of the last value of each signal that the page had;
  // server holds the last value that the room gave or that the page had at
  // the start.
  seen: Record<string, string>
  server: Record<string, string>
  queue: Values | undefined
  close: () => void
}

const rooms = new WeakMap<Element, Room>()

// share is the call of the effect. The first call of an element joins the
// room; a later call writes each value that changed in the page.
export const share = (el: Element, values: Values, deps: Deps): void => {
  let state = rooms.get(el)
  if (!state) {
    const url = el.getAttribute('data-gx-room-url') ?? ''
    const room = el.getAttribute('data-gx-room') ?? ''
    if (url === '' || room === '') return
    const seen: Record<string, string> = {}
    for (const [name, value] of Object.entries(values)) seen[name] = JSON.stringify(value)
    const joined: Room = {
      url,
      room,
      scope: (el.getAttribute('data-gx-instance') ?? '').split('.').filter((part) => part !== ''),
      seen,
      server: { ...seen },
      queue: undefined,
      close: () => {},
    }
    joined.close = deps.open(url + '?room=' + encodeURIComponent(room), (incoming) => {
      if (!el.isConnected) {
        // The component left the page: the viewer leaves the room.
        joined.close()
        rooms.delete(el)
        return
      }
      const changed: Values = {}
      for (const [name, value] of Object.entries(incoming)) {
        const text = JSON.stringify(value)
        joined.server[name] = text
        if (joined.seen[name] !== text) {
          joined.seen[name] = text
          changed[name] = value
        }
      }
      if (Object.keys(changed).length > 0) deps.patch(joined.scope, changed)
    })
    rooms.set(el, joined)
    return
  }
  const room = state
  for (const [name, value] of Object.entries(values)) {
    const text = JSON.stringify(value)
    if (room.seen[name] === text) continue
    room.seen[name] = text
    if (room.queue === undefined) {
      room.queue = {}
      // The changes of one moment go in one request.
      deps.later(() => {
        const signals = room.queue ?? {}
        room.queue = undefined
        void deps.post(room.url, room.room, signals).then((ok) => {
          if (ok) return
          // The room refused the values: the page shows the values of
          // the room again.
          const back: Values = {}
          for (const name of Object.keys(signals)) {
            const text = room.server[name]
            if (text !== undefined && room.seen[name] !== text) {
              room.seen[name] = text
              back[name] = JSON.parse(text)
            }
          }
          if (Object.keys(back).length > 0) deps.patch(room.scope, back)
        })
      })
    }
    room.queue[name] = value
  }
}
