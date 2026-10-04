package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Lint checks every source item of a registry against REQ-REG-06: each item
// has a USAGE.md with usage examples, a do and don't section and a keyboard
// spec table, and every component has a fixtures file. It returns one
// finding per violation.
func Lint(src string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(src, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "gx-item.json")); err != nil {
			continue
		}
		findings, err := lintItem(dir)
		if err != nil {
			return out, err
		}
		out = append(out, findings...)
	}
	return out, nil
}

// lintItem checks one item folder.
func lintItem(dir string) ([]string, error) {
	name := filepath.Base(dir)
	var out []string
	fail := func(format string, args ...any) {
		out = append(out, name+": "+fmt.Sprintf(format, args...))
	}
	raw, err := os.ReadFile(filepath.Join(dir, "gx-item.json"))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("registry: %s: %w", name, err)
	}
	if manifest.Name != name {
		fail("manifest name %s does not match the folder", manifest.Name)
	}
	usage, err := os.ReadFile(filepath.Join(dir, "USAGE.md"))
	if err != nil {
		fail("USAGE.md is missing")
	} else {
		out = append(out, lintUsage(name, string(usage))...)
	}
	fixtures := map[string]bool{}
	for _, target := range manifest.Files {
		if strings.HasSuffix(target.Path, ".fixtures.go") {
			fixtures[strings.TrimSuffix(target.Path, ".fixtures.go")] = true
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(target.Path))); err != nil {
			fail("file %s is missing", target.Path)
		}
	}
	for _, target := range manifest.Files {
		if !strings.HasSuffix(target.Path, ".gx") {
			continue
		}
		component := strings.TrimSuffix(target.Path, ".gx")
		if !fixtures[component] {
			fail("%s has no %s.fixtures.go", target.Path, component)
		}
	}
	return out, nil
}

// lintUsage checks the required sections of one USAGE.md (REQ-REG-06).
func lintUsage(name, usage string) []string {
	var out []string
	fail := func(format string, args ...any) {
		out = append(out, name+": USAGE.md "+fmt.Sprintf(format, args...))
	}
	sections := map[string]string{}
	current := ""
	for _, line := range strings.Split(usage, "\n") {
		if strings.HasPrefix(line, "## ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			sections[current] = ""
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	usageBody, hasUsage := sections["Usage"]
	if !hasUsage {
		usageBody, hasUsage = sections["Use"]
	}
	if !hasUsage {
		fail("has no Usage section")
	} else if !strings.Contains(usageBody, "```") {
		fail("has no Usage example")
	}
	if _, ok := sections["Do"]; !ok {
		fail("has no Do section")
	}
	if _, ok := sections["Don't"]; !ok {
		fail("has no Don't section")
	}
	if body, ok := sections["Keyboard"]; !ok {
		fail("has no Keyboard section")
	} else if !strings.Contains(body, "| Key |") {
		fail("has no keyboard spec table")
	}
	return out
}
