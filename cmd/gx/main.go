// Command gx is the Gx command line tool.
package main

import (
	"os"

	"github.com/alternayte/gx/gxcli"
)

func main() {
	// A project with its own cmd/gx has its plugins and its Gx version in
	// that command (REQ-PLG-02).
	if code, ok := gxcli.Delegate(os.Args[1:]); ok {
		os.Exit(code)
	}
	os.Exit(gxcli.Main(os.Args[1:]))
}
