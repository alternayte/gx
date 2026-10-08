package round5_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx"
)

// protoScript mounts the widget script of the repository on a small stand-in
// for a shadow root. The root has one element with the data-gx-signals
// attribute that the server wrote. The script prints what a plain object of
// the page holds after the mount.
const protoScript = `
import { mount } from %q

const signals = process.env.GX_SIGNALS
const element = () => {
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
const withSignals = element()
withSignals.setAttribute('data-gx-signals', signals)
globalThis.document = { createElement: element }
const root = {
  adoptedStyleSheets: [],
  children: [],
  addEventListener() {},
  replaceChildren() {},
  querySelector: () => null,
  querySelectorAll: (selector) => (selector === '*' ? [withSignals] : []),
}
const host = { server: 'https://api.example', request: async () => new Response(null, { status: 204 }), fail() {}, event: () => true, reload() {} }
await mount(root, { tag: 'acme-cart', html: '<div></div>', build: 'b1' }, host)
// The store reads a path that no component declared.
const other = root.gxSignals.getPath('qty')
console.log(JSON.stringify({ plain: ({}).qty ?? null, own: Object.prototype.hasOwnProperty.call(Object.prototype, 'qty'), other: other ?? null }))
`

// TestREQ_ISL_21_SignalKeyCannotReachObjectPrototype checks that the signals
// of a widget stay in the store of that widget for each instance key
// (REQ-ISL-21: "Signals of a widget are not visible to a Datastar of the
// host, to a different widget, or to a second instance of the same widget";
// SI-15: "A server value in an expression is data, not code"). The key of a
// component instance is a value of the app, for example the slug of a row.
// With the key "__proto__" the store of the widget script writes the
// signals of the instance to Object.prototype of the host page.
func TestREQ_ISL_21_SignalKeyCannotReachObjectPrototype(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatalf("bun is not on PATH: %v", err)
	}
	// The attribute as the server writes it for the instance key
	// "__proto__" of the component cart.Cart.
	signals := gx.SignalJSON("cart.Cart", gx.InstanceKey("__proto__"), map[string]any{"qty": 7})
	if !strings.Contains(signals, `"__proto__":{"qty":7}`) {
		t.Fatalf("the fixture is wrong: the server writes %s", signals)
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "proto.mjs")
	widget := filepath.Join(repoRoot(t), "runtime", "js", "widget.ts")
	if err := os.WriteFile(script, []byte(strings.Replace(protoScript, "%q", `"`+filepath.ToSlash(widget)+`"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bun", "run", script)
	cmd.Env = append(os.Environ(), "GX_SIGNALS="+signals)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bun: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	if want := `{"plain":null,"own":false,"other":null}`; got != want {
		t.Errorf("after the mount of a widget whose instance key is __proto__, the page has %s; want %s: the signal qty is on Object.prototype, so each object of the host page and each other widget has it", got, want)
	}
}
