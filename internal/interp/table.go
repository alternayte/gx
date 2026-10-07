// Package interp is the dev interpreter (REQ-DEV-02, REQ-DEV-05). It runs
// the Go code that the compiler generates for a .gx file, with reflection,
// against a table of the symbols that the running app was built with. The
// compiled path and the interpreted path then run the same statements and
// call the same functions, so the two render the same bytes.
//
// The interpreter covers a measured part of Go (DR-03): what the code
// generator writes, and the common expressions of a template. Compile
// refuses a file that uses more, and the dev server then rebuilds the app.
//
// Only a build with the gxdev tag links this package (SI-08).
package interp

import (
	"fmt"
	"go/constant"
	"go/token"
	"reflect"
	"strings"
)

// Package is the symbols of one Go package, as the generated symbol table
// of an app lists them (REQ-DEV-04).
type Package struct {
	// Path is the import path and Name is the package name.
	Path string
	Name string
	// Values holds the functions, the variables and the typed constants.
	// A variable is an addressable value, so the interpreter reads its
	// current value and can assign to it.
	Values map[string]reflect.Value
	// Consts holds the untyped constants.
	Consts map[string]constant.Value
	// Types holds the named types.
	Types map[string]reflect.Type
	// Methods holds a function for each method that reflection cannot
	// call: an unexported method. The key is "Type.method", or
	// "*Type.method" for a pointer receiver. The function takes the
	// receiver as its first argument.
	Methods map[string]reflect.Value
}

// Table is the symbol table of an app: its packages by import path.
type Table struct {
	pkgs map[string]*Package
}

// NewTable returns a table of the packages.
func NewTable(pkgs ...*Package) *Table {
	t := &Table{pkgs: map[string]*Package{}}
	for _, p := range pkgs {
		t.pkgs[p.Path] = p
	}
	return t
}

// Package returns the package with the import path, or nil.
func (t *Table) Package(path string) *Package {
	if t == nil {
		return nil
	}
	return t.pkgs[path]
}

// method returns the table function for an unexported method of a named
// type, and whether the function takes a pointer receiver.
func (t *Table) method(typ reflect.Type, name string) (fn reflect.Value, pointer, ok bool) {
	base := typ
	if base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	if base.Name() == "" {
		return reflect.Value{}, false, false
	}
	pkg := t.Package(base.PkgPath())
	if pkg == nil {
		return reflect.Value{}, false, false
	}
	typeName := base.Name()
	// The name of an instantiated generic type holds its arguments.
	for i := 0; i < len(typeName); i++ {
		if typeName[i] == '[' {
			typeName = typeName[:i]
			break
		}
	}
	if fn, ok := pkg.Methods[typeName+"."+name]; ok {
		return fn, false, true
	}
	if fn, ok := pkg.Methods["*"+typeName+"."+name]; ok {
		return fn, true, true
	}
	return reflect.Value{}, false, false
}

// Int, Float, String and Bool make the untyped constants of a generated
// symbol table from their Go text.
func Int(lit string) constant.Value  { return constant.MakeFromLiteral(lit, token.INT, 0) }
func String(s string) constant.Value { return constant.MakeString(s) }
func Bool(b bool) constant.Value     { return constant.MakeBool(b) }

// Float makes a float constant from a literal or from an exact fraction
// "a/b". A constant such as 1.0 / 3 has no exact decimal form, and Go keeps
// it exact until a use gives it a type.
func Float(lit string) constant.Value {
	if num, den, ok := strings.Cut(lit, "/"); ok {
		n := constant.MakeFromLiteral(num, token.INT, 0)
		d := constant.MakeFromLiteral(den, token.INT, 0)
		return constant.BinaryOp(constant.ToFloat(n), token.QUO, constant.ToFloat(d))
	}
	return constant.ToFloat(constant.MakeFromLiteral(lit, token.FLOAT, 0))
}

// Rune makes an untyped rune constant from its literal, for example "'/'".
// Its default type is rune, not int.
func Rune(lit string) constant.Value {
	return RuneConst{constant.MakeFromLiteral(lit, token.CHAR, 0)}
}

// RuneConst marks a constant of a table as an untyped rune constant.
type RuneConst struct{ constant.Value }

// Unsupported is the error of Compile for code outside the part of Go that
// the interpreter covers, or for a name that the table does not hold. The
// dev server takes the rebuild path for it.
type Unsupported struct {
	Pos token.Position
	Msg string
}

func (e *Unsupported) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
	}
	return e.Msg
}
