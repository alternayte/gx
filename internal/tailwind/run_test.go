//go:build !windows

package tailwind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestREQ_STY_01_Run covers running the pinned binary (REQ-STY-01).
func TestREQ_STY_01_Run(t *testing.T) {
	content := []byte("#!/bin/sh\necho ran-$1\n")
	sum := sha256.Sum256(content)
	root, cache := t.TempDir(), t.TempDir()
	writeLock(t, root, "v1.2.3", hex.EncodeToString(sum[:]))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	m := &Manager{Root: root, Cache: cache, BaseURL: srv.URL}
	out, err := m.Run(context.Background(), "ok")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(string(out), "ran-ok") {
		t.Fatalf("Run output = %q", out)
	}
}
