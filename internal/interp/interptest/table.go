package interptest

import (
	"fmt"
	"go/constant"
	"reflect"
	"strconv"
	"strings"

	"github.com/alternayte/gx/internal/interp"
)

// Table is the symbol table of this package, in the form that the compiler
// generates for an app: every package-level name, a function for each
// unexported method, and the exported names of the imports that the
// samples use.
func Table() *interp.Table {
	own := &interp.Package{
		Path: "github.com/alternayte/gx/internal/interp/interptest",
		Name: "interptest",
		Values: map[string]reflect.Value{
			"Arithmetic": reflect.ValueOf(Arithmetic), "Strings": reflect.ValueOf(Strings),
			"Control": reflect.ValueOf(Control), "Ranges": reflect.ValueOf(Ranges),
			"Literals": reflect.ValueOf(Literals), "Methods": reflect.ValueOf(Methods),
			"Closures": reflect.ValueOf(Closures), "Calls": reflect.ValueOf(Calls),
			"Variables": reflect.ValueOf(Variables), "Conversions": reflect.ValueOf(Conversions),
			"Assignments": reflect.ValueOf(Assignments),
			"Places":      reflect.ValueOf(Places),
			"Makes":       reflect.ValueOf(Makes),
			"join":        reflect.ValueOf(join), "pair": reflect.ValueOf(pair), "apply": reflect.ValueOf(apply),
			"try":     reflect.ValueOf(try),
			"items":   reflect.ValueOf(items),
			"Counter": reflect.ValueOf(&Counter).Elem(),
			"Small":   reflect.ValueOf(Small), "Large": reflect.ValueOf(Large), "Scale": reflect.ValueOf(Scale),
		},
		Consts: map[string]constant.Value{"Limit": interp.Int("3")},
		Types: map[string]reflect.Type{
			"Item": reflect.TypeFor[Item](), "Kind": reflect.TypeFor[Kind](), "Render": reflect.TypeFor[Render](),
			"Builder": reflect.TypeFor[Builder](), "Shape": reflect.TypeFor[Shape](), "Square": reflect.TypeFor[Square](),
			"Wrapped": reflect.TypeFor[Wrapped](),
		},
		Methods: map[string]reflect.Value{
			"Item.secret": reflect.ValueOf(Item.secret),
			"*Item.bump":  reflect.ValueOf((*Item).bump),
		},
	}
	return interp.NewTable(own,
		&interp.Package{Path: "fmt", Name: "fmt", Values: map[string]reflect.Value{"Sprint": reflect.ValueOf(fmt.Sprint)}},
		&interp.Package{Path: "strconv", Name: "strconv", Values: map[string]reflect.Value{
			"Itoa": reflect.ValueOf(strconv.Itoa), "Quote": reflect.ValueOf(strconv.Quote)}},
		&interp.Package{Path: "strings", Name: "strings", Values: map[string]reflect.Value{
			"Join": reflect.ValueOf(strings.Join), "ToUpper": reflect.ValueOf(strings.ToUpper), "Repeat": reflect.ValueOf(strings.Repeat)}},
	)
}

// try runs f and returns "panic" when f panics. It is in this file because
// the interpreter has no defer: the samples call it as compiled code.
func try(f func()) (out string) {
	defer func() {
		if recover() != nil {
			out = "panic"
		}
	}()
	f()
	return "ok"
}
