// The Gx browser runtime. `just runtime` builds this file to gx.js, which
// package gx embeds. Keep it free of bare imports: users never run a
// bundler for the core runtime.
//
// The functions here are the JavaScript halves of the typed helpers the
// compiler allows in client expressions (REQ-ACT-13). Navigation and active
// links join this file with the morph runtime (REQ-RTE-12).

export const gx = {
  // len counts Unicode code points, like Go's utf8.RuneCountInString.
  len: (s: string): number => [...s].length,
  // at returns the code point at a rune index, like gxc.At.
  at: (s: string, i: number): string => [...s][i] ?? "",
  // contains and index mirror gxc.Contains and gxc.Index.
  contains: (s: string, sub: string): boolean => s.includes(sub),
  index: (s: string, sub: string): number => {
    const i = s.indexOf(sub)
    return i < 0 ? -1 : [...s.slice(0, i)].length
  },
}

;(globalThis as { __gx?: typeof gx }).__gx = gx

// In dev, two signal instances that share a scope are a bug: a patch would
// reach both. The compiler reports what it can see (GX2012); this catches
// the rest.
const checkInstances = (): void => {
  if (!document.querySelector('meta[name="gx-dev"]')) return
  const seen = new Set<string>()
  document.querySelectorAll('[data-gx-instance]').forEach((el) => {
    const id = el.getAttribute('data-gx-instance') ?? ''
    if (id === '') return
    if (seen.has(id)) {
      console.error(`gx: two signal instances share the scope ${id}; add key={...}`)
    }
    seen.add(id)
  })
}

document.addEventListener('DOMContentLoaded', checkInstances)
checkInstances()
new MutationObserver(checkInstances).observe(document.documentElement, {
  subtree: true,
  childList: true,
})
