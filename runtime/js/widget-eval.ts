// The evaluator of client expressions in a widget (SI-15). A widget gets
// each client expression as data: a small tree that the Gx server writes.
// This file gives a tree its value. No text is run as code, so a host page
// needs no 'unsafe-eval' in its policy.
//
// Each node has the result of the same expression in Go (REQ-ACT-13,
// DR-05). `internal/compiler` tests this file against Go on the trees that
// the compiler writes.

export type Tree = unknown[]

// read returns the value of a signal by its path.
export type Read = (path: string[]) => unknown

// The helpers of package gxc. They count code points, as Go counts runes.
const helpers: Record<string, (...args: never[]) => unknown> = {
  len: (s: string): number => [...s].length,
  at: (s: string, i: number): string => (i < 0 ? '' : ([...s][i] ?? '')),
  contains: (s: string, sub: string): boolean => s.includes(sub),
  index: (s: string, sub: string): number => {
    const i = s.indexOf(sub)
    return i < 0 ? -1 : [...s.slice(0, i)].length
  },
}

// order compares two values as Go does (DR-05): a negative number when a is
// before b, zero when they are equal. Go orders two strings by their UTF-8
// bytes, which is the order of their code points. JavaScript orders by
// UTF-16 code units, and the two orders differ for a character above U+FFFF.
const order = (a: unknown, b: unknown): number => {
  if (typeof a !== 'string' || typeof b !== 'string') {
    const x = a as number
    const y = b as number
    return x < y ? -1 : x > y ? 1 : x === y ? 0 : NaN
  }
  const left = a[Symbol.iterator]()
  const right = b[Symbol.iterator]()
  for (;;) {
    const l = left.next()
    const r = right.next()
    if (l.done || r.done) return l.done ? (r.done ? 0 : -1) : 1
    const diff = (l.value.codePointAt(0) as number) - (r.value.codePointAt(0) as number)
    if (diff !== 0) return diff
  }
}

// evaluate returns the value of a tree node.
export const evaluate = (node: Tree, read: Read): unknown => {
  const ev = (part: unknown): never => evaluate(part as Tree, read) as never
  const [op, a, b] = node
  switch (op) {
    case 'v':
      return a
    case 's':
      return read(a as string[])
    case '!':
      return !ev(a)
    case 'neg':
      return -ev(a)
    case 'pos':
      return +ev(a)
    case '&&':
      return ev(a) && ev(b)
    case '||':
      return ev(a) || ev(b)
    case '==':
      return ev(a) === ev(b)
    case '!=':
      return ev(a) !== ev(b)
    case '<':
      return order(ev(a), ev(b)) < 0
    case '<=':
      return order(ev(a), ev(b)) <= 0
    case '>':
      return order(ev(a), ev(b)) > 0
    case '>=':
      return order(ev(a), ev(b)) >= 0
    case '+':
      // The sum of two numbers, or two strings joined: Go has one type on
      // each side.
      return (ev(a) as number) + (ev(b) as number)
    case '-':
      return ev(a) - ev(b)
    case '*':
      return ev(a) * ev(b)
    case '/':
      return ev(a) / ev(b)
    case '%':
      return ev(a) % ev(b)
    case 'idiv':
      // The division of two integers cuts the fraction, as in Go.
      return Math.trunc(ev(a) / ev(b))
  }
  const helper = helpers[op as string]
  if (helper) return helper(...(node.slice(1).map(ev) as never[]))
  throw new Error('gx: the widget script does not know the expression node ' + String(op))
}
