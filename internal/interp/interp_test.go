package interp_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alternayte/gx/internal/interp"
	"github.com/alternayte/gx/internal/interp/interptest"
)

const samplePath = "github.com/alternayte/gx/internal/interp/interptest"

func compileSamples(t *testing.T) *interp.File {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("interptest", "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := interp.Compile(interptest.Table(), samplePath, "sample.go", src)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// Each sample gives the same value as compiled code and as interpreted
// code: the same source runs both ways (REQ-DEV-05).
func TestREQ_DEV_05_InterpretedGoMatchesCompiledGo(t *testing.T) {
	file := compileSamples(t)
	samples := map[string]func() string{
		"Arithmetic": interptest.Arithmetic, "Strings": interptest.Strings, "Control": interptest.Control,
		"Ranges": interptest.Ranges, "Literals": interptest.Literals, "Methods": interptest.Methods,
		"Closures": interptest.Closures, "Calls": interptest.Calls, "Variables": interptest.Variables,
		"Conversions": interptest.Conversions, "Assignments": interptest.Assignments,
		"Places": interptest.Places,
		"Makes":  interptest.Makes,
	}
	for name, compiled := range samples {
		t.Run(name, func(t *testing.T) {
			fn := file.Func(name)
			if fn == nil {
				t.Fatalf("the file has no function %s", name)
			}
			want := compiled()
			got := fn.Call(nil)[0].String()
			if got != want {
				t.Fatalf("interpreted\n%s\ncompiled\n%s", got, want)
			}
			if len(want) < 8 {
				t.Fatalf("the sample gives almost nothing: %q", want)
			}
		})
	}
}

// A function with arguments and two results: the arguments have the types
// of the compiled function.
func TestREQ_DEV_05_ArgumentsAndResults(t *testing.T) {
	src := `package interptest

func pair(n int) (int, string) {
	if n < 0 {
		return 0, "negative"
	}
	return n * 3, "x"
}

func apply(r Render, it Item) string {
	it.Raise(1)
	return r(it) + "!"
}
`
	file, err := interp.Compile(interptest.Table(), samplePath, "edit.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out := file.Func("pair").Call([]reflect.Value{reflect.ValueOf(4)})
	if out[0].Int() != 12 || out[1].String() != "x" {
		t.Fatalf("pair(4) = %v, %v", out[0], out[1])
	}
	render := interptest.Render(func(it interptest.Item) string { return it.Label() })
	got := file.Func("apply").Call([]reflect.Value{reflect.ValueOf(render), reflect.ValueOf(interptest.Item{Name: "a", Price: 1})})
	if got[0].String() != "a:2!" {
		t.Fatalf("apply = %s", got[0])
	}
}

// Code outside the covered part of Go, or a name that the table does not
// hold, is an Unsupported error with a position. The dev server then takes
// the rebuild path (REQ-DEV-04).
func TestREQ_DEV_04_UnsupportedCodeIsRefused(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"unknown function": {`return strings.TrimSpace(" x ")`, "the symbol table has no name strings.TrimSpace"},
		"unknown package":  {`return os.Getenv("X")`, "the symbol table has no name os"},
		"unknown local":    {`return helper()`, "the symbol table has no name helper"},
		"unknown type":     {`var w Widget; _ = w; return ""`, "the symbol table has no type Widget"},
		"unknown method":   {`return Item{}.hidden()`, "has no field or method hidden"},
		// A generic function has no value for the table, so its name is unknown.
		"generic call": {`return first[string]([]string{"a"})`, "the symbol table has no name first"},
		"goroutine":    {`go join("a"); return ""`, "*ast.GoStmt is not interpreted"},
		"defer":        {`defer join("a"); return ""`, "*ast.DeferStmt is not interpreted"},
		"label":        {"loop:\n\tfor {\n\t\tbreak loop\n\t}\n\treturn \"\"", "not interpreted"},
		"type switch":  {`var x any; switch x.(type) { }; return ""`, "not interpreted"},
		"iterator":     {`for range func(yield func(int) bool) {} { }; return ""`, "is not interpreted"},
		"wrong type":   {`var n int = "text"; _ = n; return ""`, "is not a value of type int"},
		"bad argument": {`return strconv.Itoa("1")`, "is not a value of type int"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := "package interptest\n\nimport (\n\t\"strconv\"\n\t\"strings\"\n)\n\nvar _ = strconv.Itoa\nvar _ = strings.Join\n\nfunc Strings() string {\n\t" + tc.body + "\n}\n"
			_, err := interp.Compile(interptest.Table(), samplePath, "edit.go", []byte(src))
			var u *interp.Unsupported
			if !errors.As(err, &u) {
				t.Fatalf("err = %v, want an *interp.Unsupported", err)
			}
			if !strings.Contains(u.Msg, tc.want) {
				t.Fatalf("message = %q, want %q", u.Msg, tc.want)
			}
			if u.Pos.Line < 11 {
				t.Fatalf("position = %s, want a line of the function body", u.Pos)
			}
		})
	}
	// An import that the table does not hold is refused too.
	_, err := interp.Compile(interptest.Table(), samplePath, "edit.go", []byte("package interptest\n\nimport \"os\"\n\nfunc Strings() string { return os.Getenv(\"X\") }\n"))
	if err == nil || !strings.Contains(err.Error(), "the symbol table has no package os") {
		t.Fatalf("err = %v", err)
	}
	_, err = interp.Compile(interptest.Table(), "example.com/other", "edit.go", []byte("package other\n"))
	if err == nil || !strings.Contains(err.Error(), "the symbol table has no package example.com/other") {
		t.Fatalf("err = %v", err)
	}
}

// A run-time panic of interpreted code is a panic, as it is in compiled
// code: the dev overlay shows it.
func TestREQ_DEV_05_PanicsPropagate(t *testing.T) {
	src := "package interptest\n\nfunc Strings() string {\n\tvar list []string\n\treturn list[2]\n}\n"
	file, err := interp.Compile(interptest.Table(), samplePath, "edit.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("an index out of range did not panic")
		}
	}()
	file.Func("Strings").Call(nil)
}
