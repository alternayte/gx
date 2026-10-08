//go:build !windows

package gxcli

import (
	"fmt"
	"os"
	"syscall"
)

// runProjectCLI puts the command of the project in the place of this
// process. A signal or a kill for the gx of the user then reaches the
// command of the project: no second process stays behind with the terminal
// or the pipes of the first.
func runProjectCLI(bin string, args, env []string) int {
	err := syscall.Exec(bin, append([]string{bin}, args...), env)
	fmt.Fprintf(os.Stderr, "gx: the command of the project did not start: %v\n", err)
	return 1
}
