//go:build gxdev

package gx

import (
	"errors"
	"go/constant"
	"sync"

	"github.com/alternayte/gx/internal/interp"
)

// Dev is true in a build with the gxdev tag. Generated code asks it before
// it calls DevRender; in a production build the constant is false and the
// compiler removes that call.
const Dev = true

// DevPackage is the symbols of one package for the dev interpreter
// (REQ-DEV-04). The generated symbol table of an app makes the values.
type DevPackage = interp.Package

// DevInt, DevFloat, DevString and DevBool make the untyped constants of a
// generated symbol table.
func DevInt(lit string) constant.Value   { return interp.Int(lit) }
func DevFloat(lit string) constant.Value { return interp.Float(lit) }
func DevString(s string) constant.Value  { return interp.String(s) }
func DevBool(b bool) constant.Value      { return interp.Bool(b) }

// DevRune makes an untyped rune constant from its literal.
func DevRune(lit string) constant.Value { return interp.Rune(lit) }

// devState holds the symbol table of the app and the functions that a swap
// replaced (REQ-DEV-02).
var devState struct {
	mu    sync.RWMutex
	table *interp.Table
	// pkgs holds the packages of the app by path. The fuzzer finds a
	// component function here (REQ-AI-11).
	pkgs map[string]*interp.Package
	// funcs holds the interpreted functions by package path and name.
	funcs map[string]map[string]*interp.Func
	// files holds, for each swapped file, the names of its functions.
	files map[string][]string
}

// SetDevSymbols installs the symbol table of the app. The dev main of an
// app calls it with the packages of its generated gxdev_symbols package.
func SetDevSymbols(pkgs []DevPackage) {
	list := make([]*interp.Package, len(pkgs))
	byPath := map[string]*interp.Package{}
	for i := range pkgs {
		p := pkgs[i]
		if p.Path == "github.com/alternayte/gx" {
			// Interpreted code is the swapped function already: its own
			// check of gx.Dev must not ask for a swap again.
			consts := map[string]constant.Value{}
			for name, v := range p.Consts {
				consts[name] = v
			}
			consts["Dev"] = constant.MakeBool(false)
			p.Consts = consts
		}
		list[i] = &p
		byPath[p.Path] = &p
	}
	devState.mu.Lock()
	defer devState.mu.Unlock()
	devState.table = interp.NewTable(list...)
	devState.pkgs = byPath
	devState.funcs = map[string]map[string]*interp.Func{}
	devState.files = map[string][]string{}
}

// DevSwap replaces the functions of one generated file with interpreted
// code (REQ-DEV-02). It changes nothing and returns an error when the
// interpreter does not cover the code or the table does not hold a name of
// it; the dev server then rebuilds the app.
func DevSwap(pkgPath, file string, src []byte) error {
	devState.mu.Lock()
	defer devState.mu.Unlock()
	if devState.table == nil {
		return errors.New("gx: the app has no dev symbol table; call gx.SetDevSymbols(gxdev_symbols.Packages()) in the dev main")
	}
	compiled, err := interp.Compile(devState.table, pkgPath, file, src)
	if err != nil {
		return err
	}
	key := pkgPath + "\x00" + file
	funcs := devState.funcs[pkgPath]
	if funcs == nil {
		funcs = map[string]*interp.Func{}
		devState.funcs[pkgPath] = funcs
	}
	for _, name := range devState.files[key] {
		delete(funcs, name)
	}
	names := compiled.Names()
	for _, name := range names {
		funcs[name] = compiled.Func(name)
	}
	devState.files[key] = names
	return nil
}

// DevSwapped returns the count of functions that run as interpreted code.
func DevSwapped() int {
	devState.mu.RLock()
	defer devState.mu.RUnlock()
	n := 0
	for _, funcs := range devState.funcs {
		n += len(funcs)
	}
	return n
}

// DevRender runs the interpreted form of a generated function, when a swap
// installed one. A generated component or fragment function calls it first
// in a dev build. The call also keeps the props of a component for the
// capture of a fixture (REQ-AI-12).
func DevRender(pkgPath, name string, args ...any) (Node, bool) {
	devKeepProps(pkgPath, name, args)
	devState.mu.RLock()
	fn := devState.funcs[pkgPath][name]
	devState.mu.RUnlock()
	if fn == nil {
		return nil, false
	}
	node, _ := fn.CallAny(args...).(Node)
	return node, true
}
