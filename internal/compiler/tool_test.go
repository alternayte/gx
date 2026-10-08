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

// TestREQ_AI_07_FormAndActionAsTools runs a generated app: an action and a
// form with .Tool() are tools of the app, a call binds the arguments through
// the generated binder, and the rules of the form answer a bad call.
func TestREQ_AI_07_FormAndActionAsTools(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": moduleWithGx(t),
		"account/route/route.go": "package route\n\nimport \"github.com/alternayte/gx\"\n\n" +
			"type Address struct {\n\tCity string\n}\n\n" +
			"type Signup struct {\n\tgx.Route `POST /signup`\n\tEmail string\n\tAge int\n\tHome Address\n\tTags []string\n}\n\n" +
			"func (s *Signup) Rules() gx.Rules {\n\treturn gx.Rules{gx.Field(&s.Email, gx.Required, gx.Email), gx.Field(&s.Age, gx.Min(18))}\n}\n\n" +
			"type Done struct {\n\tgx.Route `GET /done`\n}\n\n" +
			"type Rename struct {\n\tgx.Route `POST /users/{id}/name`\n\tID int64\n\tName string\n\tLoud bool `query:\"loud\"`\n}\n",
		"account/SignupView.gx": "package account\n\nimport \"app/account/route\"\n\nprops {\n  F route.SignupForm\n}\n\n<form {...p.F.Attrs()}><input {...p.F.Email.Attrs()} /><button type=\"submit\">Go</button></form>\n",
		"account/account.go": "package account\n\nimport (\n\t\"strconv\"\n\n\t\"app/account/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"var Seen []route.Signup\n\n" +
			"// Makes an account for a new user.\nvar signup = gx.Form(func(c *gx.Ctx, in *route.Signup) error {\n\tSeen = append(Seen, *in)\n\tif in.Email == \"taken@x.example\" {\n\t\treturn gx.FieldError(&in.Email, \"email.taken\")\n\t}\n\treturn c.Redirect(route.Done{})\n}, SignupView).Tool()\n\n" +
			"// Gives a user a new name.\nvar rename = gx.Action(func(c *gx.Ctx, in route.Rename) error {\n\tgx.ToolResult(c, map[string]string{\"name\": in.Name, \"id\": strconv.FormatInt(in.ID, 10), \"loud\": strconv.FormatBool(in.Loud)})\n\treturn nil\n}).Tool(gx.Confirm)\n\n" +
			"var Routes = gx.Collect(signup, rename)\n",
		"account/account_test.go": "package account\n\nimport (\n\t\"context\"\n\t\"encoding/json\"\n\t\"strings\"\n\t\"testing\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
			"func TestTools(t *testing.T) {\n\tapp := gx.New(gx.Config{})\n\tapp.Group(\"/\", Routes)\n" +
			"\tvar names []string\n\tfor _, info := range app.Tools() {\n\t\tnames = append(names, info.Name+\": \"+info.Description)\n\t}\n" +
			"\tif got := strings.Join(names, \"; \"); got != \"account_rename: Gives a user a new name.; account_signup: Makes an account for a new user.\" {\n\t\tt.Fatalf(\"tools = %s\", got)\n\t}\n" +
			"\tcall := func(name, args string) gx.ToolAnswer {\n\t\treturn app.CallTool(context.Background(), nil, name, json.RawMessage(args))\n\t}\n" +
			"\ta := call(\"account_signup\", `{\"email\":\"a@b.example\",\"age\":30,\"home\":{\"city\":\"Oslo\"},\"tags\":[\"x\",\"y\"]}`)\n" +
			"\tif a.IsError || a.Text != \"redirect to /done\" || len(Seen) != 1 || Seen[0].Email != \"a@b.example\" || Seen[0].Age != 30 || Seen[0].Home.City != \"Oslo\" || strings.Join(Seen[0].Tags, \",\") != \"x,y\" {\n\t\tt.Fatalf(\"signup: %+v, seen %+v\", a, Seen)\n\t}\n" +
			"\ta = call(\"account_signup\", `{\"email\":\"nope\",\"age\":12}`)\n" +
			"\tif !a.IsError || len(Seen) != 1 || !strings.Contains(a.Text, \"field email\") || !strings.Contains(a.Text, \"field age\") {\n\t\tt.Fatalf(\"a call that breaks two rules: %+v\", a)\n\t}\n" +
			"\ta = call(\"account_signup\", `{\"email\":\"taken@x.example\",\"age\":30}`)\n" +
			"\tif !a.IsError || !strings.Contains(a.Text, \"email.taken\") {\n\t\tt.Fatalf(\"a field error of the handler: %+v\", a)\n\t}\n" +
			"\ta = call(\"account_signup\", `{\"email\":\"a@b.example\",\"age\":\"old\"}`)\n" +
			"\tif !a.IsError || len(Seen) != 2 {\n\t\tt.Fatalf(\"an argument of the wrong type: %+v, seen %d\", a, len(Seen))\n\t}\n" +
			"\ta = call(\"account_rename\", `{\"id\":7,\"name\":\"Ada & Bo\",\"loud\":true}`)\n" +
			"\tif a.IsError || string(a.Structured) != `{\"id\":\"7\",\"loud\":\"true\",\"name\":\"Ada \\u0026 Bo\"}` {\n\t\tt.Fatalf(\"rename: %+v\", a)\n\t}\n" +
			"\tif a = call(\"account_nothing\", `{}`); !a.IsError {\n\t\tt.Fatalf(\"a name with no tool: %+v\", a)\n\t}\n}\n",
	})
	buildGenerated(t, dir, "test", "./...")
}

// TestREQ_AI_08_SecretInToolResult checks GX7002 for a gx.Secret in the
// value of gx.ToolResult: the result goes to an agent (SI-04).
func TestREQ_AI_08_SecretInToolResult(t *testing.T) {
	src := "package products\n\nimport (\n\t\"app/products/route\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
		"type result struct {\n\tName string `json:\"name\"`\n\tKey gx.Secret `json:\"key\"`\n}\n\n" +
		"// Does a thing.\nvar plain = gx.Action(func(c *gx.Ctx, in route.Plain) error {\n\tgx.ToolResult(c, result{Name: \"a\"})\n\treturn nil\n}).Tool()\n\n" +
		"var Routes = gx.Collect(plain)\n"
	diags := checkDir(t, writeTree(t, map[string]string{
		"go.mod":                  moduleWithGx(t),
		"products/route/route.go": toolRoutes,
		"products/products.go":    src,
	}))
	if got := codesOf(diags, "GX7002"); len(got) != 1 || !strings.Contains(got[0].Msg, "Key") {
		t.Errorf("a secret in a tool result: %v", diags)
	}
}
