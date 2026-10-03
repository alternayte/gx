package compiler

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestREQ_ACT_13_Differential runs every allowed operator shape in Go and in
// JavaScript on the same random inputs and compares the results
// (REQ-ACT-13). It pins the semantics that jsOp and the int-division rule
// promise.
func TestREQ_ACT_13_Differential(t *testing.T) {
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatalf("bun is required for the differential test: %v", err)
	}
	rng := rand.New(rand.NewSource(13))
	type diffCase struct {
		Expr string `json:"expr"`
		Want any    `json:"-"`
	}
	var cases []diffCase
	add := func(expr string, want any) {
		cases = append(cases, diffCase{Expr: expr, Want: want})
	}
	intLit := func() int64 { return int64(rng.Intn(2_000_001) - 1_000_000) }
	floatLit := func() float64 { return float64(rng.Intn(2_000_001)-1_000_000) / 1000 }
	strLit := func() string {
		n := rng.Intn(8)
		b := make([]byte, n)
		for i := range b {
			b[i] = byte('a' + rng.Intn(26))
		}
		return string(b)
	}
	boolLit := func() bool { return rng.Intn(2) == 0 }

	for i := 0; i < 1200; i++ {
		switch i % 10 {
		case 0:
			a, b := intLit(), intLit()
			if b == 0 {
				b = 1
			}
			op := []string{"+", "-", "*", "/", "%"}[rng.Intn(5)]
			expr := fmt.Sprintf("(%d) %s (%d)", a, op, b)
			var want any
			switch op {
			case "+":
				want = a + b
			case "-":
				want = a - b
			case "*":
				want = a * b
			case "/":
				want = a / b
				expr = fmt.Sprintf("Math.trunc((%d) / (%d))", a, b)
			case "%":
				want = a % b
			}
			add(expr, want)
		case 1:
			a, b := intLit(), intLit()
			op := []string{"<", "<=", ">", ">=", "===", "!=="}[rng.Intn(6)]
			var want bool
			switch op {
			case "<":
				want = a < b
			case "<=":
				want = a <= b
			case ">":
				want = a > b
			case ">=":
				want = a >= b
			case "===":
				want = a == b
			case "!==":
				want = a != b
			}
			add(fmt.Sprintf("(%d) %s (%d)", a, op, b), want)
		case 2:
			a, b := floatLit(), floatLit()
			op := []string{"+", "-", "*"}[rng.Intn(3)]
			var want float64
			switch op {
			case "+":
				want = a + b
			case "-":
				want = a - b
			case "*":
				want = a * b
			}
			add(fmt.Sprintf("(%v) %s (%v)", a, op, b), want)
		case 3:
			a, b := floatLit(), floatLit()
			op := []string{"<", "<=", ">", ">=", "===", "!=="}[rng.Intn(6)]
			var want bool
			switch op {
			case "<":
				want = a < b
			case "<=":
				want = a <= b
			case ">":
				want = a > b
			case ">=":
				want = a >= b
			case "===":
				want = a == b
			case "!==":
				want = a != b
			}
			add(fmt.Sprintf("(%v) %s (%v)", a, op, b), want)
		case 4:
			a, b := strLit(), strLit()
			op := []string{"+", "<", "<=", ">", ">=", "===", "!=="}[rng.Intn(7)]
			var want any
			switch op {
			case "+":
				want = a + b
			case "<":
				want = a < b
			case "<=":
				want = a <= b
			case ">":
				want = a > b
			case ">=":
				want = a >= b
			case "===":
				want = a == b
			case "!==":
				want = a != b
			}
			add(fmt.Sprintf("(%q) %s (%q)", a, op, b), want)
		case 5:
			a, b := boolLit(), boolLit()
			op := []string{"&&", "||", "===", "!=="}[rng.Intn(4)]
			var want bool
			switch op {
			case "&&":
				want = a && b
			case "||":
				want = a || b
			case "===":
				want = a == b
			case "!==":
				want = a != b
			}
			add(fmt.Sprintf("(%t) %s (%t)", a, op, b), want)
		case 6:
			a := boolLit()
			add(fmt.Sprintf("!(%t)", a), !a)
		case 7:
			a := intLit()
			add(fmt.Sprintf("-(%d)", a), -a)
		case 8:
			a := floatLit()
			add(fmt.Sprintf("-(%v)", a), -a)
		case 9:
			a := intLit()
			b := 2
			add(fmt.Sprintf("Math.trunc((%d) / (%d))", a, b), a/int64(b))
		}
	}

	// The JS side runs the same operator strings through the browser engine.
	dir := t.TempDir()
	harness := filepath.Join(dir, "harness.js")
	if err := os.WriteFile(harness, []byte(`const fs = require('fs')
const cases = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'))
const out = cases.map((c) => {
  try { return Function('return (' + c.expr + ')')() } catch (e) { return 'ERROR: ' + e.message }
})
fs.writeFileSync(process.argv[3], JSON.stringify(out))
`), 0o644); err != nil {
		t.Fatal(err)
	}
	casesPath := filepath.Join(dir, "cases.json")
	outPath := filepath.Join(dir, "out.json")
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casesPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bun", harness, casesPath, outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bun: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(cases) {
		t.Fatalf("bun returned %d results for %d cases", len(got), len(cases))
	}
	for i, c := range cases {
		wantJSON, _ := json.Marshal(c.Want)
		gotJSON, _ := json.Marshal(got[i])
		if string(wantJSON) != string(gotJSON) {
			t.Fatalf("%s: Go = %s, JS = %s", c.Expr, wantJSON, gotJSON)
		}
	}
}
