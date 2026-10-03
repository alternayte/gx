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
  // at returns the code point at a rune index, like a Go []rune index.
  at: (s: string, i: number): string | undefined => [...s][i],
}

;(globalThis as { __gx?: typeof gx }).__gx = gx
