package compiler

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Overlay support: an in-memory view of the input files, so the LSP checks
// unsaved buffers (REQ-DEV-08) without touching disk.

// SetOverlay records in-memory content for one input file. The next
// Generate call reads the overlay instead of the file on disk.
func (s *Session) SetOverlay(path string, src []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overlay == nil {
		s.overlay = map[string][]byte{}
	}
	s.overlay[filepath.Clean(path)] = append([]byte(nil), src...)
}

// ClearOverlay drops the in-memory content of one file.
func (s *Session) ClearOverlay(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overlay, filepath.Clean(path))
}

// readInput returns the content of one input file, from the overlay when it
// holds the path.
func (s *Session) readInput(path string) []byte {
	if src, ok := s.overlay[filepath.Clean(path)]; ok {
		return src
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// snapshotInputsWith hashes every input file of the compiler, using the
// overlay for paths it holds and adding overlay paths that are new.
func snapshotInputsWith(root string, overlay map[string][]byte) (map[string]fileStamp, error) {
	out := map[string]fileStamp{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".gx", "vendor", "testdata", ".gx-build":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, "_gx.go") {
			return nil // generated output
		}
		if !strings.HasSuffix(name, ".gx") && !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if src, ok := overlay[filepath.Clean(path)]; ok {
			data = src
		}
		out[path] = fileStamp{hash: sha256.Sum256(data)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for path, src := range overlay {
		if _, ok := out[path]; ok {
			continue
		}
		if !strings.HasSuffix(path, ".gx") {
			continue
		}
		out[path] = fileStamp{hash: sha256.Sum256(src)}
	}
	return out, nil
}

// inputContent returns the content of one input file for signature
// checks, from the overlay when it holds the path.
func inputContent(path string, overlay map[string][]byte) ([]byte, bool) {
	if src, ok := overlay[filepath.Clean(path)]; ok {
		return src, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// overlayDirs returns the directories the overlay adds to the walk.
func overlayDirs(root string, overlay map[string][]byte) []string {
	seen := map[string]bool{}
	var out []string
	for path := range overlay {
		if !strings.HasSuffix(path, ".gx") {
			continue
		}
		dir := filepath.Dir(path)
		if !strings.HasPrefix(dir, root) || seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	sort.Strings(out)
	return out
}
