package gxcli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/alternayte/gx/gxcli"
)

// schemaErrors checks a JSON value against the subset of JSON Schema the
// describe schema uses: type, required, properties, additionalProperties,
// items, enum and $ref into $defs.
func schemaErrors(root, schema map[string]any, value any, path string) []string {
	if ref, ok := schema["$ref"].(string); ok {
		defs, _ := root["$defs"].(map[string]any)
		target, ok := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		if !ok {
			return []string{path + ": unknown $ref " + ref}
		}
		return schemaErrors(root, target, value, path)
	}
	var errs []string
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if e == value {
				found = true
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("%s: %v is not in the enum", path, value))
		}
	}
	switch schema["type"] {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return append(errs, path+": not an object")
		}
		props, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, r := range required {
				if _, ok := obj[r.(string)]; !ok {
					errs = append(errs, path+": lacks "+r.(string))
				}
			}
		}
		for key, v := range obj {
			sub, ok := props[key].(map[string]any)
			if !ok {
				if schema["additionalProperties"] == false {
					errs = append(errs, path+": unknown property "+key)
				}
				continue
			}
			errs = append(errs, schemaErrors(root, sub, v, path+"."+key)...)
		}
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return append(errs, path+": not an array")
		}
		if items, ok := schema["items"].(map[string]any); ok {
			for i, v := range arr {
				errs = append(errs, schemaErrors(root, items, v, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			errs = append(errs, path+": not a string")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			errs = append(errs, path+": not a boolean")
		}
	default:
		errs = append(errs, fmt.Sprintf("%s: schema type %v is not supported by the test", path, schema["type"]))
	}
	return errs
}

// TestREQ_AI_01_DescribeShop covers `gx describe --json` on the example
// app: the output matches the published schema, holds every part of the
// app model, and equals the snapshot (REQ-AI-01).
func TestREQ_AI_01_DescribeShop(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	shop := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "examples", "shop"))

	rawSchema, code := captureStdout(t, func() int { return gxcli.Main([]string{"describe", "--schema"}) })
	if code != 0 {
		t.Fatalf("gx describe --schema exit = %d", code)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(rawSchema), &schema); err != nil {
		t.Fatalf("the schema is not JSON: %v", err)
	}

	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"describe", "--json", shop}) })
	if code != 0 {
		t.Fatalf("gx describe --json exit = %d\n%s", code, out)
	}
	var model any
	if err := json.Unmarshal([]byte(out), &model); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if errs := schemaErrors(schema, schema, model, "$"); len(errs) > 0 {
		sort.Strings(errs)
		if len(errs) > 20 {
			errs = errs[:20]
		}
		t.Fatalf("the model does not match the schema:\n%s", strings.Join(errs, "\n"))
	}

	// The schema must be closed, or the check above proves nothing.
	if errs := schemaErrors(schema, schema, map[string]any{"module": 1, "extra": true}, "$"); len(errs) < 3 {
		t.Fatalf("the schema accepts a bad model: %v", errs)
	}

	var got struct {
		Module     string `json:"module"`
		Components []struct {
			Name    string `json:"name"`
			Package string `json:"package"`
			File    string `json:"file"`
			Props   []struct {
				Name, Type, Default, Doc string
				Required                 bool
			}
			Signals   []struct{ Name, Type, Default string }
			Fragments []struct {
				Name, Func string
				Keyed      bool
				Params     []struct{ Name, Type string }
			}
			Fixtures []string
		}
		Routes []struct {
			Type, Method, Pattern, Kind, Handler string
			Layouts                              []string
			Fields                               []struct{ Name, Type, Source, Key string }
		}
		Actions []struct {
			Handler, Route string
			Signals        []string
		}
		Forms []struct {
			Handler, Route string
			Fields         []struct {
				Name  string
				Rules []string
			}
		}
		Islands     []struct{ Name, Package, File string }
		Elements    []struct{ Name, Package, Tag string }
		Transitions []struct{ Name, Base, Key string }
		Icons       []struct{ Set, Version string }
		Registry    []struct {
			Name, Version string
			Files         []string
		}
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Module != "github.com/alternayte/gx/examples/shop" {
		t.Fatalf("module = %q", got.Module)
	}

	// A component with props, a default, a doc, signals, fragments and
	// fixtures.
	var cart, button bool
	for _, c := range got.Components {
		switch c.Package + "." + c.Name {
		case "github.com/alternayte/gx/examples/shop/cart.Cart":
			cart = true
			if c.File != "cart/Cart.gx" || len(c.Signals) == 0 || c.Signals[0].Name != "Qty" || c.Signals[0].Type != "int" {
				t.Fatalf("cart.Cart = %+v", c)
			}
			var total bool
			for _, f := range c.Fragments {
				if f.Name == "total" {
					total = true
					if f.Func != "CartTotal" || !f.Keyed || len(f.Params) == 0 || f.Params[0].Type != "gx.Key" {
						t.Fatalf("cart.Cart fragment total = %+v", f)
					}
				}
			}
			if !total {
				t.Fatalf("cart.Cart has no fragment total: %+v", c.Fragments)
			}
		case "github.com/alternayte/gx/examples/shop/ui/button.Button":
			button = true
			if len(c.Fixtures) == 0 {
				t.Fatalf("button.Button has no fixtures: %+v", c)
			}
			var variant, doc bool
			for _, p := range c.Props {
				if p.Name == "Variant" && !p.Required && p.Default != "" {
					variant = true
				}
				if p.Doc != "" {
					doc = true
				}
			}
			if !variant || !doc {
				t.Fatalf("button.Button props = %+v", c.Props)
			}
		}
	}
	if !cart || !button {
		t.Fatalf("the model lacks cart.Cart or button.Button (%d components)", len(got.Components))
	}

	kinds := map[string]bool{}
	for _, r := range got.Routes {
		kinds[r.Kind] = true
		if r.Type == "github.com/alternayte/gx/examples/shop/route.Home" {
			if r.Kind != "page" || r.Handler != "shop.HomePage" || r.Method != "GET" || len(r.Layouts) == 0 {
				t.Fatalf("route.Home = %+v", r)
			}
		}
	}
	for _, kind := range []string{"page", "action", "form"} {
		if !kinds[kind] {
			t.Fatalf("no route of kind %s", kind)
		}
	}
	var add, signalAction bool
	for _, a := range got.Actions {
		if a.Handler == "cart.Add" {
			add = true
		}
		if len(a.Signals) > 0 {
			signalAction = true
		}
	}
	if !add || !signalAction {
		t.Fatalf("actions = %+v", got.Actions)
	}
	if len(got.Forms) != 1 || got.Forms[0].Handler != "signup.Create" {
		t.Fatalf("forms = %+v", got.Forms)
	}
	rules := map[string][]string{}
	for _, f := range got.Forms[0].Fields {
		rules[f.Name] = f.Rules
	}
	if strings.Join(rules["email"], " ") != "gx.Required gx.Email gx.MaxLen(254)" || len(rules["address.street"]) != 1 {
		t.Fatalf("form rules = %+v", rules)
	}
	if _, ok := rules["addresses[].street"]; !ok {
		t.Fatalf("the repeated address fields are missing: %+v", rules)
	}
	// The dashboard slice of the shop holds the islands (REQ-ISL-01).
	// The registry items in ui/ have islands too (REQ-REG-14); this test
	// reads the ones of the dashboard.
	var islandNames []string
	for _, isl := range got.Islands {
		if isl.Package != "github.com/alternayte/gx/examples/shop/dashboard" {
			continue
		}
		islandNames = append(islandNames, isl.Name)
		if isl.File != "dashboard/"+isl.Name+".ts" {
			t.Fatalf("island = %+v", isl)
		}
	}
	if strings.Join(islandNames, " ") != "BarChart Legend Sparkline Stepper WideTable" {
		t.Fatalf("islands = %v", islandNames)
	}
	// The dashboard uses two imported web components (REQ-ISL-09).
	if len(got.Elements) != 2 || got.Elements[0].Tag != "sl-badge" || got.Elements[1].Tag != "sl-details" ||
		got.Elements[0].Package != "github.com/alternayte/gx/examples/shop/ui/sl" {
		t.Fatalf("elements = %+v", got.Elements)
	}
	if len(got.Transitions) == 0 || got.Transitions[0].Base == "" || got.Transitions[0].Key == "" {
		t.Fatalf("transitions = %+v", got.Transitions)
	}
	var registryButton bool
	for _, item := range got.Registry {
		if item.Name == "button" && item.Version != "" && len(item.Files) > 0 {
			registryButton = true
		}
	}
	if !registryButton {
		t.Fatalf("registry lacks button: %d items", len(got.Registry))
	}

	snapshot(t, "ai01_describe_shop.golden.json", out)
}

// TestREQ_AI_01_DescribeIcons covers the icon sets of the model: a pinned
// set in gx.lock is in the output, and the text form lists the counts
// (REQ-AI-01).
func TestREQ_AI_01_DescribeIcons(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"ui/card/Card.gx": "package card\n\nprops {\n  Title string\n}\n\n<article>{p.Title}</article>\n",
		"gx.lock":         "{\n  \"icons\": {\n    \"lucide\": {\"version\": \"1.2.3\", \"sha256\": \"00\"}\n  }\n}\n",
	})
	out, code := captureStdout(t, func() int { return gxcli.Main([]string{"describe", "--json", dir}) })
	if code != 0 {
		t.Fatalf("gx describe --json exit = %d\n%s", code, out)
	}
	var got struct {
		Icons []struct{ Set, Version string }
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Icons) != 1 || got.Icons[0].Set != "lucide" || got.Icons[0].Version != "1.2.3" {
		t.Fatalf("icons = %+v", got.Icons)
	}
	text, code := captureStdout(t, func() int { return gxcli.Main([]string{"describe", dir}) })
	if code != 0 || !strings.Contains(text, "components  1") || !strings.Contains(text, "card.Card") {
		t.Fatalf("gx describe = %d\n%s", code, text)
	}

	// A module with an error prints its diagnostics and fails.
	if err := os.WriteFile(filepath.Join(dir, "ui/card/Page.gx"), []byte("package card\n\n<Card />\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := captureStdout(t, func() int { return gxcli.Main([]string{"describe", "--json", dir}) }); code != 1 {
		t.Fatalf("gx describe on a broken module exit = %d, want 1", code)
	}
}
