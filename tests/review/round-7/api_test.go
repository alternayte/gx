package round7_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/internal/compiler"
)

// findIn is the input of the action Find of the fixture module, written by
// hand: it has the Bind and the GxTool that the compiler writes for
//
//	type Find struct {
//		gx.Route `GET /search/find`
//		Term string `signal:"term"`
//	}
type findIn struct{ Term string }

func (findIn) Pattern() string { return "GET /search/find" }

func (in *findIn) Bind(r *http.Request) error {
	signals, err := gx.Signals(r)
	if err != nil {
		return err
	}
	return gx.BindSignal(signals, gx.Scope(r), "term", &in.Term)
}

func (findIn) GxTool() gx.ToolInfo {
	return gx.ToolInfo{
		Name:   "search_find",
		Schema: `{"additionalProperties":false,"properties":{"term":{"maxLength":40,"type":"string"}},"type":"object"}`,
		Fields: []gx.ToolField{{Name: "term", In: "signal"}},
	}
}

// TestREQ_ACT_20_ClientCallOfAGetActionWithATextArgument checks REQ-ACT-20:
// "`gx api` writes an OpenAPI 3.1 file and a TypeScript client for the
// marked actions" with the acceptance "A call of the client against the
// shop returns the typed result", and REQ-ACT-19: "The input comes from the
// JSON body, the path and the query".
//
// The action has the method GET and one argument of type string that binds
// from a signal. The client of `gx api` writes an argument of a method with
// no body into the query as String(value): the term "123" is ?term=123.
// The server reads a signal argument of the query as JSON: 123 is a number,
// which is no value of a string field, and the call gets 400. A term such
// as "true" or "2026" has the same answer, and the term "null" arrives as
// the empty string.
func TestREQ_ACT_20_ClientCallOfAGetActionWithATextArgument(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"search/route/r.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\n// Find reads the products of a search.\ntype Find struct {\n\tgx.Route `GET /search/find`\n\tTerm string `signal:\"term\"`\n}\n\n// Rules limits the term.\nfunc (in *Find) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&in.Term, gx.MaxLen(40))}\n}\n",
		"search/search.go":  "package search\n\nimport (\n\t\"app/search/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n// Found is the result of a search.\ntype Found struct {\n\tTerm string `json:\"term\"`\n}\n\n// Finds the products of a search term.\nvar Find = gx.Action(func(c *gx.Ctx, in route.Find) error {\n\tgx.ToolResult(c, Found{Term: in.Term})\n\treturn nil\n}).API()\n\nvar Routes = gx.Collect(Find)\n",
		"main.go":           "package main\n\nimport (\n\t\"app/search\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", search.Routes)\n}\n",
	})
	files, diags := compiler.APIFiles(dir)
	if len(diags) > 0 || len(files["client.ts"]) == 0 {
		t.Fatalf("the fixture is wrong: diagnostics %v, files %d", diags, len(files))
	}
	client := filepath.Join(t.TempDir(), "client.ts")
	if err := os.WriteFile(client, files["client.ts"], 0o644); err != nil {
		t.Fatal(err)
	}

	// The server: the action of the fixture, which gives its term back.
	app := gx.New(gx.Config{Adapter: &recAdapter{}})
	app.Group("/", gx.Collect(gx.Action(func(c *gx.Ctx, in findIn) error {
		gx.ToolResult(c, map[string]string{"term": in.Term})
		return nil
	}).API()))
	srv := httptest.NewServer(app)
	defer srv.Close()

	for _, term := range []string{"tea", "123", "true", "null"} {
		got := runBun(t, `
import { createClient } from `+strconv.Quote(filepath.ToSlash(client))+`
const api = createClient({ baseURL: `+strconv.Quote(srv.URL)+` })
try {
  console.log(JSON.stringify(await api.searchFind({ term: `+strconv.Quote(term)+` })))
} catch (err) {
  console.log(JSON.stringify({ status: err.status, errors: err.errors }))
}
`)
		var result struct {
			Term   *string `json:"term"`
			Status int     `json:"status"`
		}
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("the client printed %q: %v", got, err)
		}
		if term == "tea" && (result.Term == nil || *result.Term != "tea") {
			t.Fatalf("the fixture is wrong: the call with the term tea gives %s", got)
		}
		if result.Term == nil || *result.Term != term {
			t.Errorf("searchFind({term: %q}) gives %s, want the result with the term %q", term, got, term)
		}
	}
}
