package compiler_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const toolRoutes = "package route\n\nimport (\n\t\"regexp\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
	"type Line struct {\n\tSKU string\n\tQty int\n}\n\n" +
	"type AddToCart struct {\n\tgx.Route `POST /products/{id}/cart`\n\tID int64\n\tTab string `query:\"tab\" default:\"overview\"`\n\tQty int `signal:\"qty\"`\n" +
	"\tEmail string\n\tSite string\n\tCode string\n\tPlan string\n\tNote string\n\tTerms bool\n\tPrice float64\n\tTags []string\n\tLines []Line\n\tSecret string `bind:\"-\"`\n}\n\n" +
	"func (in *AddToCart) Rules() gx.Rules {\n\treturn gx.Rules{\n" +
	"\t\tgx.Field(&in.Qty, gx.Min(1), gx.Max(99)),\n" +
	"\t\tgx.Field(&in.Email, gx.Required, gx.Email, gx.MaxLen(254)),\n" +
	"\t\tgx.Field(&in.Site, gx.IsURL),\n" +
	"\t\tgx.Field(&in.Code, gx.Pattern(regexp.MustCompile(`^[A-Z]{3}$`))),\n" +
	"\t\tgx.Field(&in.Plan, gx.OneOf(\"free\", \"pro\")),\n" +
	"\t\tgx.Field(&in.Note, gx.MinLen(2), gx.Check(func(v any) error { return nil })),\n" +
	"\t\tgx.Field(&in.Terms, gx.True(\"terms.required\")),\n" +
	"\t\tgx.Field(&in.Tags, gx.Each(gx.MaxLen(20))),\n" +
	"\t}\n}\n\n" +
	"type Plain struct {\n\tgx.Route `POST /plain`\n}\n"

const toolGo = "package products\n\nimport (\n\t\"app/products/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
	"// Adds a product to the cart of the user.\n// The answer has the new count.\nvar addToCart = gx.Action(func(c *gx.Ctx, in route.AddToCart) error { return nil }).Tool()\n\n" +
	"var plain = gx.Action(func(c *gx.Ctx, in route.Plain) error { return nil })\n\n" +
	"var Routes = gx.Collect(addToCart, plain)\n"

var toolInfoRe = regexp.MustCompile(`(?s)func \(AddToCart\) GxTool\(\) gx\.ToolInfo \{.*?Name:\s+("(?:[^"\\]|\\.)*"),.*?Description:\s+("(?:[^"\\]|\\.)*"),.*?Schema:\s+("(?:[^"\\]|\\.)*"),.*?Fields:\s+\[\]gx\.ToolField\{(.*?)\},\n`)

// TestREQ_AI_09_RulesBecomeJSONSchema checks the JSON Schema that the
// compiler writes for the input of a tool: each field with its JSON type,
// and each rule with a JSON Schema form as that keyword.
func TestREQ_AI_09_RulesBecomeJSONSchema(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":                  moduleWithGx(t),
		"products/route/route.go": toolRoutes,
		"products/products.go":    toolGo,
	})
	buildGenerated(t, dir, "build", "./...")
	src, err := os.ReadFile(filepath.Join(dir, "products", "route", "route_gx.go"))
	if err != nil {
		t.Fatal(err)
	}
	m := toolInfoRe.FindSubmatch(src)
	if m == nil {
		t.Fatalf("no GxTool method of AddToCart in:\n%s", src)
	}
	unquote := func(b []byte) string {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			t.Fatalf("%s: %v", b, err)
		}
		return s
	}
	if name := unquote(m[1]); name != "products_add_to_cart" {
		t.Errorf("name = %q", name)
	}
	if desc := unquote(m[2]); desc != "Adds a product to the cart of the user. The answer has the new count." {
		t.Errorf("description = %q", desc)
	}
	var got any
	if err := json.Unmarshal([]byte(unquote(m[3])), &got); err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}
	var want any
	if err := json.Unmarshal([]byte(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["id", "email"],
		"properties": {
			"id": {"type": "integer"},
			"tab": {"type": "string", "default": "overview"},
			"qty": {"type": "integer", "minimum": 1, "maximum": 99},
			"email": {"type": "string", "format": "email", "maxLength": 254},
			"site": {"type": "string", "format": "uri"},
			"code": {"type": "string", "pattern": "^[A-Z]{3}$"},
			"plan": {"type": "string", "enum": ["free", "pro"]},
			"note": {"type": "string", "minLength": 2},
			"terms": {"type": "boolean", "const": true},
			"price": {"type": "number"},
			"tags": {"type": "array", "items": {"type": "string", "maxLength": 20}},
			"lines": {"type": "array", "items": {"type": "object", "additionalProperties": false, "properties": {"sKU": {"type": "string"}, "qty": {"type": "integer"}}}}
		}
	}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		g, _ := json.MarshalIndent(got, "", "  ")
		t.Errorf("schema =\n%s", g)
	}
	fields := strings.Join(strings.Fields(string(m[4])), " ")
	for _, want := range []string{`{Name: "id", In: "path"}`, `{Name: "tab", In: "query"}`, `{Name: "qty", In: "signal"}`, `{Name: "email", In: "form"}`, `{Name: "lines", In: "form"}`} {
		if !strings.Contains(fields, want) {
			t.Errorf("fields have no %s: %s", want, fields)
		}
	}
	// A route type with no tool has no tool method.
	if strings.Contains(string(src), "func (Plain) GxTool()") {
		t.Error("a route type with no .Tool() has a GxTool method")
	}
}

// TestREQ_AI_09_ToolDiagnostic checks GX4011: a tool needs a doc comment for
// its description, and an input that a JSON value can fill.
func TestREQ_AI_09_ToolDiagnostic(t *testing.T) {
	noDoc := strings.Replace(toolGo, "// Adds a product to the cart of the user.\n// The answer has the new count.\n", "", 1)
	diags := checkDir(t, writeTree(t, map[string]string{
		"go.mod":                  moduleWithGx(t),
		"products/route/route.go": toolRoutes,
		"products/products.go":    noDoc,
	}))
	if got := codesOf(diags, "GX4011"); len(got) != 1 || !strings.Contains(got[0].Msg, "doc comment") {
		t.Errorf("a tool with no doc comment: %v", diags)
	}
	upload := "package route\n\nimport \"github.com/alternayte/gx\"\n\ntype Upload struct {\n\tgx.Route `POST /upload`\n\tDoc gx.File\n}\n"
	uploadGo := "package up\n\nimport (\n\t\"app/up/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n// Uploads a file.\nvar upload = gx.Action(func(c *gx.Ctx, in route.Upload) error { return nil }).Tool()\n\nvar Routes = gx.Collect(upload)\n"
	diags = checkDir(t, writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"up/route/route.go": upload,
		"up/up.go":          uploadGo,
	}))
	if got := codesOf(diags, "GX4011"); len(got) != 1 || !strings.Contains(got[0].Msg, "gx.File") {
		t.Errorf("a tool with a file field: %v", diags)
	}
}
