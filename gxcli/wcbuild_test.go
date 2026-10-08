package gxcli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

func widgetModule(t *testing.T) string {
	t.Helper()
	return scratchModule(t, map[string]string{
		"cart/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Widget struct {\n\tgx.Route `GET /cart`\n\tCurrency string `query:\"currency\" default:\"EUR\"`\n\tCompact  bool   `query:\"compact\"`\n}\n",
		"cart/Cart.gx":        "package cart\n\nprops {\n  Currency string\n}\n\n<section>{p.Currency}</section>\n",
		"cart/cart.go": "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"type ChangedDetail struct {\n\tCount int `json:\"count\"`\n}\n\nvar Changed = gx.Event[ChangedDetail](\"cart-changed\")\n\n" +
			"var CartWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {\n\treturn CartProps{Currency: in.Currency}, nil\n}, Cart).Tag(\"acme-cart\")\n\nvar Routes = gx.Collect(CartWidget)\n",
		"mail/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Widget struct {\n\tgx.Route `GET /composer`\n}\n",
		"mail/Composer.gx":    "package mail\n\n<section>Composer</section>\n",
		"mail/mail.go": "package mail\n\nimport (\n\t\"app/mail/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"var ComposerWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (ComposerProps, error) {\n\treturn ComposerProps{}, nil\n}, Composer).Tag(\"acme-composer\")\n\nvar Routes = gx.Collect(ComposerWidget)\n",
		"main.go": "package main\n\nimport (\n\t\"app/cart\"\n\t\"app/mail\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" +
			"\tapp.Group(\"/widgets\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes, mail.Routes)\n\t_ = app\n}\n",
	})
}

// TestREQ_ISL_10_WCBuildCommand checks `gx wc build`: for each widget it
// writes the element file with the origin of the server, a type file and the
// JSX types, and one manifest for the app. It runs no node.
func TestREQ_ISL_10_WCBuildCommand(t *testing.T) {
	dir := widgetModule(t)
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate = %d", code)
	}
	out := filepath.Join(t.TempDir(), "widgets")
	if code := gxcli.Main([]string{"wc", "build", "--server", "https://api.acme.dev/", "--base", "/shop", "--out", out, dir}); code != 0 {
		t.Fatalf("gx wc build = %d", code)
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	element := read("acme-cart.js")
	if !strings.Contains(element, `{"tag":"acme-cart","attrs":["currency","compact"],"server":"https://api.acme.dev","path":"/shop/widgets/cart"}`) {
		t.Errorf("the element file has no configuration of the widget:\n%.300s", element)
	}
	if !strings.Contains(element, "customElements.define") || strings.Contains(element, "__GX_WIDGET_CONFIG__") {
		t.Errorf("the element file is not the loader")
	}
	if dts := read("acme-cart.d.ts"); !strings.Contains(dts, `"cart-changed": CustomEvent<ChangedDetail>;`) || !strings.Contains(dts, "currency?: string;") {
		t.Errorf("the type file:\n%s", dts)
	}
	if react := read("acme-cart.react.d.ts"); !strings.Contains(react, `from "./acme-cart";`) {
		t.Errorf("the React type file:\n%s", react)
	}
	if composer := read("acme-composer.js"); !strings.Contains(composer, `"path":"/shop/widgets/composer"`) {
		t.Errorf("the element file of the second widget:\n%.300s", composer)
	}
	var manifest struct {
		Modules []struct {
			Path string `json:"path"`
		} `json:"modules"`
	}
	if err := json.Unmarshal([]byte(read("custom-elements.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Modules) != 2 || manifest.Modules[0].Path != "acme-cart.js" || manifest.Modules[1].Path != "acme-composer.js" {
		t.Errorf("manifest modules = %+v", manifest.Modules)
	}

	// A package directory builds the widgets of that package only.
	one := filepath.Join(t.TempDir(), "one")
	if code := gxcli.Main([]string{"wc", "build", "--out", one, filepath.Join(dir, "mail")}); code != 0 {
		t.Fatalf("gx wc build of one package = %d", code)
	}
	entries, err := os.ReadDir(one)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if got := strings.Join(names, " "); got != "acme-composer.d.ts acme-composer.js acme-composer.react.d.ts custom-elements.json" {
		t.Errorf("files of one package = %s", got)
	}
	// With no server the element uses the origin of the host page.
	data, err := os.ReadFile(filepath.Join(one, "acme-composer.js"))
	if err != nil || !strings.Contains(string(data), `"server":"","path":"/widgets/composer"`) {
		t.Errorf("the element file with no server: %v", err)
	}

	// A server that is not an origin, and a directory with no widget, stop
	// the command.
	if code := gxcli.Main([]string{"wc", "build", "--server", "api.acme.dev", "--out", one, dir}); code == 0 {
		t.Error("gx wc build took a server with no scheme")
	}
	if code := gxcli.Main([]string{"wc", "build", "--out", one, filepath.Join(dir, "cart", "route")}); code == 0 {
		t.Error("gx wc build of a directory with no widget gave no error")
	}
}
