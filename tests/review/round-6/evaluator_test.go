package round6_test

import (
	"encoding/json"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/alternayte/gx/internal/compiler"
)

// orderGx compares two strings with the four order operators. The signal S
// starts with a fullwidth letter (U+FF21, three bytes of UTF-8 and one code
// unit of UTF-16). The prop W starts with a character above U+FFFF (U+20BB7,
// in Japanese names; four bytes of UTF-8 and two code units of UTF-16).
const orderGx = "package calc\n\nprops {\n  W string\n}\n\nsignals {\n  S string = \"Ａ列\"\n}\n\n<div>\n" +
	"  <i text={$S < p.W}></i>\n  <i text={$S <= p.W}></i>\n  <i text={$S > p.W}></i>\n  <i text={$S >= p.W}></i>\n</div>\n"

// orderDump renders the component for a widget and writes the HTML and the
// Go value of each expression.
const orderDump = "package calc\n\nimport (\n\t\"encoding/json\"\n\t\"net/http/httptest\"\n\t\"os\"\n\t\"testing\"\n\n\t\"github.com/alternayte/gx\"\n)\n\n" +
	"func TestDump(t *testing.T) {\n" +
	"\tsS, pW := \"Ａ列\", \"𠮷野\"\n" +
	"\twant := []any{sS < pW, sS <= pW, sS > pW, sS >= pW}\n" +
	"\treq := httptest.NewRequest(\"GET\", \"/\", nil)\n\treq.Header.Set(\"Gx-Widget\", \"x-calc\")\n" +
	"\tout := map[string]any{\"html\": gx.StringRequest(req, Calc(CalcProps{W: pW})), \"want\": want}\n" +
	"\tdata, err := json.Marshal(out)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n" +
	"\tif err := os.WriteFile(os.Getenv(\"GX_DUMP\"), data, 0o644); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"

// TestREQ_ACT_13_StringOrderInAWidgetMatchesGo checks the order operators
// on strings in the evaluator of a widget (DR-05: "Client expressions allow
// only types and operators where Go and JS give the same result";
// REQ-ACT-13; SI-15: the widget script evaluates each client expression, and
// runtime/js/widget-eval.ts says "Each node has the result of the same
// expression in Go").
//
// Go compares the bytes of UTF-8. The evaluator uses < of JavaScript, which
// compares the code units of UTF-16. The two orders differ for a character
// above U+FFFF against a character from U+E000 to U+FFFF: the first has a
// surrogate (0xD800 to 0xDFFF) as its first code unit, and the larger first
// byte in UTF-8. The compiler takes < on two strings in a client expression
// with no diagnostic. The differential test of the build has strings below
// U+FFFF only. The compiler must refuse the order of strings (GX4007), or
// the evaluator must give the order of Go.
func TestREQ_ACT_13_StringOrderInAWidgetMatchesGo(t *testing.T) {
	dir := scratchModule(t, map[string]string{
		"calc/Calc.gx":      orderGx,
		"calc/dump_test.go": orderDump,
	})
	files, diags := compiler.Generate(dir)
	if len(diags) > 0 {
		// The compiler refuses the order of two strings in a client
		// expression: no result can differ.
		for _, d := range diags {
			if d.Code != "GX4007" {
				t.Fatalf("the fixture is wrong: %v", diags)
			}
		}
		return
	}
	for path, src := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dump := filepath.Join(t.TempDir(), "dump.json")
	cmd := exec.Command("go", "test", "-vet=off", "./calc")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GX_DUMP="+dump)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test of the generated module: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	var dumped struct {
		HTML string `json:"html"`
		Want []bool `json:"want"`
	}
	if err := json.Unmarshal(raw, &dumped); err != nil {
		t.Fatal(err)
	}
	attr := func(name string) []string {
		var out []string
		for _, m := range regexp.MustCompile(` `+name+`="([^"]*)"`).FindAllStringSubmatch(dumped.HTML, -1) {
			out = append(out, html.UnescapeString(m[1]))
		}
		return out
	}
	trees, signals := attr("data-gx-text"), attr("data-gx-signals")
	if len(trees) != 4 || len(signals) != 1 {
		t.Fatalf("the fixture is wrong: the render has %d trees and %d signal attributes:\n%s", len(trees), len(signals), dumped.HTML)
	}
	input, err := json.Marshal(map[string]any{"trees": trees, "signals": json.RawMessage(signals[0])})
	if err != nil {
		t.Fatal(err)
	}
	got := runBun(t, `
import { evaluate } from "%EVAL%"
const input = JSON.parse(process.env.GX_INPUT)
const read = (path) => path.reduce((node, part) => (node == null ? undefined : node[part]), input.signals)
console.log(JSON.stringify(input.trees.map((tree) => evaluate(JSON.parse(tree), read))))
`, "GX_INPUT="+string(input))
	var results []bool
	if err := json.Unmarshal([]byte(got), &results); err != nil {
		t.Fatalf("%v: %s", err, got)
	}
	ops := []string{"<", "<=", ">", ">="}
	for i, want := range dumped.Want {
		if results[i] != want {
			t.Errorf("$S %s p.W with $S = \"Ａ列\" and p.W = \"𠮷野\": Go gives %v, and the evaluator of the widget gives %v for the tree %s", ops[i], want, results[i], trees[i])
		}
	}
}
