package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

const apiRoute = `package cart

import "github.com/alternayte/gx"

type Add struct {
	gx.Route ` + "`" + `POST /cart/{id}/add` + "`" + `
	ID   int64
	Note string ` + "`" + `query:"note"` + "`" + `
	Qty  int
}

func (in *Add) Rules() gx.Rules {
	return gx.Rules{gx.Field(&in.Qty, gx.Required, gx.Max(9))}
}

type Plain struct {
	gx.Route ` + "`" + `POST /cart/plain` + "`" + `
}
`

const apiAction = `package cart

import "github.com/alternayte/gx"

type Added struct {
	Total int      ` + "`" + `json:"total"` + "`" + `
	Tags  []string ` + "`" + `json:"tags,omitempty"` + "`" + `
}

// Adds a product to the cart.
var add = gx.Action(func(c *gx.Ctx, in Add) error {
	gx.ToolResult(c, Added{Total: in.Qty})
	return nil
}).API()

var plain = gx.Action(func(c *gx.Ctx, in Plain) error { return nil })

var Routes = gx.Collect(add, plain)
`

// TestREQ_ACT_20_APIFiles checks the two files of gx api for a module: the
// OpenAPI file and the TypeScript client hold each action with .API() and no
// other action, with the path, the query, the body, the rules and the result
// (REQ-ACT-20).
func TestREQ_ACT_20_APIFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": apiRoute,
		"cart/action.go": apiAction,
		"main.go":        "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tgx.New(gx.Config{}).Group(\"/\", cart.Routes)\n}\n",
	})
	files, diags := compiler.APIFiles(dir)
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string           `json:"operationId"`
			Description string           `json:"description"`
			Parameters  []map[string]any `json:"parameters"`
			RequestBody struct {
				Content map[string]struct {
					Schema map[string]any `json:"schema"`
				} `json:"content"`
			} `json:"requestBody"`
			Responses map[string]map[string]any `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(files["openapi.json"], &doc); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}
	if doc.OpenAPI != "3.1.0" || len(doc.Paths) != 1 {
		t.Fatalf("openapi %q with the paths %v, want 3.1.0 with the one marked action", doc.OpenAPI, doc.Paths)
	}
	op, ok := doc.Paths["/cart/{id}/add"]["post"]
	if !ok {
		t.Fatalf("no POST /cart/{id}/add in %s", files["openapi.json"])
	}
	if op.OperationID != "cart_add" || op.Description != "Adds a product to the cart." {
		t.Errorf("operation %q, %q", op.OperationID, op.Description)
	}
	in := map[string]string{}
	for _, p := range op.Parameters {
		in[p["name"].(string)] = p["in"].(string)
	}
	if in["id"] != "path" || in["note"] != "query" || len(in) != 2 {
		t.Errorf("parameters = %v, want id in the path and note in the query", in)
	}
	body := op.RequestBody.Content["application/json"].Schema
	props, _ := body["properties"].(map[string]any)
	qty, _ := props["qty"].(map[string]any)
	if len(props) != 1 || qty["type"] != "integer" || qty["maximum"] != float64(9) {
		t.Errorf("body schema = %v, want qty as an integer with the maximum of its rule", body)
	}
	if req, _ := body["required"].([]any); len(req) != 1 || req[0] != "qty" {
		t.Errorf("required of the body = %v, want qty", body["required"])
	}
	for _, status := range []string{"200", "204", "422", "default"} {
		if op.Responses[status] == nil {
			t.Errorf("no response %s", status)
		}
	}

	client := string(files["client.ts"])
	for _, want := range []string{
		"// Adds a product to the cart.\nexport type CartAddInput = {\n  id: number\n  note?: string\n  qty: number\n}\n",
		"export type CartAddResult = {\n  tags?: string[]\n  total: number\n}\n",
		"cartAdd: (input: CartAddInput): Promise<CartAddResult> =>\n      call<CartAddResult>(options, \"POST\", `/cart/${encodeURIComponent(String(input.id))}/add`, { \"note\": input.note }, { \"qty\": input.qty }),",
	} {
		if !strings.Contains(client, want) {
			t.Errorf("client.ts lacks:\n%s", want)
		}
	}
	if strings.Contains(client, "Plain") || strings.Contains(string(files["openapi.json"]), "plain") {
		t.Error("the files hold an action with no .API()")
	}
	if t.Failed() {
		t.Logf("client.ts:\n%s", client)
	}
}

// TestREQ_ACT_20_StaleAPIFile checks that gx check fails for a file of gx
// api that differs from the actions, and has no finding for a module that
// never ran gx api (REQ-ACT-20).
func TestREQ_ACT_20_StaleAPIFile(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":         moduleWithGx(t),
		"cart/routes.go": apiRoute,
		"cart/action.go": apiAction,
		"main.go":        "package main\n\nimport (\n\t\"app/cart\"\n\n\t\"github.com/alternayte/gx\"\n)\n\nfunc main() {\n\tgx.New(gx.Config{}).Group(\"/\", cart.Routes)\n}\n",
	})
	writeGenerated(t, dir)
	if diags := compiler.CheckApp(dir, compiler.CheckOptions{}); len(diags) != 0 {
		t.Fatalf("a module with no file of gx api: %v", diags)
	}
	files, _ := compiler.APIFiles(dir)
	for name, data := range files {
		path := filepath.Join(dir, compiler.APIDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if diags := compiler.CheckApp(dir, compiler.CheckOptions{}); len(diags) != 0 {
		t.Fatalf("fresh files: %v", diags)
	}
	// The action gets a new field: the two files are stale.
	changed := strings.Replace(apiRoute, "\tQty  int\n", "\tQty  int\n\tGift bool\n", 1)
	if err := os.WriteFile(filepath.Join(dir, "cart/routes.go"), []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGenerated(t, dir)
	stale := map[string]bool{}
	for _, d := range compiler.CheckApp(dir, compiler.CheckOptions{}) {
		if d.Code != compiler.CodeStale || !strings.Contains(d.Msg, "gx api") {
			t.Fatalf("an unexpected diagnostic: %s", d.String())
		}
		stale[filepath.Base(d.File)] = true
	}
	if !stale["client.ts"] || !stale["openapi.json"] {
		t.Errorf("stale files = %v, want client.ts and openapi.json", stale)
	}
}

// TestREQ_ACT_20_ShopFiles checks that the committed files of the shop are
// the files that gx api writes: they are the golden files (REQ-ACT-20).
func TestREQ_ACT_20_ShopFiles(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	shop := filepath.Join(filepath.Dir(file), "..", "..", "examples", "shop")
	files, diags := compiler.APIFiles(shop)
	if len(diags) > 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	for _, name := range []string{"openapi.json", "client.ts"} {
		onDisk, err := os.ReadFile(filepath.Join(shop, compiler.APIDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(onDisk, files[name]) {
			t.Errorf("examples/shop/api/%s is stale; run gx api examples/shop", name)
		}
	}
	client := string(files["client.ts"])
	for _, want := range []string{"basketSetQty: (input: BasketSetQtyInput): Promise<BasketSetQtyResult>", "basketCount: (): Promise<BasketCountResult>"} {
		if !strings.Contains(client, want) {
			t.Errorf("the client of the shop lacks %s", want)
		}
	}
}
