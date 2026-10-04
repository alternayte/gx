package tailwind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREQ_STY_12_MirrorAndVendor covers the gx.toml mirror and the vendored
// binary: after gx vendor the pinned binary is used with the network off
// (REQ-STY-12).
func TestREQ_STY_12_MirrorAndVendor(t *testing.T) {
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

	root := t.TempDir()
	writeLock(t, root, "v1.2.3", hexSum)
	if err := os.WriteFile(filepath.Join(root, "gx.toml"), []byte("[mirrors]\ntailwind = \""+srv.URL+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manager{Root: root, Cache: t.TempDir()}
	path, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatalf("mirror Ensure: %v", err)
	}
	if hits != 1 {
		t.Fatalf("mirror hits = %d, want 1", hits)
	}
	_ = path

	vendored, err := m.Vendor(context.Background())
	if err != nil {
		t.Fatalf("Vendor: %v", err)
	}
	if !strings.Contains(vendored, filepath.Join(".gx", "vendor", "tailwind")) {
		t.Fatalf("vendored path = %q", vendored)
	}

	// The network is off: a dead mirror and a cold cache still work.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "offline", http.StatusBadGateway)
	}))
	defer dead.Close()
	if err := os.WriteFile(filepath.Join(root, "gx.toml"), []byte("[mirrors]\ntailwind = \""+dead.URL+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	offline := &Manager{Root: root, Cache: t.TempDir()}
	got, err := offline.Ensure(context.Background())
	if err != nil {
		t.Fatalf("offline Ensure: %v", err)
	}
	if got != vendored {
		t.Fatalf("offline path = %q, want %q", got, vendored)
	}
}

// TestSI_10_TamperedVendor covers the pin check: a tampered vendored binary
// stops the build (SI-10).
func TestSI_10_TamperedVendor(t *testing.T) {
	content := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	root := t.TempDir()
	writeLock(t, root, "v1.2.3", hexSum)
	m := &Manager{Root: root, Cache: t.TempDir(), BaseURL: srv.URL}
	vendored, err := m.Vendor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendored, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Ensure(context.Background()); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("tampered vendor error = %v", err)
	}
}
