// Package execname names a built binary for the host platform.
package execname

import "runtime"

// Name appends ".exe" on Windows, so a binary built with a bare name runs.
func Name(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}
