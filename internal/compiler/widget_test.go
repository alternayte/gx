package compiler_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const widgetRoutes = "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
	"type Widget struct {\n\tgx.Route `GET /widgets/cart`\n\tCurrency string `query:\"currency\" default:\"EUR\"`\n\tCompact  bool   `query:\"compact\"`\n\tLimit    int    `query:\"limit\"`\n}\n\n" +
	"type Add struct {\n\tgx.Route `POST /widgets/cart/add`\n}\n"

const widgetGx = "package cart\n\nimport \"app/cart/route\"\n\nprops {\n  Currency string\n}\n\n<section>\n  <p>{p.Currency}</p>\n  <button on:click={route.Add{}}>Add</button>\n</section>\n"

func widgetGo(tag string) string {
	return "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"var CartWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {\n\treturn CartProps{Currency: in.Currency}, nil\n}, Cart)" + tag + "\n\n" +
		"var add = gx.Action(func(c *gx.Ctx, in route.Add) error { return nil })\n\n" +
		"var Routes = gx.Collect(CartWidget, add)\n"
}

func widgetMain(groups string) string {
	return "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" + groups + "\t_ = app\n}\n"
}

func widgetTree(t *testing.T, files map[string]string) []compiler.Diagnostic {
	t.Helper()
	tree := map[string]string{
		"go.mod":              moduleWithGx(t),
		"cart/route/route.go": widgetRoutes,
		"cart/Cart.gx":        widgetGx,
		"cart/cart.go":        widgetGo(`.Tag("acme-cart")`),
		"main.go":             widgetMain("\tapp.Group(\"/\", gx.AllowOrigins(\"https://shop.example.com\"), cart.Routes)\n"),
	}
	for name, src := range files {
		tree[name] = src
	}
	return checkDir(t, writeTree(t, tree))
}

func codesOf(diags []compiler.Diagnostic, code string) []compiler.Diagnostic {
	var out []compiler.Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// TestREQ_ISL_10_WidgetCompiles checks that a widget with a tag, scalar
// attributes and a group with origins has no diagnostic.
func TestREQ_ISL_10_WidgetCompiles(t *testing.T) {
	if diags := widgetTree(t, nil); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

// TestREQ_ISL_10_WidgetTagDiagnostic covers GX6003: a tag that is not a
// custom element name, a widget with no tag, and one tag on two widgets.
func TestREQ_ISL_10_WidgetTagDiagnostic(t *testing.T) {
	t.Run("a name with no hyphen", func(t *testing.T) {
		diags := codesOf(widgetTree(t, map[string]string{"cart/cart.go": widgetGo(`.Tag("cart")`)}), compiler.CodeWidgetTag)
		if len(diags) != 1 || !strings.HasSuffix(diags[0].File, "cart.go") || diags[0].Line != 11 || !strings.Contains(diags[0].Msg, "hyphen") {
			t.Fatalf("diagnostics = %v, want one GX6003 at the tag", diags)
		}
	})
	t.Run("no tag", func(t *testing.T) {
		diags := codesOf(widgetTree(t, map[string]string{"cart/cart.go": widgetGo("")}), compiler.CodeWidgetTag)
		if len(diags) != 1 || !strings.Contains(diags[0].Msg, "no tag") {
			t.Fatalf("diagnostics = %v, want one GX6003 for the missing tag", diags)
		}
	})
	t.Run("one tag two times", func(t *testing.T) {
		second := "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"var Second = gx.Widget(func(c *gx.Ctx, in route.Second) (CartProps, error) {\n\treturn CartProps{}, nil\n}, Cart).Tag(\"acme-cart\")\n\nvar More = gx.Collect(Second)\n"
		routes := widgetRoutes + "\ntype Second struct {\n\tgx.Route `GET /widgets/second`\n}\n"
		diags := codesOf(widgetTree(t, map[string]string{
			"cart/second.go":      second,
			"cart/route/route.go": routes,
			"main.go":             widgetMain("\tapp.Group(\"/\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes, cart.More)\n"),
		}), compiler.CodeWidgetTag)
		if len(diags) != 1 || !strings.Contains(diags[0].Msg, "acme-cart") {
			t.Fatalf("diagnostics = %v, want one GX6003 for the second use of the tag", diags)
		}
	})
}

// TestREQ_ISL_15_WidgetAttributeType covers GX6006: a field of a widget
// input that an attribute cannot hold.
func TestREQ_ISL_15_WidgetAttributeType(t *testing.T) {
	routes := strings.Replace(widgetRoutes, "\tLimit    int    `query:\"limit\"`\n", "\tLimit    int    `query:\"limit\"`\n\tTags     []string `query:\"tags\"`\n", 1)
	diags := codesOf(widgetTree(t, map[string]string{"cart/route/route.go": routes}), compiler.CodeWidgetAttr)
	if len(diags) != 1 || !strings.HasSuffix(diags[0].File, "route.go") || diags[0].Line != 10 || !strings.Contains(diags[0].Msg, "Tags") {
		t.Fatalf("diagnostics = %v, want one GX6006 at the field Tags", diags)
	}
}

// TestREQ_ISL_22_WidgetNeedsOrigins covers GX6008: a widget, or an action
// that its component invokes, in a group with no gx.AllowOrigins.
func TestREQ_ISL_22_WidgetNeedsOrigins(t *testing.T) {
	cases := []struct {
		name   string
		groups string
		want   []string // a part of the message of each GX6008
	}{
		{"origins before the routes", "\tapp.Group(\"/\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes)\n", nil},
		{"credentials only", "\tapp.Group(\"/\", gx.AllowCredentials(\"https://app.acme.dev\"), cart.Routes)\n", nil},
		{"no origins", "\tapp.Group(\"/\", cart.Routes)\n", []string{"widget acme-cart", "action"}},
		{"origins after the routes", "\tapp.Group(\"/\", cart.Routes, gx.AllowOrigins(gx.AnyOrigin))\n", []string{"widget acme-cart", "action"}},
		{"the widget in a second group with no origins",
			"\tapp.Group(\"/w\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes)\n\tapp.Group(\"/app\", cart.CartWidget)\n",
			[]string{"widget acme-cart"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := codesOf(widgetTree(t, map[string]string{"main.go": widgetMain(tc.groups)}), compiler.CodeWidgetOrigins)
			if len(diags) != len(tc.want) {
				t.Fatalf("GX6008 diagnostics = %v, want %d", diags, len(tc.want))
			}
			for i, part := range tc.want {
				if !strings.Contains(diags[i].Msg, part) || !strings.HasSuffix(diags[i].File, "cart.go") {
					t.Errorf("diagnostic %d = %v, want %q in cart.go", i, diags[i], part)
				}
			}
		})
	}

	t.Run("an action of the component in a group with no origins", func(t *testing.T) {
		cartGo := strings.Replace(widgetGo(`.Tag("acme-cart")`), "var Routes = gx.Collect(CartWidget, add)", "var Routes = gx.Collect(CartWidget)\n\nvar Actions = gx.Collect(add)", 1)
		diags := codesOf(widgetTree(t, map[string]string{
			"cart/cart.go": cartGo,
			"main.go":      widgetMain("\tapp.Group(\"/w\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes)\n\tapp.Group(\"/app\", cart.Actions)\n"),
		}), compiler.CodeWidgetOrigins)
		if len(diags) != 1 || !strings.Contains(diags[0].Msg, "action") || !strings.Contains(diags[0].Msg, "acme-cart") || diags[0].Line != 13 {
			t.Fatalf("diagnostics = %v, want one GX6008 at the action", diags)
		}
	})
}

// TestREQ_ISL_11_WidgetClassList checks the class list of a widget: the
// classes of its component, of each component that it uses, and of the Go
// files of their packages. A class of a different part of the app is not in
// the list.
func TestREQ_ISL_11_WidgetClassList(t *testing.T) {
	cartGx := "package cart\n\nimport (\n  \"app/cart/route\"\n  \"app/ui/badge\"\n)\n\nprops {\n  Currency string\n}\n\n" +
		"<section class=\"p-4 font-bold\" class:ring-2={p.Currency == \"USD\"}>\n  <badge.Badge label={p.Currency} />\n  <button class={tone()} on:click={route.Add{}}>Add</button>\n</section>\n"
	dir := writeTree(t, map[string]string{
		"go.mod":              moduleWithGx(t),
		"cart/route/route.go": widgetRoutes,
		"cart/Cart.gx":        cartGx,
		"cart/cart.go":        widgetGo(`.Tag("acme-cart")`),
		"cart/styles.go":      "package cart\n\nfunc tone() string { return \"bg-primary text-white\" }\n",
		"ui/badge/Badge.gx":   "package badge\n\nprops {\n  Label string\n}\n\n<span class=\"rounded-full\">{p.Label}</span>\n",
		"other/Page.gx":       "package other\n\n<p class=\"underline\">Other</p>\n",
		"other/styles.go":     "package other\n\nconst tone = \"italic\"\n",
		"main.go":             widgetMain("\tapp.Group(\"/\", gx.AllowOrigins(\"https://shop.example.com\"), cart.Routes)\n"),
	})
	files, diags := compiler.Generate(dir)
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	var got string
	for path, src := range files {
		if strings.HasSuffix(filepath.ToSlash(path), ".gx/widget-classes.json") {
			got = string(src)
		}
	}
	var lists map[string][]string
	if err := json.Unmarshal([]byte(got), &lists); err != nil {
		t.Fatalf("no class lists of the widgets, or not JSON: %v\n%s", err, got)
	}
	want := []string{"bg-primary", "font-bold", "p-4", "ring-2", "rounded-full", "text-white"}
	have := map[string]bool{}
	for _, c := range lists["acme-cart"] {
		have[c] = true
	}
	for _, c := range want {
		if !have[c] {
			t.Errorf("the class list of acme-cart has no %q: %v", c, lists["acme-cart"])
		}
	}
	for _, c := range []string{"underline", "italic"} {
		if have[c] {
			t.Errorf("the class list of acme-cart holds %q, a class of a different part of the app", c)
		}
	}
	if len(lists) != 1 {
		t.Errorf("class lists = %v, want one widget", lists)
	}
}

// TestREQ_ISL_17_SecretInEventDetail covers GX7002 for a domain event: the
// detail goes to the browser, so its type holds no gx.Secret (SI-04).
func TestREQ_ISL_17_SecretInEventDetail(t *testing.T) {
	events := func(detail string) string {
		return "package cart\n\nimport \"github.com/alternayte/gx\"\n\ntype Line struct {\n\tSKU  string\n\tCard gx.Secret\n}\n\n" +
			"type Detail struct {\n" + detail + "}\n\nvar Changed = gx.Event[Detail](\"cart-changed\")\n"
	}
	for name, detail := range map[string]string{
		"a field":             "\tToken gx.Secret\n",
		"a field of a struct": "\tLine Line\n",
		"a list":              "\tLines []Line\n",
		"a map and a pointer": "\tByID map[string]*Line\n",
	} {
		t.Run(name, func(t *testing.T) {
			diags := codesOf(checkDir(t, writeTree(t, map[string]string{
				"go.mod":         moduleWithGx(t),
				"cart/events.go": events(detail),
			})), compiler.CodeSecret)
			if len(diags) != 1 || !strings.HasSuffix(diags[0].File, "events.go") || diags[0].Line != 14 || !strings.Contains(diags[0].Msg, "cart-changed") {
				t.Fatalf("diagnostics = %v, want one GX7002 at the event", diags)
			}
		})
	}
	clean := checkDir(t, writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/events.go": events("\tCount int\n\tNote  string\n\tNext  *Detail\n"),
	}))
	if len(clean) != 0 {
		t.Fatalf("a detail with no secret: %v", clean)
	}
}

// TestREQ_ISL_20_HeadInWidget covers GX6007: gx.Head in the view of a
// widget, or in a component that the view uses. A widget has no document.
func TestREQ_ISL_20_HeadInWidget(t *testing.T) {
	withHead := "package cart\n\nimport \"app/cart/route\"\n\nprops {\n  Currency string\n}\n\n<gx.Head title=\"Cart\" />\n<section>\n  <button on:click={route.Add{}}>Add</button>\n</section>\n"
	diags := codesOf(widgetTree(t, map[string]string{"cart/Cart.gx": withHead}), compiler.CodeWidgetHead)
	if len(diags) != 1 || !strings.HasSuffix(diags[0].File, "Cart.gx") || diags[0].Line != 9 || !strings.Contains(diags[0].Msg, "acme-cart") {
		t.Fatalf("diagnostics = %v, want one GX6007 at the gx.Head tag", diags)
	}

	// In a component that the view uses.
	view := "package cart\n\nimport (\n  \"app/cart/route\"\n  \"app/ui/title\"\n)\n\nprops {\n  Currency string\n}\n\n<section>\n  <title.Title text={p.Currency} />\n  <button on:click={route.Add{}}>Add</button>\n</section>\n"
	child := "package title\n\nprops {\n  Text string\n}\n\n<gx.Head title={p.Text} />\n<h2>{p.Text}</h2>\n"
	diags = codesOf(widgetTree(t, map[string]string{"cart/Cart.gx": view, "ui/title/Title.gx": child}), compiler.CodeWidgetHead)
	if len(diags) != 1 || !strings.HasSuffix(diags[0].File, "Title.gx") || diags[0].Line != 7 {
		t.Fatalf("diagnostics = %v, want one GX6007 in the child component", diags)
	}

	// A page with gx.Head, in an app with a widget, has no finding.
	page := "package other\n\n<gx.Head title=\"Other\" />\n<p>Other</p>\n"
	if diags := codesOf(widgetTree(t, map[string]string{"other/Page.gx": page}), compiler.CodeWidgetHead); len(diags) != 0 {
		t.Fatalf("a page with gx.Head: %v", diags)
	}
}
