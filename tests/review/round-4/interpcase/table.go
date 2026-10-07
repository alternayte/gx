package interpcase

import (
	"fmt"
	"go/constant"
	"math"
	"reflect"

	"github.com/alternayte/gx/internal/interp"
)

// Path is the import path of the package.
const Path = "github.com/alternayte/gx/tests/review/round-4/interpcase"

// Samples holds the compiled functions by name.
var Samples = map[string]func() string{
	"ConstDivision": ConstDivision, "NaNOrder": NaNOrder, "NaNMinMax": NaNMinMax,
	"OpAssignOnce": OpAssignOnce, "RuneConst": RuneConst,
}

// Table is the symbol table of the package, in the form that the compiler
// generates for an app. The entry of Sep is the text that
// internal/compiler/symbols.go writes for const Sep = '/'. At the time of
// the review that was gx.DevInt("47"), which was the defect. The generator
// now writes gx.DevRune("'/'"), and
// TestREQ_DEV_04_SymbolTableKeepsExactConstants pins that text (D-249).
func Table() *interp.Table {
	own := &interp.Package{
		Path: Path, Name: "interpcase",
		Values: map[string]reflect.Value{
			"next":  reflect.ValueOf(next),
			"calls": reflect.ValueOf(&calls).Elem(),
		},
		Consts: map[string]constant.Value{"Sep": interp.Rune("'/'")},
	}
	for name, fn := range Samples {
		own.Values[name] = reflect.ValueOf(fn)
	}
	return interp.NewTable(own,
		&interp.Package{Path: "fmt", Name: "fmt", Values: map[string]reflect.Value{"Sprint": reflect.ValueOf(fmt.Sprint)}},
		&interp.Package{Path: "math", Name: "math", Values: map[string]reflect.Value{"NaN": reflect.ValueOf(math.NaN)}},
	)
}
