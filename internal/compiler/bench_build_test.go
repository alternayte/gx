package compiler_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alternayte/gx/internal/compiler"
)

// repoPath returns the repo root for a benchmark.
func repoPath(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// BenchmarkNFR_05_ColdBuild measures a cold gx build of the example app
// against the 10 s budget (NFR-05).
func BenchmarkNFR_05_ColdBuild(b *testing.B) {
	root := repoPath(b)
	bin := filepath.Join(b.TempDir(), "shop")
	for i := 0; i < b.N; i++ {
		start := time.Now()
		cmd := exec.Command("go", "run", "./cmd/gx", "build", "-o", bin, filepath.Join(root, "examples", "shop"))
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("gx build: %v\n%s", err, out)
		}
		took := time.Since(start)
		b.ReportMetric(took.Seconds(), "s/cold-build")
		if took > 10*time.Second {
			b.Fatalf("NFR-05 cold gx build %s, want under 10s", took)
		}
	}
}

// BenchmarkNFR_05_IncrementalCompile measures the compile of one .gx file
// against the 50 ms budget (NFR-05).
func BenchmarkNFR_05_IncrementalCompile(b *testing.B) {
	dir := filepath.Join(repoPath(b), "examples", "shop")
	view := filepath.Join(dir, "Home.gx")
	original, err := os.ReadFile(view)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = os.WriteFile(view, original, 0o644) })
	session := compiler.NewSession()
	if _, diags := session.Generate(dir); len(diags) > 0 {
		b.Fatalf("warm up: %v", diags)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		changed := append(append([]byte{}, original...), bytes.Repeat([]byte(" "), i+1)...)
		if err := os.WriteFile(view, changed, 0o644); err != nil {
			b.Fatal(err)
		}
		start := time.Now()
		if _, diags := session.Generate(dir); len(diags) > 0 {
			b.Fatalf("generate: %v", diags)
		}
		took := time.Since(start)
		b.ReportMetric(float64(took.Microseconds())/1000, "ms/compile")
		if took > 50*time.Millisecond {
			b.Fatalf("NFR-05 incremental .gx compile %s, want under 50ms", took)
		}
	}
}
