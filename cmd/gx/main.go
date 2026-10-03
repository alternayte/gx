// Command gx is the Gx command line tool.
package main

import (
	"os"

	"github.com/alternayte/gx/gxcli"
)

func main() {
	os.Exit(gxcli.Main(os.Args[1:]))
}
