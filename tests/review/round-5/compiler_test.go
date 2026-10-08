package round5_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/widgetpkg"
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

const (
	cartRoutes = "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
		"type Widget struct {\n\tgx.Route `GET /cart`\n\tCurrency string `query:\"currency\" default:\"EUR\"`\n}\n\n" +
		"type Add struct {\n\tgx.Route `POST /cart/add`\n}\n"
	cartGx = "package cart\n\nimport \"app/cart/route\"\n\nprops {\n  Currency string\n}\n\n<section>\n  <p>{p.Currency}</p>\n  <button on:click={route.Add{}}>Add</button>\n</section>\n"
	cartGo = "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"var CartWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {\n\treturn CartProps{Currency: in.Currency}, nil\n}, Cart).Tag(\"acme-cart\")\n\n" +
		"var add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\n" +
		"var Routes = gx.Collect(CartWidget, add)\n"
	cartMain = "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" +
		"\tapp.Group(\"/widgets\", gx.AllowOrigins(\"https://shop.example.com\"), cart.Routes)\n\t_ = app\n}\n"
)

// cartEvents declares the event cart-changed. Its detail has a field of the
// string enum Mode with the given values.
func cartEvents(values ...string) string {
	var consts strings.Builder
	for i, v := range values {
		consts.WriteString("\tMode" + string(rune('A'+i)) + " Mode = \"" + v + "\"\n")
	}
	return "package cart\n\nimport \"github.com/alternayte/gx\"\n\n" +
		"type Mode string\n\nconst (\n" + consts.String() + ")\n\n" +
		"type ChangedDetail struct {\n\tCount int  `json:\"count\"`\n\tMode  Mode `json:\"mode\"`\n}\n\n" +
		"var Changed = gx.Event[ChangedDetail](\"cart-changed\")\n"
}

// manifestOf returns the custom elements manifest of the widgets of an app,
// as gx wc check makes it, and the type file of its first widget.
func manifestOf(t *testing.T, events string) (manifest []byte, dts string) {
	t.Helper()
	dir := scratchModule(t, map[string]string{
		"cart/route/route.go": cartRoutes,
		"cart/Cart.gx":        cartGx,
		"cart/cart.go":        cartGo,
		"cart/events.go":      events,
		"main.go":             cartMain,
	})
	widgets, diags := compiler.Widgets(dir)
	if len(diags) != 0 || len(widgets) != 1 {
		t.Fatalf("widgets %d, diagnostics %v", len(widgets), diags)
	}
	manifest, err := compiler.WidgetManifest(widgets)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, string(widgets[0].DTS)
}

// TestREQ_ISL_13_RetypedEnumDetailFieldIsBreaking checks the class of a
// detail field whose string enum changes its values (REQ-ISL-13: "A removed
// or retyped ... event detail field ... is breaking and needs a major
// bump"). The type file of the host changes from "full" | "quick" to
// "full" | "fast": a host that compares the field with "quick" does not
// compile, and no longer gets that value.
func TestREQ_ISL_13_RetypedEnumDetailFieldIsBreaking(t *testing.T) {
	before, beforeDTS := manifestOf(t, cartEvents("full", "quick"))
	after, afterDTS := manifestOf(t, cartEvents("full", "fast"))
	if !strings.Contains(beforeDTS, `export type Mode = "full" | "quick";`) || !strings.Contains(afterDTS, `export type Mode = "fast" | "full";`) {
		t.Fatalf("the fixture is wrong: the type files do not hold the two enums:\n%s\n%s", beforeDTS, afterDTS)
	}
	changes, err := widgetpkg.Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if level := widgetpkg.Needed(changes); level != widgetpkg.Major {
		t.Errorf("the type of the detail field mode changed from \"full\" | \"quick\" to \"fast\" | \"full\", and gx wc check needs the bump %q with the changes %v; want major", level, changes)
	}
}

// A widget whose component is a form. The form is mounted in a group with no
// gx.AllowOrigins.
const (
	noteRoutes = "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
		"type Widget struct {\n\tgx.Route `GET /note`\n\tTo string `query:\"to\"`\n}\n\n" +
		"type Send struct {\n\tgx.Route `POST /note/send`\n\tTo   string\n\tBody string\n}\n\n" +
		"func (in *Send) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.To, gx.Required), gx.Field(&in.Body, gx.Required)}\n}\n"
	noteGx = "package note\n\nimport \"app/note/route\"\n\nprops {\n  F route.SendForm\n}\n\n" +
		"<form {...p.F.Attrs()}>\n  <input name={p.F.To.Name} value={p.F.To.Value} />\n  <input name={p.F.Body.Name} value={p.F.Body.Value} />\n  <button type=\"submit\">Send</button>\n</form>\n"
	noteGo = "package note\n\nimport (\n\t\"app/note/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"var Send = gx.Form(func(c *gx.Ctx, in *route.Send) error { return nil }, Note)\n\n" +
		"var Widget = gx.Widget(func(c *gx.Ctx, in route.Widget) (NoteProps, error) {\n\treturn Send.Props(&route.Send{To: in.To}), nil\n}, Note).Tag(\"acme-note\")\n"
)

func noteMain(groups string) string {
	return "package main\n\nimport (\n\t\"app/note\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" + groups + "\t_ = app\n}\n"
}

// TestREQ_ISL_22_FormOfAWidgetNeedsOrigins checks GX6008 for the form of a
// widget in a group with no gx.AllowOrigins (REQ-ISL-22: "A widget, or an
// action that the component of a widget invokes, in a group with no
// AllowOrigins is an error in gx check"; a form is a form action, section 7,
// and REQ-ISL-20 names forms as a part of a widget). The submit of the host
// gets 403 at run time, and gx check says nothing.
func TestREQ_ISL_22_FormOfAWidgetNeedsOrigins(t *testing.T) {
	files := func(groups string) map[string]string {
		return map[string]string{
			"note/route/route.go": noteRoutes,
			"note/Note.gx":        noteGx,
			"note/note.go":        noteGo,
			"main.go":             noteMain(groups),
		}
	}
	// With the form in the group of the widget the app has no finding.
	good := compiler.Check(scratchModule(t, files(
		"\tapp.Group(\"/\", gx.AllowOrigins(\"https://shop.example.com\"), note.Widget, note.Send)\n")))
	if len(good) != 0 {
		t.Fatalf("the fixture is wrong: diagnostics %v", good)
	}
	diags := compiler.Check(scratchModule(t, files(
		"\tapp.Group(\"/\", gx.AllowOrigins(\"https://shop.example.com\"), note.Widget)\n\tapp.Group(\"/\", note.Send)\n")))
	found := false
	for _, d := range diags {
		if d.Code == "GX6008" && strings.Contains(d.Msg, "Send") {
			found = true
		}
	}
	if !found {
		t.Errorf("the form Send of the widget acme-note is in a group with no gx.AllowOrigins, and gx check has no GX6008 for it: %v", diags)
	}
}

// TestSI_04_SecretMapKeyIsGX7002 checks the compile error for a gx.Secret
// that is the key of a map in a tool result and in the detail of an event
// (SI-04: "Compile error where visible"). The JSON of a map writes its keys
// as text, so the key reaches the agent and the browser.
func TestSI_04_SecretMapKeyIsGX7002(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"keys/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype List struct {\n\tgx.Route `POST /keys`\n}\n",
		"keys/keys.go": "package keys\n\nimport (\n\t\"app/keys/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"type Detail struct {\n\tKeys map[gx.Secret]bool `json:\"keys\"`\n}\n\n" +
			"var Listed = gx.Event[Detail](\"keys-listed\")\n\n" +
			"// List lists the keys of the user.\n" +
			"var List = gx.Action(func(c *gx.Ctx, in route.List) error {\n\tgx.ToolResult(c, map[gx.Secret]bool{})\n\treturn nil\n}).Tool()\n",
	})
	var event, result bool
	diags := compiler.Check(dir)
	for _, d := range diags {
		if d.Code != "GX7002" {
			continue
		}
		if strings.Contains(d.Msg, "keys-listed") {
			event = true
		}
		if strings.Contains(d.Msg, "tool result") {
			result = true
		}
	}
	if !event {
		t.Errorf("the detail of the event keys-listed has a map with gx.Secret keys, and gx check has no GX7002 for it: %v", diags)
	}
	if !result {
		t.Errorf("the tool result is a map with gx.Secret keys, and gx check has no GX7002 for it: %v", diags)
	}
}
