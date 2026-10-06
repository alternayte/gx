package lsp

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// watch polls the module for input changes and re-checks when one appears
// (REQ-DEV-11). The poll keeps the 2 s rename budget without requiring the
// client to register a file watcher. A client that sends
// workspace/didChangeWatchedFiles also triggers a check.
func (s *Server) watch() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
		}
		s.mu.Lock()
		root := s.root
		s.mu.Unlock()
		if root == "" {
			continue
		}
		// The reference is the state that the last check read. A file that
		// changes between that check and the next poll differs from it, so
		// the poll never takes a changed state as its start.
		fp := fingerprint(root)
		checked, _ := s.checked.Load().(string)
		if checked == "" {
			s.checked.CompareAndSwap(nil, fp)
			continue
		}
		if fp == checked {
			continue
		}
		_ = s.diagnose()
	}
}

// fingerprint hashes the paths and metadata of every compiler input under
// root.
func fingerprint(root string) string {
	h := sha256.New()
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".gx", "vendor", ".gx-build":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".gx") && !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".md") && name != "go.mod" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(h, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}
