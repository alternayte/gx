//go:build windows

package gxcli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// runProjectCLI runs the command of the project and waits for it. Windows
// cannot put one program in the place of a different one.
func runProjectCLI(bin string, args, env []string) int {
	cmd := exec.Command(bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "gx: the command of the project did not start: %v\n", err)
		return 1
	}
	return 0
}
