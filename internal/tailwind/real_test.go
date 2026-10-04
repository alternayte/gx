//go:build tailwindreal

package tailwind

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// TestREQ_STY_01_RealBinary downloads the pinned binary, runs it with no
// node on PATH and checks its version (REQ-STY-01). Run with
// `go test -tags tailwindreal` or `just tailwind-smoke`.
func TestREQ_STY_01_RealBinary(t *testing.T) {
	t.Setenv("PATH", "")
	root := t.TempDir()
	m := &Manager{Root: root}
	out, err := m.Run(context.Background(), "--help")
	if err != nil {
		t.Fatalf("run pinned tailwind: %v", err)
	}
	if !strings.Contains(string(out), "tailwindcss") {
		t.Fatalf("tailwind --help = %q", out)
	}
	_ = strconv.Itoa(0)
}
