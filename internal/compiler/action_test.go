package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const addRoute = `package cart

import "github.com/alternayte/gx"

type Add struct {
	gx.Route ` + "`" + `POST /cart/add` + "`" + `
	ID int64
}
`

const addButton = `package cart

<button on:click={Add{ID: 1}}>Add</button>
`

const addAction = `package cart

import "github.com/alternayte/gx"

var add = gx.Action(func(c *gx.Ctx, in Add) error { return nil })
`

func TestREQ_ACT_02_MissingRegistration(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": addRoute,
		"cart/Button.gx": addButton,
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeActionMissing)
	if !strings.Contains(d.Msg, `"Add"`) {
		t.Fatalf("GX4001 message = %q", d.Msg)
	}
	if !strings.HasSuffix(d.File, "Button.gx") || d.Line != 3 {
		t.Fatalf("GX4001 position = %s:%d, want Button.gx:3", d.File, d.Line)
	}
}

func TestREQ_ACT_02_RegisteredAction(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": addRoute,
		"cart/action.go": addAction,
		"cart/Button.gx": addButton,
	})
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("registered action: unexpected diagnostics %v", diags)
	}
}

func TestREQ_ACT_02_TwoRegistrations(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": addRoute,
		"cart/one.go":    addAction,
		"cart/two.go":    strings.Replace(addAction, "var add", "var addTwo", 1),
		"cart/Button.gx": addButton,
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeActionTwice)
	if !strings.Contains(d.Msg, "registered 2 times") {
		t.Fatalf("GX4002 message = %q", d.Msg)
	}
}

func TestREQ_ACT_02_UnsupportedMethod(t *testing.T) {
	route := strings.Replace(addRoute, "POST /cart/add", "OPTIONS /cart/add", 1)
	action := strings.Replace(addAction, "POST", "OPTIONS", 0)
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": route,
		"cart/action.go": action,
		"cart/Button.gx": addButton,
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeActionMethod)
	if !strings.Contains(d.Msg, "OPTIONS") {
		t.Fatalf("GX4009 message = %q", d.Msg)
	}
}

func TestREQ_ACT_02_ActionCodegen(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": addRoute,
		"cart/action.go": addAction,
		"cart/Button.gx": addButton,
	})
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Button_gx.go")])
	want := `gx.Attr{Key: "data-on:click", Value: "@post('" + (Add{ID: 1}).URL() + "')", Kind: gx.AttrText}`
	if !strings.Contains(src, want) {
		t.Fatalf("Button_gx.go lacks the action attribute:\n%s", src)
	}
	routes := string(files[filepath.Join(dir, "cart/routes_gx.go")])
	if !strings.Contains(routes, "func (in Add) URL() string") {
		t.Fatalf("routes_gx.go lacks the URL method:\n%s", routes)
	}
}

const signalRoute = `package cart

import "github.com/alternayte/gx"

type Add struct {
	gx.Unchecked
	gx.Route ` + "`" + `POST /cart/add` + "`" + `
	Qty int ` + "`" + `signal:"qty"` + "`" + `
}
`

const signalAction = `package cart

import "github.com/alternayte/gx"

var add = gx.Action(func(c *gx.Ctx, in Add) error { return nil })
`

func TestREQ_ACT_03_SignalBinding(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalRoute,
		"cart/action.go": signalAction,
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty int = 1\n}\n\n<button on:click={Add{}}>Add</button>\n",
	})
	if diags := checkDir(t, dir); len(diags) != 0 {
		t.Fatalf("declared signal: unexpected diagnostics %v", diags)
	}
	files := generateFiles(t, dir)
	src := string(files[filepath.Join(dir, "cart/Cart_gx.go")])
	want := `"@post('" + (Add{}).URL() + "', {headers: {'Gx-Scope': '" + gx.ScopeString("cart.Cart", p.GxKey) + "'}})"`
	if !strings.Contains(src, want) {
		t.Fatalf("Cart_gx.go lacks the scoped action attribute:\n%s", src)
	}
}

func TestREQ_ACT_03_MissingSignal(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalRoute,
		"cart/action.go": signalAction,
		"cart/Cart.gx":   "package cart\n\n<button on:click={Add{}}>Add</button>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeActionMissingSignal)
	if !strings.Contains(d.Msg, `"qty"`) {
		t.Fatalf("GX4003 message = %q", d.Msg)
	}
}

func TestREQ_ACT_03_SignalTypeMismatch(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": signalRoute,
		"cart/action.go": signalAction,
		"cart/Cart.gx":   "package cart\n\nsignals {\n  Qty string = \"x\"\n}\n\n<button on:click={Add{}}>Add</button>\n",
	})
	diags := checkDir(t, dir)
	d := diagWith(t, diags, compiler.CodeSignalTypeMismatch)
	if !strings.Contains(d.Msg, `"int"`) || !strings.Contains(d.Msg, `"string"`) {
		t.Fatalf("GX4004 message = %q", d.Msg)
	}
}

func TestREQ_ACT_03_BindGenerated(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"cart/routes.go": "package cart\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"type Add struct {\n" +
			"\tgx.Unchecked\n" +
			"\tgx.Route `POST /cart/{id}`\n" +
			"\tID   int64\n" +
			"\tNote string\n" +
			"\tQty  int `signal:\"qty\"`\n" +
			"}\n" +
			"\nvar add = gx.Action(func(c *gx.Ctx, in Add) error { return nil })\n",
		"cart/bind_test.go": "package cart\n\nimport (\n\t\"net/http/httptest\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestBind(t *testing.T) {\n" +
			"\treq := httptest.NewRequest(\"POST\", \"/cart/7\", strings.NewReader(\"note=hi\"))\n" +
			"\treq.Header.Set(\"Content-Type\", \"application/x-www-form-urlencoded\")\n" +
			"\treq.SetPathValue(\"id\", \"7\")\n" +
			"\tvar in Add\n" +
			"\tif err := in.Bind(req); err != nil { t.Fatal(err) }\n" +
			"\tif in.ID != 7 || in.Note != \"hi\" || in.Qty != 0 {\n" +
			"\t\tt.Fatalf(\"in = %+v\", in)\n" +
			"\t}\n" +
			"}\n",
	})
	files := generateFiles(t, dir)
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test: %v\n%s", err, out)
	}
}
