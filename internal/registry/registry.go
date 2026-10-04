// Package registry is the Gx component registry format (REQ-REG-01). A
// registry is static HTTP: an index.json and one items/<name>.json per
// item. Build turns a source folder of items into that shape; Validate
// checks it against the JSON Schema.
package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Item is one published registry item (REQ-REG-01).
type Item struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	// Kind is "component", "block" or "theme".
	Kind                 string   `json:"kind"`
	Files                []File   `json:"files"`
	RegistryDependencies []string `json:"registryDependencies,omitempty"`
	IconPacks            []string `json:"iconPacks,omitempty"`
	JSPins               []string `json:"jsPins,omitempty"`
	RequiredTokens       []string `json:"requiredTokens,omitempty"`
	// Usage is the USAGE.md of the item.
	Usage string `json:"usage"`
}

// File is one file of an item.
type File struct {
	// Path is the path inside the item source.
	Path string `json:"path"`
	// Target is the path inside the app, relative to the module root.
	Target string `json:"target"`
	// Content is the file text.
	Content string `json:"content"`
	// SHA256 is the hex digest of Content.
	SHA256 string `json:"sha256"`
}

// Manifest is the source gx-item.json of one item folder (REQ-REG-01).
type Manifest struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Description          string   `json:"description"`
	Kind                 string   `json:"kind"`
	Files                []Target `json:"files"`
	RegistryDependencies []string `json:"registryDependencies,omitempty"`
	IconPacks            []string `json:"iconPacks,omitempty"`
	JSPins               []string `json:"jsPins,omitempty"`
	RequiredTokens       []string `json:"requiredTokens,omitempty"`
}

// Target names one source file and its app path.
type Target struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// Index is the registry index.json (REQ-REG-01).
type Index struct {
	Items []Summary `json:"items"`
}

// Summary is one line of the index.
type Summary struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

// namePattern is one registry item name.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// versionPattern is a semver version, with or without a leading v.
var versionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

// hexPattern is one sha256 digest.
var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// kinds are the item kinds.
var kinds = map[string]bool{"component": true, "block": true, "theme": true}

// Build reads every item folder under src (a folder with gx-item.json and
// USAGE.md) and writes the published registry into out: index.json and
// items/<name>.json (REQ-REG-01).
func Build(src, out string) (Index, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return Index{}, err
	}
	var items []Item
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(src, entry.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "gx-item.json"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Index{}, err
		}
		var manifest Manifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return Index{}, fmt.Errorf("registry: %s: %w", entry.Name(), err)
		}
		item, err := buildItem(dir, manifest)
		if err != nil {
			return Index{}, err
		}
		if err := ValidateItem(item); err != nil {
			return Index{}, fmt.Errorf("registry: %s: %w", entry.Name(), err)
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return Index{}, fmt.Errorf("registry: no items under %s", src)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var index Index
	for _, item := range items {
		index.Items = append(index.Items, Summary{
			Name: item.Name, Version: item.Version, Description: item.Description, Kind: item.Kind,
		})
	}
	if err := os.MkdirAll(filepath.Join(out, "items"), 0o755); err != nil {
		return Index{}, err
	}
	for _, item := range items {
		data, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return Index{}, err
		}
		data = append(data, '\n')
		if err := os.WriteFile(filepath.Join(out, "items", item.Name+".json"), data, 0o644); err != nil {
			return Index{}, err
		}
	}
	if err := copySchema(out); err != nil {
		return Index{}, err
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return Index{}, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(out, "index.json"), data, 0o644); err != nil {
		return Index{}, err
	}
	return index, nil
}

// copySchema embeds the item schema in the published registry.
func copySchema(out string) error {
	data, err := Schema()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(out, "schema"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "schema", "item.schema.json"), data, 0o644)
}

// buildItem reads one item folder into its published form.
func buildItem(dir string, manifest Manifest) (Item, error) {
	item := Item{
		Name:                 manifest.Name,
		Version:              manifest.Version,
		Description:          manifest.Description,
		Kind:                 manifest.Kind,
		RegistryDependencies: manifest.RegistryDependencies,
		IconPacks:            manifest.IconPacks,
		JSPins:               manifest.JSPins,
		RequiredTokens:       manifest.RequiredTokens,
	}
	for _, target := range manifest.Files {
		full := filepath.Join(dir, filepath.FromSlash(target.Path))
		if !strings.HasPrefix(full, dir) {
			return Item{}, fmt.Errorf("registry: %s: file path %s leaves the item", manifest.Name, target.Path)
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return Item{}, fmt.Errorf("registry: %s: %w", manifest.Name, err)
		}
		sum := sha256.Sum256(content)
		item.Files = append(item.Files, File{
			Path:    target.Path,
			Target:  target.Target,
			Content: string(content),
			SHA256:  hex.EncodeToString(sum[:]),
		})
	}
	usage, err := os.ReadFile(filepath.Join(dir, "USAGE.md"))
	if err != nil {
		return Item{}, fmt.Errorf("registry: %s: USAGE.md is required", manifest.Name)
	}
	item.Usage = string(usage)
	return item, nil
}

// ValidateItem checks one item against the format rules (REQ-REG-01).
func ValidateItem(item Item) error {
	if !namePattern.MatchString(item.Name) {
		return fmt.Errorf("item name %q is not a lowercase slug", item.Name)
	}
	if !versionPattern.MatchString(item.Version) {
		return fmt.Errorf("item %s version %q is not semver", item.Name, item.Version)
	}
	if strings.TrimSpace(item.Description) == "" {
		return fmt.Errorf("item %s has no description", item.Name)
	}
	if !kinds[item.Kind] {
		return fmt.Errorf("item %s kind %q is not component, block or theme", item.Name, item.Kind)
	}
	if len(item.Files) == 0 {
		return fmt.Errorf("item %s has no files", item.Name)
	}
	for _, file := range item.Files {
		if file.Path == "" || file.Target == "" {
			return fmt.Errorf("item %s has a file without a path or target", item.Name)
		}
		if filepath.IsAbs(file.Target) || strings.HasPrefix(file.Target, "/") ||
			strings.HasPrefix(file.Target, `\`) || strings.Contains(file.Target, "..") {
			return fmt.Errorf("item %s target %q is not a relative app path", item.Name, file.Target)
		}
		if !hexPattern.MatchString(file.SHA256) {
			return fmt.Errorf("item %s file %s has no sha256", item.Name, file.Path)
		}
		sum := sha256.Sum256([]byte(file.Content))
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("item %s file %s sha256 does not match its content", item.Name, file.Path)
		}
	}
	for _, dep := range item.RegistryDependencies {
		if !namePattern.MatchString(dep) {
			return fmt.Errorf("item %s registry dependency %q is not an item name", item.Name, dep)
		}
	}
	for _, token := range item.RequiredTokens {
		if !strings.HasPrefix(token, "--") {
			return fmt.Errorf("item %s required token %q is not a CSS variable", item.Name, token)
		}
	}
	if item.Kind != "theme" && strings.TrimSpace(item.Usage) == "" {
		return fmt.Errorf("item %s has no USAGE.md", item.Name)
	}
	return nil
}

// ValidateRegistry reads a published registry and checks every item and the
// index (REQ-REG-01).
func ValidateRegistry(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("registry: index.json: %w", err)
	}
	if len(index.Items) == 0 {
		return fmt.Errorf("registry: index.json holds no items")
	}
	seen := map[string]bool{}
	for _, summary := range index.Items {
		if seen[summary.Name] {
			return fmt.Errorf("registry: duplicate item %s in index.json", summary.Name)
		}
		seen[summary.Name] = true
		raw, err := os.ReadFile(filepath.Join(dir, "items", summary.Name+".json"))
		if err != nil {
			return fmt.Errorf("registry: %w", err)
		}
		var item Item
		if err := json.Unmarshal(raw, &item); err != nil {
			return fmt.Errorf("registry: items/%s.json: %w", summary.Name, err)
		}
		if err := ValidateItem(item); err != nil {
			return err
		}
		if item.Name != summary.Name || item.Version != summary.Version {
			return fmt.Errorf("registry: index.json does not match items/%s.json", summary.Name)
		}
	}
	return nil
}

// Find returns one item of a published registry.
func Find(dir, name string) (Item, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "items", name+".json"))
	if err != nil {
		return Item{}, fmt.Errorf("registry: no item %q: %w", name, err)
	}
	var item Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return Item{}, fmt.Errorf("registry: items/%s.json: %w", name, err)
	}
	if err := ValidateItem(item); err != nil {
		return Item{}, err
	}
	return item, nil
}
