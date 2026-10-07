// Prints the TextMate scopes of every token of the corpus files
// (REQ-DEV-09). VS Code and the JetBrains plugin share this grammar. The
// embedded grammars of Go, CSS, JS and TS belong to the editor. Each one is
// an empty grammar here, so an embedded region has its region scope and no
// scope of the other language.
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { basename } from 'node:path'
import oniguruma from 'vscode-oniguruma'
import textmate from 'vscode-textmate'

const [grammarPath, ...files] = process.argv.slice(2)
const require = createRequire(import.meta.url)
const wasm = readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'))
await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength))

const registry = new textmate.Registry({
  onigLib: Promise.resolve({
    createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
    createOnigString: (s) => new oniguruma.OnigString(s),
  }),
  loadGrammar: async (scope) =>
    scope === 'source.gx'
      ? textmate.parseRawGrammar(readFileSync(grammarPath, 'utf8'), grammarPath)
      : { scopeName: scope, patterns: [{ match: '(?!)', name: 'never' }], repository: {} },
})
const grammar = await registry.loadGrammar('source.gx')

for (const file of files) {
  console.log(`== ${basename(file)}`)
  let stack = textmate.INITIAL
  const lines = readFileSync(file, 'utf8').split('\n')
  for (const [i, line] of lines.entries()) {
    const result = grammar.tokenizeLine(line, stack)
    stack = result.ruleStack
    for (const token of result.tokens) {
      const text = line.slice(token.startIndex, token.endIndex)
      if (text.trim() === '') continue
      const scopes = token.scopes.filter((s) => s !== 'source.gx')
      console.log(`${i + 1}:${token.startIndex + 1} ${JSON.stringify(text)} ${scopes.join(' ') || '-'}`)
    }
  }
}
