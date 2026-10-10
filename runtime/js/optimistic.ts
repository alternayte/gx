// Optimistic updates (REQ-ACT-18). An optimistic directive gives the runtime
// the values of the signals that it changes, just before it changes them.
// The next request of the same task is the action of the element. When that
// request fails, the runtime puts the values back.

// Kept is the saved values, in the shape of the signals of the page.
export type Kept = Record<string, unknown>

let pending: Kept | undefined

// mergeKept merges src into dst. A value that dst has stays: the first saved
// value of a signal is its value before the update.
export const mergeKept = (dst: Kept, src: Kept): Kept => {
  for (const [key, value] of Object.entries(src)) {
    const have = dst[key]
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      dst[key] = mergeKept(have !== null && typeof have === 'object' ? (have as Kept) : {}, value as Kept)
    } else if (!(key in dst)) {
      dst[key] = value
    }
  }
  return dst
}

// keep saves the values for the request that follows in this task. A
// directive that no request follows saves nothing: the values go when the
// task ends.
export const keep = (parts: Kept[], later: (fn: () => void) => void = (fn) => setTimeout(fn, 0)): void => {
  const first = pending === undefined
  pending = parts.reduce(mergeKept, pending ?? {})
  if (first) later(() => (pending = undefined))
}

// take returns the saved values and forgets them. The request takes them.
export const take = (): Kept | undefined => {
  const kept = pending
  pending = undefined
  return kept
}

// failed reports whether an answer is a failure of the action: an error
// status, or the mark of an action error (the answer then has status 200,
// with a toast).
export const failed = (res: { ok: boolean; headers: { get(name: string): string | null } }): boolean =>
  !res.ok || res.headers.get('Gx-Error') !== null

// watch gives restore the saved values when the request fails: no answer,
// or an answer that failed says is a failure.
export const watch = (request: Promise<Response>, kept: Kept | undefined, restore: (kept: Kept) => void): Promise<Response> => {
  if (kept === undefined) return request
  return request.then(
    (res) => {
      if (failed(res)) restore(kept)
      return res
    },
    (err) => {
      restore(kept)
      throw err
    },
  )
}
