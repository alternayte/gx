package pagefind_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

	"github.com/alternayte/gx/internal/pagefind"
)

// fakeTarball builds a release tarball that holds one executable script.
func fakeTarball(t *testing.T) []byte {
	t.Helper()
	name := "pagefind"
	if runtime.GOOS == "windows" {
		name = "pagefind.exe"
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho pagefind $@\n")
	if runtime.GOOS == "windows" {
		body = []byte("@echo off\r\necho pagefind %*\r\n")
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestREQ_CNT_07_Asset covers the platform asset names (REQ-CNT-07).
func TestREQ_CNT_07_Asset(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
		ok                 bool
	}{
		{"darwin", "arm64", "pagefind-v1.5.2-aarch64-apple-darwin.tar.gz", true},
		{"darwin", "amd64", "pagefind-v1.5.2-x86_64-apple-darwin.tar.gz", true},
		{"linux", "amd64", "pagefind-v1.5.2-x86_64-unknown-linux-musl.tar.gz", true},
		{"linux", "arm64", "pagefind-v1.5.2-aarch64-unknown-linux-musl.tar.gz", true},
		{"windows", "amd64", "pagefind-v1.5.2-x86_64-pc-windows-msvc.tar.gz", true},
		{"plan9", "mips", "", false},
	}
	for _, c := range cases {
		got, err := pagefind.Asset(c.goos, c.goarch)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("Asset(%s, %s) = %q, %v; want %q", c.goos, c.goarch, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("Asset(%s, %s) = %q, want an error", c.goos, c.goarch, got)
		}
	}
}

// TestREQ_CNT_07_PinnedDownload covers the pinned download: the tarball is
// verified against gx.lock, the binary is unpacked and it runs (REQ-CNT-07).
func TestREQ_CNT_07_PinnedDownload(t *testing.T) {
	data := fakeTarball(t)
	sum := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer server.Close()

	asset, err := pagefind.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("no pagefind asset for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	root := t.TempDir()
	lock := map[string]any{
		"tailwind": map[string]any{"version": "v4.3.3"},
		"pagefind": map[string]any{
			"version": "v1.5.2",
			"sha256":  map[string]string{asset: hex.EncodeToString(sum[:])},
		},
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gx.lock"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m := &pagefind.Manager{Root: root, Cache: filepath.Join(root, "cache"), BaseURL: server.URL}
	path, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("unpacked binary is empty")
	}
	if runtime.GOOS != "windows" {
		out, err := m.Run(context.Background(), "--version")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !strings.Contains(string(out), "pagefind") {
			t.Fatalf("Run output = %q", out)
		}
	}

	// A tampered tarball is rejected.
	bad := &pagefind.Manager{Root: root, Cache: filepath.Join(root, "cache2"), BaseURL: server.URL}
	lock["pagefind"] = map[string]any{"version": "v1.5.2", "sha256": map[string]string{asset: strings.Repeat("0", 64)}}
	raw, _ = json.Marshal(lock)
	if err := os.WriteFile(filepath.Join(root, "gx.lock"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := bad.Ensure(context.Background()); err == nil {
		t.Fatal("tampered download was accepted")
	}
}

// TestREQ_CNT_07_LockMerge covers the gx.lock merge: the pagefind section
// lands next to the other sections (REQ-CNT-07).
func TestREQ_CNT_07_LockMerge(t *testing.T) {
	root := t.TempDir()
	raw := []byte(`{"tailwind":{"version":"v4.3.3"}}`)
	if err := os.WriteFile(filepath.Join(root, "gx.lock"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := pagefind.SaveLock(root, pagefind.LockEntry{Version: "v1.5.2", SHA256: map[string]string{"a": "b"}}); err != nil {
		t.Fatal(err)
	}
	lock, err := pagefind.LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Pagefind.Version != "v1.5.2" || lock.Pagefind.SHA256["a"] != "b" {
		t.Fatalf("lock = %+v", lock)
	}
	var all map[string]json.RawMessage
	data, _ := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatal(err)
	}
	if _, ok := all["tailwind"]; !ok {
		t.Fatalf("merge dropped the tailwind section: %s", data)
	}
}
