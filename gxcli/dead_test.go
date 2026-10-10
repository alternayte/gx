package gxcli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// deadModule has one of each: a component with no caller, a page route with
// no typed link and a class of the theme that no file uses. It also has the
// same three things in use.
func deadModule(t *testing.T) string {
	t.Helper()
	return scratchModule(t, map[string]string{
		"site/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Home struct {\n\tgx.Route `GET /{$}`\n}\n\ntype About struct {\n\tgx.Route `GET /about`\n}\n\ntype Legal struct {\n\tgx.Route `GET /legal`\n}\n",
		"site/Home.gx":        "package site\n\nimport \"app/site/route\"\n\n<main class=\"page\">\n  <Used />\n  <a href={route.About{}}>About</a>\n</main>\n",
		"site/About.gx":       "package site\n\n<main class=\"page\"><h1>About</h1></main>\n",
		"site/Legal.gx":       "package site\n\n<main><h1>Legal</h1></main>\n",
		"site/Used.gx":        "package site\n\n<p class=\"note\">Used</p>\n",
		"site/FromGo.gx":      "package site\n\n<p>From Go</p>\n",
		"site/Unused.gx":      "package site\n\n<p>Unused</p>\n",
		"site/Unused.fixtures.go": "package site\n\nimport \"github.com/alternayte/gx\"\n\nvar UnusedFixtures = gx.Fixtures[UnusedProps]{\"default\": {}}\n",
		"site/site.go": "package site\n\nimport (\n\t\"app/site/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"var HomePage = gx.Page(func(c *gx.Ctx, in route.Home) (HomeProps, error) { return HomeProps{}, nil }, Home)\n" +
			"var AboutPage = gx.Page(func(c *gx.Ctx, in route.About) (AboutProps, error) { return AboutProps{}, nil }, About)\n" +
			"var LegalPage = gx.Page(func(c *gx.Ctx, in route.Legal) (LegalProps, error) { return LegalProps{}, nil }, Legal)\n\n" +
			"func Extra() gx.Node { return FromGo(FromGoProps{}) }\n\n" +
			"var Routes = gx.Collect(HomePage, AboutPage, LegalPage)\n",
		"app/theme.css": "@import \"tailwindcss\";\n\n/* .comment is not a class */\n@utility note {\n  color: red;\n}\n\n@utility stale-box {\n  color: blue;\n}\n\n.page {\n  margin: 0;\n}\n\n.old-banner,\n.older-banner {\n  display: none;\n}\n",
	})
}

// TestREQ_DEV_14_DeadCodeReport checks gx check --dead: it prints the
// components with no caller, the page routes with no typed link and the
// classes of the theme that no file uses, and it does not change the exit
// code of the check (REQ-DEV-14).
func TestREQ_DEV_14_DeadCodeReport(t *testing.T) {
	dir := deadModule(t)
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate exit = %d", code)
	}
	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"check", "--dead", dir}) })
	if code != 0 {
		t.Fatalf("gx check --dead exit = %d, want the exit code of the check, 0\n%s", code, out)
	}
	want := `Dead code. This is a report; it does not fail the check.

Components with no caller: 1
  Unused (site/Unused.gx)

Page routes with no typed link: 2
  GET /legal (app/site/route.Legal)
  GET /{$} (app/site/route.Home)

Classes of app/theme.css that no file uses: 3
  .old-banner
  .older-banner
  .stale-box
`
	if out != want {
		t.Errorf("the report:\n%s\nwant:\n%s", out, want)
	}

	// The machine form: the diagnostics of the check and the report.
	out, code = captureStdout(t, func() int { return gxcli.Main([]string{"check", "--dead", "--json", dir}) })
	var got struct {
		Diagnostics []any `json:"diagnostics"`
		Dead        struct {
			Components []struct{ Name, Package, File string }
			Routes     []struct{ Pattern, Type string }
			Classes    []string
		} `json:"dead"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 0 {
		t.Fatalf("--json: exit %d, %v\n%s", code, err, out)
	}
	if len(got.Diagnostics) != 0 || len(got.Dead.Components) != 1 || got.Dead.Components[0].Name != "Unused" ||
		got.Dead.Components[0].Package != "site" || len(got.Dead.Routes) != 2 || strings.Join(got.Dead.Classes, " ") != "old-banner older-banner stale-box" {
		t.Errorf("--json: %+v", got)
	}

	// With no flag, the check prints no report.
	out, _ = captureStdout(t, func() int { return gxcli.Main([]string{"check", dir}) })
	if strings.Contains(out, "Dead code") {
		t.Errorf("gx check with no --dead prints the report:\n%s", out)
	}
}

// TestREQ_DEV_14_ReportKeepsTheExitCode checks that the report does not hide
// a failure of the check: a module with an error and with dead code exits 1
// with the report (REQ-DEV-14).
func TestREQ_DEV_14_ReportKeepsTheExitCode(t *testing.T) {
	dir := deadModule(t)
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate exit = %d", code)
	}
	// An image with no alt is an error of the check (REQ-AUT-23).
	writeFile(t, dir, "site/Legal.gx", "package site\n\n<main><h1>Legal</h1><img src=\"/a.png\"></main>\n")
	if code := gxcli.Main([]string{"generate", dir}); code != 0 {
		t.Fatalf("gx generate exit = %d", code)
	}
	var code int
	var out string
	stderr := captureStderr(t, func() {
		out, code = captureStdout(t, func() int { return gxcli.Main([]string{"check", "--dead", dir}) })
	})
	if code != 1 || !strings.Contains(stderr, "GX2016") || !strings.Contains(out, "Components with no caller: 1") {
		t.Errorf("exit %d, want 1 with GX2016 and the report\nstderr:\n%s\nstdout:\n%s", code, stderr, out)
	}
}
