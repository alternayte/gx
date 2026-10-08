package round6_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/widgetelement"
)

// scratchModule returns a temp module with the files. The module replaces
// gx with the repository under review.
func scratchModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	all := map[string]string{
		"go.mod": "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n",
	}
	for rel, content := range files {
		all[rel] = content
	}
	for rel, content := range all {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// cartModule returns the files of an app with the widget acme-cart. field is
// the one attribute field of the widget input, as Go source.
func cartModule(field string) map[string]string {
	return map[string]string{
		"cart/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"type Widget struct {\n\tgx.Route `GET /cart`\n\t" + field + "\n}\n\n" +
			"type Add struct {\n\tgx.Route `POST /cart/add`\n}\n",
		"cart/Cart.gx": "package cart\n\nimport \"app/cart/route\"\n\nprops {\n  Size int\n}\n\n<section>\n  <p>{p.Size}</p>\n  <button on:click={route.Add{}}>Add</button>\n</section>\n",
		"cart/cart.go": "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"type ChangedDetail struct {\n\tCount int `json:\"count\"`\n}\n\n" +
			"var Changed = gx.Event[ChangedDetail](\"cart-changed\")\n\n" +
			"var CartWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {\n\treturn CartProps{Size: in.PageSize}, nil\n}, Cart).Tag(\"acme-cart\")\n\n" +
			"var add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\n" +
			"var Routes = gx.Collect(CartWidget, add)\n",
		"main.go": "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" +
			"\tapp.Group(\"/widgets\", gx.AllowOrigins(\"https://shop.example.com\"), cart.Routes)\n\t_ = app\n}\n",
	}
}

// browserScript is a stand-in for the parts of a browser that the element
// file and the widget script use. It follows two rules of the DOM standard
// for an element of an HTML page: setAttribute writes the name of an
// attribute in lower case, and attributeChangedCallback runs for a name
// that observedAttributes holds with the same letters.
//
// The script loads runtime/js/widget-element.ts of the repository with the
// configuration of GX_CONFIG. The Gx server is the temp directory: its first
// answer names the script /runtime.mjs of that directory and the stylesheet
// /cart.css. The stylesheet arrives when the test calls releaseSheet.
const browserScript = `
const log = []
const defined = new Map()
globalThis.customElements = { get: (n) => defined.get(n), define: (n, c) => defined.set(n, c) }
let shadow
const makeNode = () => {
  const attrs = new Map()
  const root = shadow
  const node = {
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
    remove() { root.children = root.children.filter((c) => c !== node) },
    html: '',
    set innerHTML(v) { node.html = v },
    get innerHTML() { return node.html },
  }
  return node
}
const makeShadow = () => {
  const root = {
    adoptedStyleSheets: [],
    children: [],
    addEventListener() {},
    replaceChildren(...nodes) { root.children = nodes },
    append(...nodes) { root.children.push(...nodes) },
    querySelector: () => null,
    querySelectorAll: () => [],
    // The element writes the fallback slot with innerHTML.
    set innerHTML(v) {
      const slot = { slot: true, remove() { root.children = root.children.filter((c) => c !== slot) } }
      root.children = v ? [slot] : []
    },
  }
  return root
}
globalThis.document = { createElement: () => makeNode() }
globalThis.HTMLElement = class {
  #attrs = new Map()
  isConnected = false
  events = []
  attachShadow() { shadow = makeShadow(); return shadow }
  getAttribute(name) { name = name.toLowerCase(); return this.#attrs.has(name) ? this.#attrs.get(name) : null }
  hasAttribute(name) { return this.#attrs.has(name.toLowerCase()) }
  setAttribute(name, value) {
    name = name.toLowerCase()
    const before = this.getAttribute(name)
    this.#attrs.set(name, String(value))
    if ((this.constructor.observedAttributes ?? []).includes(name)) this.attributeChangedCallback?.(name, before, String(value))
  }
  dispatchEvent(e) { this.events.push(e.type); return true }
}
globalThis.CSSStyleSheet = class { replaceSync(text) { this.text = text } }
// The morph of the widget script: the server puts Idiomorph before it.
globalThis.Idiomorph = { morph(target, html) { target.innerHTML = html } }
globalThis.location = { origin: 'https://host.example', href: 'https://host.example/' }
globalThis.__GX_WIDGET_CONFIG__ = { ...JSON.parse(process.env.GX_CONFIG), server: 'file://%DIR%' }
// answer is the first answer of the server. A step of a test changes it for a new build.
let answer = { tag: 'acme-cart', html: '<p>cart</p>', script: '/runtime.mjs', style: '/cart.css', build: 'b1' }
let releaseSheet
const sheet = new Promise((done) => (releaseSheet = done))
globalThis.fetch = async (url) => {
  log.push(String(url).replace('file://%DIR%', ''))
  if (String(url).endsWith('.css')) {
    await sheet
    // The text of a stylesheet names its file.
    return new Response(log[log.length - 1])
  }
  return new Response(JSON.stringify(answer), { headers: { 'Content-Type': 'application/json' } })
}
const tick = async () => { for (let i = 0; i < 3; i++) await new Promise((done) => setTimeout(done, 5)) }
await import("%ELEMENT%")
const el = new (customElements.get('acme-cart'))()
const result = () => ({
  state: el.getAttribute('data-gx-state'),
  events: el.events,
  // The boxes of the widget in the shadow root, and its stylesheets.
  boxes: shadow.children.filter((c) => !c.slot).length,
  sheets: shadow.adoptedStyleSheets.length,
  styles: shadow.adoptedStyleSheets.map((s) => s.text),
  requests: log.filter((url) => !url.endsWith('.css')),
})
`

// runElement runs the element file of the repository in the stand-in of a
// browser, with the steps of one test. runtime is the source of the widget
// script that the server of the test names.
func runElement(t *testing.T, config map[string]any, runtime, steps string) string {
	t.Helper()
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	// runBun puts the script in a temp directory; the runtime of the server
	// is a file next to it.
	script := "import { writeFileSync } from 'node:fs'\nwriteFileSync('%DIR%/runtime.mjs', " + string(mustJSON(t, runtime)) + ")\n" + browserScript + steps
	return runBun(t, script, "GX_CONFIG="+string(data))
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// realRuntime is the widget script of the repository.
const realRuntime = `export { mount } from "%WIDGET%"`

// TestREQ_ISL_16_SecondLoadWhileTheFirstRenderMounts checks the state of a
// widget whose host changes an attribute, or sets the token, while the
// first render waits for its stylesheet (REQ-ISL-16: data-gx-state is ready
// and gx-ready fires "after the first HTML is in the shadow root";
// REQ-ISL-15: "A changed attribute fetches again and morphs").
//
// The element starts a second load. The first load is past its fetch, so
// nothing stops it: it is in mount of the widget script, which waits for the
// stylesheet. The second load finds no mounted widget and calls mount too.
// When the stylesheet arrives both mounts put a box into the shadow root.
// The first load then sees that it is old and destroys its mount, and
// destroy removes each child of the shadow root that is not its own box and
// sets adoptedStyleSheets to none: the box and the stylesheet of the second
// mount. The element says ready and fires gx-ready with an empty shadow
// root. A host that sets the token from an effect or after a fetch of its
// own reaches this window at the first load of a page.
func TestREQ_ISL_16_SecondLoadWhileTheFirstRenderMounts(t *testing.T) {
	config := map[string]any{"tag": "acme-cart", "attrs": []string{"currency"}, "path": "/widgets/cart"}
	runtime := strings.ReplaceAll(realRuntime, "%WIDGET%", filepath.ToSlash(filepath.Join(repoRoot(t), "runtime", "js", "widget.ts")))
	const tail = `
releaseSheet()
await tick()
console.log(JSON.stringify(result()))
`
	// With no second load the widget is in its shadow root.
	control := runElement(t, config, runtime, `
el.isConnected = true
el.connectedCallback()
await tick()
`+tail)
	if want := `{"state":"ready","events":["gx-ready"],"boxes":1,"sheets":1,"styles":["/cart.css"],"requests":["/widgets/cart"]}`; control != want {
		t.Fatalf("the fixture is wrong: one load gives %s; want %s", control, want)
	}
	for name, change := range map[string]string{
		"attribute": `el.setAttribute('currency', 'USD')`,
		"token":     `el.token = 't2'`,
	} {
		t.Run(name, func(t *testing.T) {
			got := runElement(t, config, runtime, `
el.isConnected = true
el.connectedCallback()
await tick()
// The first answer is here, and the stylesheet is not. The host changes the element.
`+change+`
await tick()
`+tail)
			var res struct {
				State  string
				Boxes  int
				Sheets int
			}
			if err := json.Unmarshal([]byte(got), &res); err != nil {
				t.Fatalf("%v: %s", err, got)
			}
			if res.State != "ready" || res.Boxes != 1 || res.Sheets != 1 {
				t.Errorf("after a second load that starts while the first render waits for its stylesheet, the element has %s; want the state ready with 1 box and 1 stylesheet in the shadow root", got)
			}
		})
	}
}

// stubRuntime is a widget script that mounts nothing. The test of the
// attributes reads only the requests of the element.
const stubRuntime = `export const mount = async () => ({ update() {}, destroy() {} })`

// TestREQ_ISL_15_AttributeWithAnUpperCaseLetter checks a widget attribute
// whose query tag has an upper-case letter (REQ-ISL-15: "The fields of the
// route input of a widget are the attributes of the element ... A changed
// attribute fetches again and morphs"). The fix of GX6006 for a field with no
// tag names such a tag itself: query:"pageSize" for the field PageSize.
//
// A browser writes the name of an attribute of an HTML element in lower
// case, and calls attributeChangedCallback only for a name that
// observedAttributes holds with the same letters. The element file observes
// "pageSize", so no change of the attribute fetches again: the widget keeps
// its first render. gx check must refuse such a tag, or the element must
// observe the name as the browser writes it.
func TestREQ_ISL_15_AttributeWithAnUpperCaseLetter(t *testing.T) {
	dir := scratchModule(t, cartModule("PageSize int `query:\"pageSize\" default:\"10\"`"))
	widgets, diags := compiler.Widgets(dir)
	if len(diags) > 0 {
		// gx check refuses the tag: the app owner learns of it at compile
		// time.
		for _, d := range diags {
			if !strings.Contains(d.Msg, "pageSize") && !strings.Contains(d.Msg, "PageSize") {
				t.Fatalf("the fixture is wrong: %v", diags)
			}
		}
		return
	}
	if len(widgets) != 1 || len(widgets[0].Attributes) != 1 {
		t.Fatalf("the fixture is wrong: widgets %+v", widgets)
	}
	attrs := []string{}
	for _, a := range widgets[0].Attributes {
		attrs = append(attrs, a.Name)
	}
	config := map[string]any{"tag": "acme-cart", "attrs": attrs, "path": widgets[0].Path}
	got := runElement(t, config, stubRuntime, `
releaseSheet()
el.setAttribute(`+string(mustJSON(t, attrs[0]))+`, '3')
el.isConnected = true
el.connectedCallback()
await tick()
// The host page changes the attribute, as page.setAttribute or a render of React does.
el.setAttribute(`+string(mustJSON(t, attrs[0]))+`, '5')
await tick()
console.log(JSON.stringify(result().requests))
`)
	var requests []string
	if err := json.Unmarshal([]byte(got), &requests); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	if len(requests) == 0 || !strings.HasSuffix(requests[0], "=3") {
		t.Fatalf("the fixture is wrong: the first request is %v", requests)
	}
	if len(requests) != 2 || !strings.HasSuffix(requests[1], "=5") {
		t.Errorf("the host changed the attribute %s of <acme-cart> from 3 to 5, and the element sent the requests %v; want a second request with the value 5. gx check gave no diagnostic for the tag query:\"pageSize\"", attrs[0], requests)
	}
}

// valueExport is a declaration of a type file that says the module exports a
// value: a class, a constant or a function.
var valueExport = regexp.MustCompile(`(?m)^export declare (?:class|const|let|var|function) ([A-Za-z_$][A-Za-z0-9_$]*)`)

// TestREQ_ISL_10_TypeFileNamesOnlyExportsOfTheElementFile checks that the
// .d.ts file of gx wc build describes the element file next to it
// (REQ-ISL-10: gx wc build "writes for each widget the element file (ESM), a
// .d.ts file and custom-elements.json"). The type file says
// "export declare class AcmeCartElement extends HTMLElement". The element
// file exports nothing. A TypeScript host that writes
//
//	import { AcmeCartElement } from "@acme/widgets/acme-cart"
//	if (el instanceof AcmeCartElement) ...
//
// compiles with no error, and the browser then refuses the module: the file
// does not provide an export named AcmeCartElement.
func TestREQ_ISL_10_TypeFileNamesOnlyExportsOfTheElementFile(t *testing.T) {
	dir := scratchModule(t, cartModule("PageSize int `query:\"size\"`"))
	widgets, diags := compiler.Widgets(dir)
	if len(diags) != 0 || len(widgets) != 1 {
		t.Fatalf("the fixture is wrong: widgets %d, diagnostics %v", len(widgets), diags)
	}
	w := widgets[0]
	file, err := widgetelement.File(widgetelement.Config{Tag: w.Tag, Attrs: []string{"size"}, Server: "https://api.example", Path: w.Path})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	element := filepath.Join(out, w.Tag+".mjs")
	if err := os.WriteFile(element, file, 0o644); err != nil {
		t.Fatal(err)
	}
	got := runBun(t, `
globalThis.HTMLElement = class {}
globalThis.customElements = { get: () => undefined, define() {} }
globalThis.location = { origin: 'https://host.example', href: 'https://host.example/' }
const m = await import(`+string(mustJSON(t, filepath.ToSlash(element)))+`)
console.log(JSON.stringify(Object.keys(m)))
`)
	var exports []string
	if err := json.Unmarshal([]byte(got), &exports); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	has := map[string]bool{}
	for _, name := range exports {
		has[name] = true
	}
	for _, m := range valueExport.FindAllStringSubmatch(string(w.DTS), -1) {
		if !has[m[1]] {
			t.Errorf("%s.d.ts declares the value export %s, and %s.js exports %v: an import of %s compiles in a host and fails in the browser", w.Tag, m[1], w.Tag, exports, m[1])
		}
	}
}

// TestREQ_ISL_19_AttributeChangeAfterANewBuild checks an open widget whose
// server gets a new build before the host changes an attribute (REQ-ISL-19:
// "The first answer of the server names one script and one stylesheet with
// content hashes ... Each answer carries the hash of the build"; acceptance:
// "a new build of the server shows a changed widget in the host").
//
// The answer to the attribute change is a first answer of the new build: it
// names the script and the stylesheet of that build. The element gives it to
// update of the mounted widget script of the old build, which morphs the new
// HTML and reads no other part of the answer. The widget then shows the HTML
// of the new build with the stylesheet of the old build: each class that
// only the new build uses has no rule. The element must do a fresh first
// render, as it does for a patch answer with a different build.
func TestREQ_ISL_19_AttributeChangeAfterANewBuild(t *testing.T) {
	config := map[string]any{"tag": "acme-cart", "attrs": []string{"currency"}, "path": "/widgets/cart"}
	runtime := strings.ReplaceAll(realRuntime, "%WIDGET%", filepath.ToSlash(filepath.Join(repoRoot(t), "runtime", "js", "widget.ts")))
	got := runElement(t, config, runtime, `
releaseSheet()
el.isConnected = true
el.connectedCallback()
await tick()
const first = result()
// The app owner deploys a new build. Its stylesheet has a new content hash.
answer = { ...answer, html: '<p class="new-class">cart</p>', style: '/cart.b2.css', build: 'b2' }
el.setAttribute('currency', 'USD')
await tick()
await tick()
console.log(JSON.stringify({ first: first.styles, state: el.getAttribute('data-gx-state'), styles: result().styles }))
`)
	var res struct {
		First  []string
		State  string
		Styles []string
	}
	if err := json.Unmarshal([]byte(got), &res); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	if len(res.First) != 1 || res.First[0] != "/cart.css" {
		t.Fatalf("the fixture is wrong: the first render has the stylesheets %v", res.First)
	}
	if res.State != "ready" || len(res.Styles) != 1 || res.Styles[0] != "/cart.b2.css" {
		t.Errorf("after an attribute change that the build b2 of the server answers, the element has the state %q and the stylesheets %v; want ready with the stylesheet /cart.b2.css of the build that made the HTML", res.State, res.Styles)
	}
}
