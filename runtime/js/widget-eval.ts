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
      return ev(a) < ev(b)
    case '<=':
      return ev(a) <= ev(b)
    case '>':
      return ev(a) > ev(b)
    case '>=':
      return ev(a) >= ev(b)
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
