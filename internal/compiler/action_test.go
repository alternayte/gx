package compiler_test

import (
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
		"go.mod":           moduleWithGx(t),
		"cart/routes.go":   addRoute,
		"cart/Button.gx":   addButton,
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
