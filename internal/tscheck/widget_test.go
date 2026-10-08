package tscheck_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/tscheck"
)

// fakeReact is the part of the React types that the JSX types of a widget
// use. The test needs no install of a package.
const fakeReact = `declare module "react" {
  export interface HTMLAttributes<T> {
    className?: string;
    id?: string;
  }
  export type DetailedHTMLProps<A, E> = A & { ref?: (el: E | null) => void };
  export namespace JSX {
    interface Element {}
    interface IntrinsicElements {
      div: DetailedHTMLProps<HTMLAttributes<HTMLDivElement>, HTMLDivElement>;
    }
  }
  export function createElement(type: unknown, props?: unknown, ...children: unknown[]): JSX.Element;
}
`

const hostConfig = `{
  "compilerOptions": {
    "strict": true,
    "noEmit": true,
    "target": "es2022",
    "module": "esnext",
    "moduleResolution": "bundler",
    "lib": ["es2022", "dom"],
    "jsx": "preserve",
    "types": []
  },
  "include": ["*.ts", "*.tsx"]
}
`

// TestREQ_ISL_17_WidgetTypesCheckInAHost checks the type files of gx wc
// build in a host project, with the pinned TypeScript compiler: the element,
// its token, its attributes and its events have types, and the detail of a
// domain event has the type of the Go struct. A wrong use is an error.
func TestREQ_ISL_17_WidgetTypesCheckInAHost(t *testing.T) {
	app := writeTree(t, map[string]string{
		"go.mod":              "module app\n\ngo 1.25.0\n\nrequire github.com/alternayte/gx v0.0.0\n\nreplace github.com/alternayte/gx => " + filepath.ToSlash(repoRoot(t)) + "\n",
		"cart/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Widget struct {\n\tgx.Route `GET /cart`\n\tCurrency string `query:\"currency\" default:\"EUR\"`\n\tCompact  bool   `query:\"compact\"`\n\tLimit    int    `query:\"limit\"`\n}\n",
		"cart/Cart.gx":        "package cart\n\nprops {\n  Currency string\n}\n\n<section>{p.Currency}</section>\n",
		"cart/cart.go": "package cart\n\nimport (\n\t\"app/cart/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"type Line struct {\n\tSKU string `json:\"sku\"`\n\tQty int    `json:\"qty\"`\n}\n\n" +
			"type ChangedDetail struct {\n\tCount int    `json:\"count\"`\n\tLines []Line `json:\"lines\"`\n}\n\nvar Changed = gx.Event[ChangedDetail](\"cart-changed\")\n\n" +
			"var CartWidget = gx.Widget(func(c *gx.Ctx, in route.Widget) (CartProps, error) {\n\treturn CartProps{Currency: in.Currency}, nil\n}, Cart).Tag(\"acme-cart\")\n\nvar Routes = gx.Collect(CartWidget)\n",
		"main.go": "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n" +
			"\tapp.Group(\"/widgets\", gx.AllowOrigins(gx.AnyOrigin), cart.Routes)\n\t_ = app\n}\n",
	})
	widgets, diags := compiler.Widgets(app)
	if len(diags) != 0 || len(widgets) != 1 {
		t.Fatalf("widgets %d, diagnostics %v", len(widgets), diags)
	}
	bin, err := (&tscheck.Manager{Root: app}).Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// check runs the compiler on a host project with the type files of
	// the widget and the given sources, and returns its error lines.
	check := func(sources map[string]string) []string {
		t.Helper()
		host := t.TempDir()
		files := map[string]string{
			"tsconfig.json":        hostConfig,
			"react.d.ts":           fakeReact,
			"acme-cart.d.ts":       string(widgets[0].DTS),
			"acme-cart.react.d.ts": string(widgets[0].ReactDTS),
		}
		for name, src := range sources {
			files[name] = src
		}
		for name, src := range files {
			if err := os.WriteFile(filepath.Join(host, name), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(bin, "--project", host, "--pretty", "false")
		cmd.Dir = host
		out, _ := cmd.CombinedOutput()
		var errs []string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "error TS") {
				errs = append(errs, line)
			}
		}
		if len(errs) == 0 && strings.TrimSpace(string(out)) != "" {
			t.Fatalf("the compiler wrote text that is no error line:\n%s", out)
		}
		return errs
	}

	good := map[string]string{
		"host.ts": `import "./acme-cart";
import type { ChangedDetail, Line } from "./acme-cart";

const cart = document.querySelector("acme-cart")!;
cart.token = "text";
cart.token = async () => "fresh";
cart.addEventListener("cart-changed", (e) => {
  const count: number = e.detail.count;
  const lines: Line[] = e.detail.lines;
  const detail: ChangedDetail = e.detail;
  console.log(count, lines[0]?.sku, detail);
});
cart.addEventListener("gx-error", (e) => console.log(e.detail.status + 1, e.detail.key.length));
cart.addEventListener("gx-navigate", (e) => console.log(e.detail.url.length));
cart.addEventListener("gx-ready", () => {});
cart.addEventListener("click", (e) => console.log(e.clientX));
`,
		// The React file has its own name: a compiler takes one of host.ts
		// and host.tsx only.
		"page.tsx": `import * as React from "react";
import "./acme-cart.react";

export const Cart = () => (
  <div className="page">
    <acme-cart
      id="cart"
      currency="USD"
      compact
      limit={3}
      oncart-changed={(e) => console.log(e.detail.count + 1)}
      ongx-error={(e) => console.log(e.detail.status)}
      ongx-ready={() => {}}
    />
  </div>
);
void React;
`,
	}
	if errs := check(good); len(errs) != 0 {
		t.Fatalf("a correct host has type errors:\n%s", strings.Join(errs, "\n"))
	}

	bad := map[string]string{
		"host.ts": `import "./acme-cart";

const cart = document.querySelector("acme-cart")!;
cart.addEventListener("cart-changed", (e) => {
  const count: string = e.detail.count;
  console.log(count, e.detail.total);
});
cart.token = 42;
`,
		"page.tsx": `import * as React from "react";
import "./acme-cart.react";

export const A = () => <acme-cart colour="red" />;
export const B = () => <acme-cart limit="three" />;
export const C = () => <acme-cart oncart-changed={(e) => console.log(e.detail.total)} />;
void React;
`,
	}
	errs := strings.Join(check(bad), "\n")
	for _, want := range []string{
		"host.ts(5,",  // the count is a number
		"host.ts(6,",  // the detail has no total
		"host.ts(8,",  // the token is text or a function
		"page.tsx(4,", // an attribute that the widget does not have
		"page.tsx(5,", // a number attribute with text
		"page.tsx(6,", // a field that the detail does not have
	} {
		if !strings.Contains(errs, want) {
			t.Errorf("no type error at %s\n%s", want, errs)
		}
	}
}
