package gx

import (
	_ "embed"
)

// coreRuntimeJS is the built Gx browser runtime. `just runtime` rebuilds it
// from runtime/js/gx.ts; `checks/runtime.sh` fails when it is stale.
//
//go:embed runtime/js/gx.js
var coreRuntimeJS []byte

// coreRuntime returns the script tag of the Gx browser runtime.
func coreRuntime() Node {
	return El("script", Attrs{
		{Key: "type", Value: "module"},
		{Key: "src", Value: BasePath() + "/_gx/gx.js", Kind: AttrURL},
	})
}
