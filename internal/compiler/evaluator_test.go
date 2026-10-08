package compiler_test

import (
	"encoding/json"
	"fmt"
	"html"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// exprGen writes random client expressions of each type. Each expression
// has two texts: the text of the .gx file, with signals as $Name and props
// as p.Name, and the same expression in Go, with a variable for each signal
// and prop.
type exprGen struct {
	r *rand.Rand
}

type genExpr struct{ gx, goText string }

func (g *exprGen) pick(options ...func() genExpr) genExpr { return options[g.r.Intn(len(options))]() }

func signal(name string) func() genExpr {
	return func() genExpr { return genExpr{"$" + name, "s" + name} }
}

func prop(name string) func() genExpr {
	return func() genExpr { return genExpr{"p." + name, "p" + name} }
}

func lit(text string) func() genExpr {
	return func() genExpr { return genExpr{text, text} }
}

func (g *exprGen) binary(op string, l, r genExpr) genExpr {
	return genExpr{"(" + l.gx + " " + op + " " + r.gx + ")", "(" + l.goText + " " + op + " " + r.goText + ")"}
}

func (g *exprGen) call(name string, args ...genExpr) genExpr {
	var a, b []string
	for _, arg := range args {
		a = append(a, arg.gx)
		b = append(b, arg.goText)
	}
	return genExpr{"gxc." + name + "(" + strings.Join(a, ", ") + ")", "gxc." + name + "(" + strings.Join(b, ", ") + ")"}
}

// nonZeroInt is a divisor. Each signal and each literal here is not zero.
func (g *exprGen) nonZeroInt() genExpr {
	return g.pick(signal("A"), signal("B"), signal("C"), lit(strconv.Itoa(g.r.Intn(9)+1)), lit("("+strconv.Itoa(-g.r.Intn(9)-1)+")"))
}

func (g *exprGen) intExpr(depth int) genExpr {
	leaf := []func() genExpr{signal("A"), signal("B"), signal("C"), prop("K"), lit(strconv.Itoa(g.r.Intn(101) - 50))}
	if depth == 0 {
		return g.pick(leaf...)
	}
	return g.pick(
		func() genExpr { return g.pick(leaf...) },
		func() genExpr { return g.binary("+", g.intExpr(depth-1), g.intExpr(depth-1)) },
		func() genExpr { return g.binary("-", g.intExpr(depth-1), g.intExpr(depth-1)) },
		func() genExpr { return g.binary("*", g.intExpr(depth-1), g.intExpr(depth-1)) },
		func() genExpr { return g.binary("/", g.intExpr(depth-1), g.nonZeroInt()) },
		func() genExpr { return g.binary("%", g.intExpr(depth-1), g.nonZeroInt()) },
		func() genExpr {
			e := g.intExpr(depth - 1)
			return genExpr{"(-(" + e.gx + "))", "(-(" + e.goText + "))"}
		},
		func() genExpr { return g.call("Len", g.stringExpr(depth-1)) },
		func() genExpr { return g.call("Index", g.stringExpr(depth-1), g.stringExpr(0)) },
	)
}

func (g *exprGen) floatExpr(depth int) genExpr {
	leaf := []func() genExpr{signal("X"), signal("Y"), prop("F"), lit(strconv.FormatFloat(float64(g.r.Intn(4001)-2000)/16, 'f', 4, 64))}
	if depth == 0 {
		return g.pick(leaf...)
	}
	return g.pick(
		func() genExpr { return g.pick(leaf...) },
		func() genExpr { return g.binary("+", g.floatExpr(depth-1), g.floatExpr(depth-1)) },
		func() genExpr { return g.binary("-", g.floatExpr(depth-1), g.floatExpr(depth-1)) },
		func() genExpr { return g.binary("*", g.floatExpr(depth-1), g.floatExpr(depth-1)) },
		func() genExpr {
			return g.binary("/", g.floatExpr(depth-1), lit(strconv.FormatFloat(float64(g.r.Intn(60)+1)/8, 'f', 3, 64))())
		},
		func() genExpr {
			e := g.floatExpr(depth - 1)
			return genExpr{"(-(" + e.gx + "))", "(-(" + e.goText + "))"}
		},
	)
}

func (g *exprGen) stringExpr(depth int) genExpr {
	words := []string{`""`, `"a"`, `"wö"`, `"llo"`, `"日本"`, `"o w"`, `"<b>&\"q\"</b>"`}
	leaf := []func() genExpr{signal("S"), signal("T"), prop("W"), lit(words[g.r.Intn(len(words))])}
	if depth == 0 {
		return g.pick(leaf...)
	}
	return g.pick(
		func() genExpr { return g.pick(leaf...) },
		func() genExpr { return g.binary("+", g.stringExpr(depth-1), g.stringExpr(depth-1)) },
		func() genExpr { return g.call("At", g.stringExpr(depth-1), lit(strconv.Itoa(g.r.Intn(14)-1))()) },
	)
}

func (g *exprGen) boolExpr(depth int) genExpr {
	leaf := []func() genExpr{signal("P"), signal("Q"), lit("true"), lit("false")}
	if depth == 0 {
		return g.pick(leaf...)
	}
	compare := func(ops []string, operand func(int) genExpr) func() genExpr {
		return func() genExpr { return g.binary(ops[g.r.Intn(len(ops))], operand(depth-1), operand(depth-1)) }
	}
	order := []string{"==", "!=", "<", "<=", ">", ">="}
	equal := []string{"==", "!="}
	return g.pick(
		func() genExpr { return g.pick(leaf...) },
		func() genExpr { e := g.boolExpr(depth - 1); return genExpr{"(!" + e.gx + ")", "(!" + e.goText + ")"} },
		compare([]string{"&&", "||"}, g.boolExpr),
		compare(order, g.intExpr),
		compare(order, g.floatExpr),
		compare(equal, g.stringExpr),
		compare(equal, g.boolExpr),
		func() genExpr { return g.call("Contains", g.stringExpr(depth-1), g.stringExpr(0)) },
	)
}

// TestSI_15_EvaluatorMatchesGo checks that the evaluator of the widget
// script gives each client expression the value that Go gives it
// (SI-15, REQ-ACT-13). The compiler writes the trees: the test generates a
// component with 800 random expressions, renders it for a widget, and runs
// each tree of the render through runtime/js/widget-eval.ts in Bun. The Go
// value of the same expression comes from the Go compiler.
func TestSI_15_EvaluatorMatchesGo(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatalf("bun is not on PATH: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	g := &exprGen{r: rand.New(rand.NewSource(15))}
	var cases []genExpr
	for len(cases) < 800 {
		var e genExpr
		switch len(cases) % 4 {
		case 0:
			e = g.intExpr(2)
		case 1:
			e = g.floatExpr(2)
		case 2:
			e = g.stringExpr(2)
		default:
			e = g.boolExpr(2)
		}
		// A client expression reads a signal. An expression with no
		// signal is a server expression.
		if strings.Contains(e.gx, "$") {
			cases = append(cases, e)
		}
	}

	var gxFile, wants strings.Builder
	gxFile.WriteString("package calc\n\nimport \"github.com/alternayte/gx/gxc\"\n\nprops {\n  K int\n  F float64\n  W string\n}\n\n" +
		"signals {\n  A int = 7\n  B int = -3\n  C int = 1003\n  X float64 = 2.5\n  Y float64 = -0.75\n  S string = \"héllo wörld 日本\"\n  T string = \"wö\"\n  P bool = true\n  Q bool = false\n}\n\n<div>\n")
	for _, c := range cases {
		gxFile.WriteString("  <i text={" + c.gx + "}></i>\n")
		wants.WriteString("\t\t" + c.goText + ",\n")
	}
	gxFile.WriteString("</div>\n")
	dump := "package calc\n\nimport (\n\t\"encoding/json\"\n\t\"net/http/httptest\"\n\t\"os\"\n\t\"testing\"\n\n\t\"github.com/alternayte/gx\"\n\t\"github.com/alternayte/gx/gxc\"\n)\n\n" +
		"var _ = gxc.Len\n\n" +
		"func TestDump(t *testing.T) {\n" +
		"\tsA, sB, sC := 7, -3, 1003\n\tsX, sY := 2.5, -0.75\n\tsS, sT := \"héllo wörld 日本\", \"wö\"\n\tsP, sQ := true, false\n" +
		"\tpK, pF, pW := 12, 1.25, \"o wö\"\n" +
		"\t_, _, _, _, _, _, _, _, _, _, _, _ = sA, sB, sC, sX, sY, sS, sT, sP, sQ, pK, pF, pW\n" +
		"\twant := []any{\n" + wants.String() + "\t}\n" +
		"\treq := httptest.NewRequest(\"GET\", \"/\", nil)\n\treq.Header.Set(\"Gx-Widget\", \"x-calc\")\n" +
		"\tout := map[string]any{\"html\": gx.StringRequest(req, Calc(CalcProps{K: pK, F: pF, W: pW})), \"want\": want}\n" +
		"\tdata, err := json.Marshal(out)\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n" +
		"\tif err := os.WriteFile(os.Getenv(\"GX_DUMP\"), data, 0o644); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"

	dir := writeTree(t, map[string]string{
		"go.mod":            moduleWithGx(t),
		"calc/Calc.gx":      gxFile.String(),
		"calc/dump_test.go": dump,
	})
	for path, src := range generateFiles(t, dir) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dumpPath := filepath.Join(t.TempDir(), "dump.json")
	// The random expressions have forms that vet reports, as "p && p".
	cmd := exec.Command("go", "test", "-vet=off", "./calc")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GX_DUMP="+dumpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test of the generated module: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatal(err)
	}
	var dumped struct {
		HTML string `json:"html"`
		Want []any  `json:"want"`
	}
	if err := json.Unmarshal(raw, &dumped); err != nil {
		t.Fatal(err)
	}

	// The trees and the first values of the signals, as the render for a
	// widget holds them.
	attr := func(name string) []string {
		var out []string
		for _, m := range regexp.MustCompile(` `+name+`="([^"]*)"`).FindAllStringSubmatch(dumped.HTML, -1) {
			out = append(out, html.UnescapeString(m[1]))
		}
		return out
	}
	trees, signals := attr("data-gx-text"), attr("data-gx-signals")
	if len(trees) != len(cases) || len(signals) != 1 {
		t.Fatalf("the render has %d trees for %d expressions, and %d signal attributes", len(trees), len(cases), len(signals))
	}
	if strings.Contains(dumped.HTML, "data-text") || strings.Contains(dumped.HTML, "$[") {
		t.Fatalf("the render for a widget holds the text of an adapter")
	}

	work := t.TempDir()
	harness := filepath.Join(work, "harness.ts")
	if err := os.WriteFile(harness, []byte(`import { readFileSync, writeFileSync } from 'node:fs'
import { evaluate } from `+strconv.Quote(filepath.ToSlash(filepath.Join(repo, "runtime", "js", "widget-eval.ts")))+`
const input = JSON.parse(readFileSync(process.argv[2], 'utf8'))
const read = (path: string[]): unknown => path.reduce((node: any, part) => (node == null ? undefined : node[part]), input.signals)
const out = input.trees.map((tree: string) => {
  try {
    return evaluate(JSON.parse(tree), read)
  } catch (e) {
    return 'ERROR: ' + (e as Error).message
  }
})
writeFileSync(process.argv[3], JSON.stringify(out))
`), 0o644); err != nil {
		t.Fatal(err)
	}
	inPath, outPath := filepath.Join(work, "in.json"), filepath.Join(work, "out.json")
	input, err := json.Marshal(map[string]any{"trees": trees, "signals": json.RawMessage(signals[0])})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inPath, input, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("bun", harness, inPath, outPath).CombinedOutput(); err != nil {
		t.Fatalf("bun: %v\n%s", err, out)
	}
	raw, err = os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(cases) {
		t.Fatalf("bun returned %d results for %d expressions", len(got), len(cases))
	}
	kinds := map[string]int{}
	for i, c := range cases {
		wantJSON, _ := json.Marshal(dumped.Want[i])
		gotJSON, _ := json.Marshal(got[i])
		if string(wantJSON) == "-0" {
			// JSON of JavaScript has no negative zero: it writes 0.
			// The two values are equal numbers.
			wantJSON = []byte("0")
		}
		if string(wantJSON) != string(gotJSON) {
			t.Fatalf("%s\n  tree %s\n  Go = %s, widget script = %s", c.gx, trees[i], wantJSON, gotJSON)
		}
		kinds[fmt.Sprintf("%T", dumped.Want[i])]++
	}
	// Each type of value is in the run.
	for _, kind := range []string{"float64", "string", "bool"} {
		if kinds[kind] < 100 {
			t.Errorf("only %d expressions with a %s value: %v", kinds[kind], kind, kinds)
		}
	}
}
