// Package interpcase_test holds the blocking findings of review round 3 on
// the dev interpreter (REQ-DEV-05, P7). Every test fails on the reviewed
// HEAD 71e3564.
//
// REQ-DEV-05 requires that interpreted and compiled code render the same
// bytes. The interpreter accepts each function of sample.go (Compile gives
// no error, so the dev server swaps the code and does not rebuild), and
// then gives a value that differs from the value of the compiled function.
// The cause is one: the value of a variable is the variable itself and not
// a copy, so a later write changes a value that Go has read already.
package interpcase_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/interp"
	"github.com/alternayte/gx/tests/review/round-3/interpcase"
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
	src.WriteString("package interpcase\n\nimport \"fmt\"\n\n")
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
	got := fn.Call(nil)[0].String()
	if got != want {
		t.Fatalf("%s: interpreted %q, compiled %q", name, got, want)
	}
}

// A tuple assignment reads each value of the right side before it writes:
// a, b = b, a exchanges the two.
func TestREQ_DEV_05_TupleAssignmentReadsBeforeItWrites(t *testing.T) {
	differ(t, "Swap")
}

// The index of the left side is read before a value is written:
// i, rows[i] = 2, "changed" writes rows[0].
func TestREQ_DEV_05_AssignmentReadsTheIndexFirst(t *testing.T) {
	differ(t, "IndexBeforeWrite")
}

// A range reads its operand one time. A write to the array, or a new slice
// in the variable, does not change the rows of the loop.
func TestREQ_DEV_05_RangeReadsItsOperandOnce(t *testing.T) {
	differ(t, "RangeOnce")
}

// A method value of a value receiver holds a copy of the receiver.
func TestREQ_DEV_05_MethodValueCopiesItsReceiver(t *testing.T) {
	differ(t, "MethodValue")
}
