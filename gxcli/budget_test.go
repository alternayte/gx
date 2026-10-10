package gxcli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
	"github.com/alternayte/gx/internal/gxconfig"
	"github.com/alternayte/gx/internal/gxstyles"
)

// TestREQ_DEV_13_BudgetInGxCheck checks the budget of gx.toml in gx check: a
// page over its budget is GX9001 with the files of the page, and the check
// fails; a larger budget passes; and an app with no budget has no finding
// (REQ-DEV-13).
func TestREQ_DEV_13_BudgetInGxCheck(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=mod")
	dir := scratchModule(t, map[string]string{
		"home/route/route.go":   "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n",
		"home/page.go":          "package home\n\nimport (\n\t\"github.com/alternayte/gx\"\n\t\"app/home/route\"\n)\n\nvar Page = gx.Page(func(c *gx.Ctx, in route.Home) (ViewProps, error) {\n\treturn ViewProps{}, nil\n}, View)\n\nvar Routes = gx.Collect(Page)\n",
		"home/View.gx":          "package home\n\nsignals {\n  Open bool = false\n}\n\n<main>\n  <button on:click={$Open = !$Open}>Menu</button>\n  <p show={$Open}>A page with a signal</p>\n</main>\n",
		"gxstyles/styles_gx.go": string(gxstyles.Generate([]byte(".rule{color:red}"))),
		"cmd/app/main.go":       "package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/adapters/datastar\"\n\t\"app/gxstyles\"\n\t\"app/home\"\n)\n\nfunc main() {\n\tgx.SetStylesheet(gxstyles.CSS())\n\tapp := gx.New(gx.Config{Adapter: datastar.Adapter()})\n\tapp.Group(\"/\", home.Routes)\n\t_ = http.ListenAndServe(os.Getenv(\"GX_DEV_ADDR\"), app)\n}\n",
	})
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate exit = %d", code)
	}
	check := func(toml string) (int, string) {
		t.Helper()
		path := filepath.Join(dir, "gx.toml")
		if toml == "" {
			_ = os.Remove(path)
		} else if err := os.WriteFile(path, []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
		code := 0
		out := captureStderr(t, func() { code = gxcli.Main([]string{"check", dir}) })
		return code, out
	}

	// No budget: no check and no finding.
	if code, out := check(""); code != 0 || strings.Contains(out, "GX9001") {
		t.Fatalf("no budget: exit %d\n%s", code, out)
	}
	// The page has a signal, so it loads the runtime and the adapter.
	code, out := check("[budget]\njs = 1000\n")
	if code != 1 || !strings.Contains(out, "GX9001") {
		t.Fatalf("a JS budget of 1000 bytes: exit %d, want 1 with GX9001\n%s", code, out)
	}
	for _, want := range []string{"gx.toml:1:1", "the route GET /", "bytes of JS (gzipped)", "its budget is 1000", "/_gx/gx."} {
		if !strings.Contains(out, want) {
			t.Errorf("the finding lacks %q:\n%s", want, out)
		}
	}
	// A budget that the page is inside, and the numbers of one route.
	if code, out := check("[budget]\njs = 1000\n\n[budget.routes.\"GET /{$}\"]\njs = 500000\n"); code != 0 {
		t.Errorf("the route has its own budget of 500000 bytes: exit %d\n%s", code, out)
	}

	// The table of gx.toml, as the check reads it.
	if err := os.WriteFile(filepath.Join(dir, "gx.toml"), []byte("[budget]\njs = 20000\ncss = 30000\n\n[budget.routes.\"GET /basket\"]\ncss = 9000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := gxconfig.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	route := cfg.Budget.Routes["GET /basket"]
	if cfg.Budget.JS != 20000 || cfg.Budget.CSS != 30000 || route.CSS != 9000 || route.JS != -1 || !cfg.Budget.Set() {
		t.Errorf("budget = %+v, want the two defaults and the CSS of one route", cfg.Budget)
	}
}
