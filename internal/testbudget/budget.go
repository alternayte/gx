// Package testbudget scales the NFR time budgets to the machine that runs
// the test. SDD §13.1 measures the NFR timings on an Apple Silicon laptop,
// so the reference budget applies to a developer machine and a wider
// ceiling applies under CI. The wider ceiling still catches a gross
// regression while tolerating a shared runner.
package testbudget

import (
	"os"
	"runtime"
	"time"
)

// Reference reports whether this machine is the NFR reference machine and
// not CI.
func Reference() bool {
	return os.Getenv("CI") == "" && runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

// Budget returns reference on the reference machine, or reference scaled
// by factor elsewhere.
func Budget(reference time.Duration, factor int) time.Duration {
	if Reference() {
		return reference
	}
	return time.Duration(factor) * reference
}
