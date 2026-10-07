// Package interpcase_test holds the blocking findings of review round 4 on
// the dev interpreter (REQ-DEV-05, P7). Every test fails on the reviewed
// HEAD 05bda5b.
//
// REQ-DEV-05 requires that interpreted and compiled code render the same
// bytes. The interpreter accepts each function of sample.go (Compile gives
// no error, so the dev server swaps the code and does not rebuild), and
// then gives a value that differs from the value of the compiled function.
package interpcase_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/interp"
	"github.com/alternayte/gx/tests/review/round-4/interpcase"
)

// differ runs one sample as compiled code and as interpreted code and
// fails when the two values differ. The interpreted source is the one
// function, cut from sample.go. A function that the interpreter refuses is
// not a difference: the dev server then rebuilds (DR-03).
func differ(t *testing.T, name string) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "sample.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var src strings.Builder
	src.WriteString("package interpcase\n\nimport (\n\t\"fmt\"\n\t\"math\"\n)\n\n")
	for _, decl := range parsed.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
			if err := printer.Fprint(&src, fset, fd); err != nil {
				t.Fatal(err)
			}
		}
	}
	file, err := interp.Compile(interpcase.Table(), interpcase.Path, name+".go", []byte(src.String()))
	if err != nil {
		t.Logf("the interpreter refuses the function, so the dev server rebuilds: %v", err)
		return
	}
	fn := file.Func(name)
	if fn == nil {
		t.Fatalf("the file has no function %s", name)
	}
	want := interpcase.Samples[name]()
	got := func() (out string) {
		defer func() {
			if r := recover(); r != nil {
				out = fmt.Sprint("panic: ", r)
			}
		}()
		return fn.Call(nil)[0].String()
	}()
	if got != want {
		t.Fatalf("%s: interpreted %q, compiled %q", name, got, want)
	}
}

// A conversion of a constant gives a typed constant. The interpreter keeps
// the whole-number form of the value and divides as integers
// (internal/interp/ops.go binary, token.QUO): float64(1) / 2 is 0.
func TestREQ_DEV_05_DivisionOfAFloatConstant(t *testing.T) {
	differ(t, "ConstDivision")
}

// The order comparisons of the interpreter come from one three-way
// compare (ops.go compare). NaN is not less and not greater, so <= and >=
// are true. A mean of no rows then passes "mean >= 0".
func TestREQ_DEV_05_OrderComparisonWithNaN(t *testing.T) {
	differ(t, "NaNOrder")
}

// min and max of the interpreter (ops.go pick) return the first value when
// the other value is NaN.
func TestREQ_DEV_05_MinMaxWithNaN(t *testing.T) {
	differ(t, "NaNMinMax")
}

// The interpreter rewrites x op= y and x++ to x = x op y (stmt.go assign)
// and evaluates the operands of x two times: a call in an index runs two
// times, and the read and the write use different elements.
func TestREQ_DEV_05_OpAssignEvaluatesItsOperandOnce(t *testing.T) {
	differ(t, "OpAssignOnce")
}

// The result of constant arithmetic drops the rune mark (ops.go binary),
// and the generated symbol table writes an untyped rune constant as an
// integer (internal/compiler/symbols.go constantExpr; interp.Rune exists
// and nothing calls it). The value then has the type int, not rune.
func TestREQ_DEV_05_DefaultTypeOfARuneConstant(t *testing.T) {
	differ(t, "RuneConst")
}
