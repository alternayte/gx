package interpcase

import (
	"fmt"
	"reflect"

	"github.com/alternayte/gx/internal/interp"
)

// Path is the import path of the package.
const Path = "github.com/alternayte/gx/tests/review/round-3/interpcase"

// Samples holds the compiled functions by name.
var Samples = map[string]func() string{
	"Swap": Swap, "IndexBeforeWrite": IndexBeforeWrite, "RangeOnce": RangeOnce, "MethodValue": MethodValue,
}

// Table is the symbol table of the package, in the form that the compiler
// generates for an app.
func Table() *interp.Table {
	own := &interp.Package{
		Path: Path, Name: "interpcase",
		Values: map[string]reflect.Value{},
		Types:  map[string]reflect.Type{"Point": reflect.TypeFor[Point]()},
	}
	for name, fn := range Samples {
		own.Values[name] = reflect.ValueOf(fn)
	}
	return interp.NewTable(own,
		&interp.Package{Path: "fmt", Name: "fmt", Values: map[string]reflect.Value{"Sprint": reflect.ValueOf(fmt.Sprint)}},
	)
}
