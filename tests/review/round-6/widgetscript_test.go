package round6_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// repoRoot returns the root of the gx module.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// runBun writes a script to a temp directory and runs it in Bun. The text
// %WIDGET% in the script is the path of runtime/js/widget.ts of the
// repository, and %ELEMENT% is the path of runtime/js/widget-element.ts.
func runBun(t *testing.T, script string, env ...string) string {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatalf("bun is not on PATH: %v", err)
	}
	dir := t.TempDir()
	js := filepath.Join(repoRoot(t), "runtime", "js")
	script = strings.ReplaceAll(script, "%WIDGET%", filepath.ToSlash(filepath.Join(js, "widget.ts")))
	script = strings.ReplaceAll(script, "%ELEMENT%", filepath.ToSlash(filepath.Join(js, "widget-element.ts")))
	script = strings.ReplaceAll(script, "%EVAL%", filepath.ToSlash(filepath.Join(js, "widget-eval.ts")))
	script = strings.ReplaceAll(script, "%DIR%", filepath.ToSlash(dir))
	path := filepath.Join(dir, "script.mjs")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bun", "run", path)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bun: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// radioScript mounts the widget script of the repository on a small stand-in
// for a shadow root with three radio buttons of one group. Each has the
// data-gx-bind attribute that the server writes for bind:value={$Size}.
const radioScript = `
import { mount } from "%WIDGET%"

class HTMLInputElement {
  constructor(type, value, attrs) {
    this.type = type
    this.value = value
    this.checked = false
    this.attrs = new Map(Object.entries(attrs))
    this.listeners = new Map()
    this.style = { display: '', setProperty() {}, removeProperty() {} }
    this.children = []
  }
  get attributes() { return [...this.attrs].map(([name, value]) => ({ name, value })) }
  getAttribute(name) { return this.attrs.has(name) ? this.attrs.get(name) : null }
  setAttribute(name, value) { this.attrs.set(name, String(value)) }
  hasAttribute(name) { return this.attrs.has(name) }
  querySelectorAll() { return [] }
  querySelector() { return null }
  addEventListener(name, fn) { this.listeners.set(name, [...(this.listeners.get(name) ?? []), fn]) }
  removeEventListener() {}
  fire(name) { for (const fn of this.listeners.get(name) ?? []) fn({ type: name, target: this }) }
  remove() {}
}
globalThis.HTMLInputElement = HTMLInputElement

const plain = () => {
  const attrs = new Map()
  return {
    attrs,
    style: { display: '', setProperty() {}, removeProperty() {} },
    children: [],
    get attributes() { return [...attrs].map(([name, value]) => ({ name, value })) },
    getAttribute: (name) => (attrs.has(name) ? attrs.get(name) : null),
    setAttribute: (name, value) => void attrs.set(name, String(value)),
    hasAttribute: (name) => attrs.has(name),
    querySelectorAll: () => [],
    querySelector: () => null,
    addEventListener() {},
    removeEventListener() {},
    remove() {},
    set innerHTML(_) {},
  }
}
const holder = plain()
holder.setAttribute('data-gx-signals', process.env.GX_SIGNALS)
const bind = process.env.GX_BIND
const radios = ['s', 'm', 'l'].map((value) => new HTMLInputElement('radio', value, { type: 'radio', name: 'size', value, 'data-gx-bind': bind }))
globalThis.document = { createElement: plain }
const root = {
  adoptedStyleSheets: [],
  children: [],
  addEventListener() {},
  replaceChildren() {},
  querySelector: () => null,
  querySelectorAll: (selector) => (selector === '*' ? [holder, ...radios] : []),
}
const host = { server: 'https://api.example', request: async () => new Response(null, { status: 204 }), fail() {}, event: () => true, reload() {} }
await mount(root, { tag: 'acme-cart', html: '<div></div>', build: 'b1' }, host)
await new Promise((done) => setTimeout(done, 0))
const first = { values: radios.map((r) => r.value), checked: radios.map((r) => r.checked) }
// The user chooses the radio button "l".
radios[1].checked = false
radios[2].checked = true
radios[2].fire('input')
radios[2].fire('change')
await new Promise((done) => setTimeout(done, 0))
console.log(JSON.stringify({ first, signal: root.gxSignals.getPath(bind), values: radios.map((r) => r.value), checked: radios.map((r) => r.checked) }))
`

// TestREQ_ISL_20_BindOnARadioGroupInAWidget checks bind: on a radio group
// inside a widget (REQ-ISL-20: "Inside a widget these work as on a Gx page:
// signals, client expressions, ..."; SDD 6.1: bind:value two-way binds an
// input to a signal). On a Gx page the adapter checks the radio button whose
// value is the value of the signal, and a choice of the user writes the
// value of that button to the signal. The widget script writes the value of
// the signal into the value of each radio button of the group: each button
// then has one value, no button is checked, and each choice of the user
// gives the signal the first value again.
func TestREQ_ISL_20_BindOnARadioGroupInAWidget(t *testing.T) {
	signals := gx.SignalJSON("cart.Cart", "", map[string]any{"size": "m"})
	bind := gx.SignalName("cart.Cart", "", "size")
	got := runBun(t, radioScript, "GX_SIGNALS="+signals, "GX_BIND="+bind)
	want := `{"first":{"values":["s","m","l"],"checked":[false,true,false]},"signal":"l","values":["s","m","l"],"checked":[false,false,true]}`
	if got != want {
		t.Errorf("a radio group with bind:value={$Size} and $Size = \"m\" in a widget:\n got %s\nwant %s\nthe widget script writes the signal into the value of each radio button; a page under Datastar checks the button with the value of the signal", got, want)
	}
}
