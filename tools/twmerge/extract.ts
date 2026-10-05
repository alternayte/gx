// Extracts the default-config cases of the tailwind-merge test suite: every
// expect(twMerge(...)).toBe(...) whose arguments are plain strings.
import { cpSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const [testsDir, srcDir, outFile] = process.argv.slice(2).map((p) => resolve(p))
const work = resolve('.extract-work')
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })

writeFileSync(
  join(work, '_src.ts'),
  `export * from ${JSON.stringify(join(srcDir, 'index.ts'))}
import { twMerge as real } from ${JSON.stringify(join(srcDir, 'index.ts'))}
export const calls: { args: unknown[]; result: string; used: boolean }[] = []
export const twMerge = (...args: unknown[]) => {
  const result = (real as (...a: unknown[]) => string)(...args)
  calls.push({ args, result, used: false })
  return result
}
`,
)
writeFileSync(
  join(work, '_vitest.ts'),
  `import { calls } from './_src'
export const cases: { file: string; test: string; args: unknown[]; want: string }[] = []
export const skipped: { file: string; test: string; reason: string }[] = []
export const state = { file: '', test: '' }
const run = (name: string, fn: () => unknown) => { const prev = state.test; state.test = name; try { fn() } finally { state.test = prev } }
export const test: any = run
export const it: any = run
export const describe: any = (name: string, fn: () => unknown) => fn()
test.each = () => () => {}
it.each = () => () => {}
test.skip = () => {}
it.skip = () => {}
export const bench: any = () => {}
export const expectTypeOf: any = () => new Proxy(() => {}, { get: () => () => expectTypeOf(), apply: () => expectTypeOf() })
const noop: any = new Proxy(() => noop, { get: () => noop, apply: () => noop })
export const vi: any = new Proxy(() => vi, { get: () => vi, apply: () => vi })
export const expect: any = (value: unknown) => {
  const call = calls.length > 0 ? calls[calls.length - 1] : undefined
  const fromMerge = call && !call.used && call.result === value
  if (fromMerge) call.used = true
  return new Proxy({}, {
    get: (_t, prop) => {
      if (prop === 'toBe' && fromMerge) {
        return (want: unknown) => {
          const flat = call!.args.every((a) => typeof a === 'string')
          if (typeof want !== 'string') skipped.push({ file: state.file, test: state.test, reason: 'the expected value is not a string' })
          else if (!flat) skipped.push({ file: state.file, test: state.test, reason: 'an argument is not a string' })
          else cases.push({ file: state.file, test: state.test, args: call!.args, want })
          return noop
        }
      }
      return noop
    },
  })
}
expect.assertions = () => {}
expect.any = () => ({})
expect.anything = () => ({})
expect.objectContaining = () => ({})
expect.arrayContaining = () => ({})
`,
)

// These files test how the library starts, with mocks of the test runner.
// They hold no merge expectation.
const internals = new Set(['lazy-initialization.test.ts'])
const files = readdirSync(testsDir)
  .filter((f) => f.endsWith('.test.ts') && !internals.has(f))
  .sort()
const failed: string[] = []
for (const file of files) {
  const text = readFileSync(join(testsDir, file), 'utf8')
    .replaceAll(`from 'vitest'`, `from './_vitest'`)
    .replaceAll(`from '../src'`, `from './_src'`)
    .replaceAll(`from '../src/`, `from '${srcDir}/`)
  writeFileSync(join(work, file), text)
}
const vt = await import(join(work, '_vitest.ts'))
for (const file of files) {
  vt.state.file = file
  try {
    await import(join(work, file))
  } catch (err) {
    failed.push(`${file}: ${String(err).split('\n')[0]}`)
  }
}
// docs-examples.test.ts reads the examples of the README and the docs with
// this pattern and expects 63 of them. It needs a glob package, so the same
// scan runs here.
const exampleRe =
  /twMerge\((?<arguments>[\w\s\-:[\]#(),!&%\n'"]+?)\)(?!.*(?<!\/\/.*)')\s*\n?\s*\/\/\s*→\s*['"](?<result>.+)['"]/g
const pkgRoot = resolve(testsDir, '..')
const docFiles = [join(pkgRoot, 'README.md')]
const walkDocs = (dir: string) => {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) walkDocs(join(dir, entry.name))
    else if (entry.name.endsWith('.md')) docFiles.push(join(dir, entry.name))
  }
}
walkDocs(join(pkgRoot, 'docs'))
let docExamples = 0
for (const file of docFiles.sort()) {
  for (const match of readFileSync(file, 'utf8').matchAll(exampleRe)) {
    docExamples++
    // eslint-disable-next-line no-eval
    const args = (0, eval)(`[${match.groups!.arguments}]`) as unknown[]
    if (args.every((a) => typeof a === 'string')) {
      vt.cases.push({ file: 'docs-examples.test.ts', test: 'docs examples', args, want: match.groups!.result })
    } else vt.skipped.push({ file: 'docs-examples.test.ts', test: 'docs examples', reason: 'an argument is not a string' })
  }
}
if (docExamples !== 63) failed.push(`docs-examples.test.ts: found ${docExamples} examples, the test expects 63`)
const byFile: Record<string, number> = {}
for (const c of vt.cases) byFile[c.file] = (byFile[c.file] ?? 0) + 1
writeFileSync(outFile, JSON.stringify({ cases: vt.cases }, null, 1) + '\n')
console.log(JSON.stringify({ cases: vt.cases.length, byFile, skipped: vt.skipped, failed }, null, 1))
// Self-check: the real twMerge gives each expected value.
const { twMerge } = await import(join(srcDir, 'index.ts'))
let bad = 0
for (const c of vt.cases) if (twMerge(...(c.args as string[])) !== c.want) bad++
console.log('cases the real twMerge fails:', bad)
rmSync(work, { recursive: true, force: true })
if (failed.length > 0 || bad > 0) {
  console.error('the extraction is not complete')
  process.exit(1)
}
