package site_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/content"
	"github.com/alternayte/gx/docs/registry"
	"github.com/alternayte/gx/docs/site"
)

// registryFixtures reads the registry source itself: every item name, and
// per item every "Component/Fixture" of its fixtures files.
func registryFixtures(t *testing.T, dir string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	manifests, err := filepath.Glob(filepath.Join(dir, "*", "gx-item.json"))
	if err != nil || len(manifests) == 0 {
		t.Fatalf("no registry items under %s: %v", dir, err)
	}
	for _, path := range manifests {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &m); err != nil || m.Name == "" {
			t.Fatalf("%s: no item name: %v", path, err)
		}
		out[m.Name] = nil
		files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.fixtures.go"))
		for _, file := range files {
			comp := strings.TrimSuffix(filepath.Base(file), ".fixtures.go")
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				index, ok := lit.Type.(*ast.IndexExpr)
				if !ok {
					return true
				}
				if sel, ok := index.X.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Fixtures" {
					return true
				}
				for _, elt := range lit.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.BasicLit); ok {
							if name, err := strconv.Unquote(key.Value); err == nil {
								out[m.Name] = append(out[m.Name], comp+"/"+name)
							}
						}
					}
				}
				return false
			})
		}
	}
	return out
}

// tags matches one HTML tag. The highlighter wraps the words of a code line.
var tags = regexp.MustCompile(`<[^>]*>`)

// get renders one path of the app.
func get(t *testing.T, app http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, rec.Code)
	}
	return rec.Body.String()
}

// TestREQ_DOC_02_EveryRegistryItemHasAPage renders the docs app: every
// registry item has a page, the index links to it, and every fixture of the
// item shows on the page and renders in its preview (REQ-DOC-02).
func TestREQ_DOC_02_EveryRegistryItemHasAPage(t *testing.T) {
	t.Chdir("..") // the collection reads content/ under the module root
	content.Install()
	app := gx.New(gx.Config{Adapter: datastar.Adapter()})
	app.Group("/", site.Routes)

	index := get(t, app, "/components/")
	for name, fixtures := range registryFixtures(t, filepath.Join("..", "registry")) {
		it, ok := registry.Find(name)
		if !ok {
			t.Errorf("item %s is not in the generated item list; run just docs-gen", name)
			continue
		}
		href := "/components/" + name + "/"
		if !strings.Contains(index, `href="`+href+`"`) {
			t.Errorf("the index does not link to %s", href)
		}
		page := get(t, app, href)
		if !strings.Contains(page, html.EscapeString(it.Description)) {
			t.Errorf("%s does not show the item description", href)
		}
		if !strings.Contains(tags.ReplaceAllString(page, ""), "gx add "+name) {
			t.Errorf("%s does not show the install command", href)
		}
		for _, fixture := range fixtures {
			var ex *registry.Example
			for i := range it.Examples {
				for _, f := range it.Examples[i].Fixtures {
					if f == fixture {
						ex = &it.Examples[i]
					}
				}
			}
			if ex == nil {
				t.Errorf("fixture %s of %s is not in the generated item list; run just docs-gen", fixture, name)
				continue
			}
			preview := "/preview/" + name + "/" + ex.Name + "/"
			shown := `src="` + preview + `"`
			if it.IconSet {
				shown = ">" + ex.Component + "</code>"
			}
			if !strings.Contains(page, shown) {
				t.Errorf("%s does not show fixture %s (%s)", href, fixture, shown)
			}
			if body := get(t, app, preview); !strings.Contains(body, `class="gx-stage`) {
				t.Errorf("%s has no stage", preview)
			}
		}
	}
}
