package tailwind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeLock(t *testing.T, root, version, sum string) {
	t.Helper()
	lock := Lock{Tailwind: LockEntry{Version: version, SHA256: map[string]string{sumName(): sum}}}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gx.lock"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sumName() string {
	name, err := Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		panic(err)
	}
	return name
}

// TestREQ_STY_01_PinnedDownload covers the pinned download: gx.lock holds
// the version and sha256, the binary is cached, verified and executable, and
// a hash mismatch stops it (REQ-STY-01).
func TestREQ_STY_01_PinnedDownload(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.2.3/"+sumName() {
			http.NotFound(w, r)
			return
		}
		hits++
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	root, cache := t.TempDir(), t.TempDir()
	writeLock(t, root, "v1.2.3", hexSum)
	m := &Manager{Root: root, Cache: cache, BaseURL: srv.URL}
	path, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if hits != 1 {
		t.Fatalf("downloads = %d, want 1", hits)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("binary is not executable: %v", info.Mode())
	}
	// A second Ensure uses the cache and does not download again.
	if _, err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if hits != 1 {
		t.Fatalf("downloads after cache = %d, want 1", hits)
	}

	// A tampered binary is re-downloaded.
	if err := os.WriteFile(path, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure after tamper: %v", err)
	}
	if hits != 2 {
		t.Fatalf("downloads after tamper = %d, want 2", hits)
	}

	// A lock with the wrong hash stops the download.
	badRoot := t.TempDir()
	writeLock(t, badRoot, "v1.2.3", strings.Repeat("0", 64))
	bad := &Manager{Root: badRoot, Cache: t.TempDir(), BaseURL: srv.URL}
	if _, err := bad.Ensure(context.Background()); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("bad hash error = %v", err)
	}
}

// TestREQ_STY_01_DefaultLock covers the built-in pin and the platform asset
// names (REQ-STY-01).
func TestREQ_STY_01_DefaultLock(t *testing.T) {
	root := t.TempDir()
	lock, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Tailwind.Version != DefaultVersion || lock.Tailwind.SHA256["tailwindcss-macos-arm64"] == "" {
		t.Fatalf("default lock = %+v", lock.Tailwind)
	}
	for _, tc := range []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "tailwindcss-macos-arm64"},
		{"darwin", "amd64", "tailwindcss-macos-x64"},
		{"linux", "amd64", "tailwindcss-linux-x64"},
		{"linux", "arm64", "tailwindcss-linux-arm64"},
		{"windows", "amd64", "tailwindcss-windows-x64.exe"},
	} {
		got, err := Asset(tc.goos, tc.goarch)
		if err != nil || got != tc.want {
			t.Fatalf("Asset(%s, %s) = %q, %v", tc.goos, tc.goarch, got, err)
		}
	}
	if _, err := Asset("plan9", "arm"); err == nil {
		t.Fatal("Asset accepted an unsupported platform")
	}
}
