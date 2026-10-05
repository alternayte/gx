package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// BaseDir is the committed base snapshot directory (REQ-REG-02).
const BaseDir = ".gx/base"

// DefaultRoot is the published target root that Dir replaces.
const DefaultRoot = "ui"

// RegistryImport is the import prefix of the official registry source. An
// installed item rewrites it to the app module path (REQ-REG-05).
const RegistryImport = "github.com/alternayte/gx/registry/"

// Installer installs published registry items into an app (REQ-REG-02).
type Installer struct {
	// Root is the app module root that holds gx.toml and gx.lock.
	Root string
	// Source is the registry base: an http(s) URL or a directory that
	// holds index.json and items/ (REQ-REG-01).
	Source string
	// Dir is the app directory the published DefaultRoot maps to. Empty
	// keeps DefaultRoot.
	Dir string
	// Headers are "name: value" strings sent with every HTTP request
	// (REQ-REG-04).
	Headers []string
	// Client is the HTTP client, or a client with a 30 s timeout.
	Client *http.Client
}

// Lock is the registry section of gx.lock (REQ-REG-02).
type Lock struct {
	Items map[string]LockItem `json:"items"`
}

// LockItem is one installed item.
type LockItem struct {
	Version string `json:"version"`
	// Files maps the installed app path to the sha256 of its bytes.
	Files map[string]string `json:"files"`
}

// Add fetches one item and its dependency closure, verifies every file hash
// (SI-09), writes the files, stores the base snapshots and records the
// items in gx.lock. It returns the items in install order, dependencies
// first.
func (in *Installer) Add(name, version string) ([]Item, error) {
	if in.Root == "" {
		return nil, errors.New("registry: no app root")
	}
	if in.Source == "" {
		return nil, errors.New("registry: no registry source")
	}
	if !namePattern.MatchString(name) {
		return nil, fmt.Errorf("registry: %q is not an item name", name)
	}
	dir, err := in.appDir()
	if err != nil {
		return nil, err
	}
	var order []Item
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(dep string) error {
		if seen[dep] {
			return nil
		}
		seen[dep] = true
		item, err := in.fetch(dep)
		if err != nil {
			return err
		}
		if dep == name && version != "" && item.Version != version {
			return fmt.Errorf("registry: item %s is version %s, not %s", dep, item.Version, version)
		}
		for _, d := range item.RegistryDependencies {
			if err := visit(d); err != nil {
				return err
			}
		}
		order = append(order, item)
		return nil
	}
	if err := visit(name); err != nil {
		return nil, err
	}
	type planned struct {
		item  Item
		files map[string]string // app path -> installed bytes
	}
	var plans []planned
	for _, item := range order {
		files := map[string]string{}
		for _, f := range item.Files {
			target, err := in.targetPath(dir, f.Target)
			if err != nil {
				return nil, fmt.Errorf("registry: item %s: %w", item.Name, err)
			}
			if _, ok := files[target]; ok {
				return nil, fmt.Errorf("registry: item %s writes %s twice", item.Name, target)
			}
			if err := in.checkTarget(target, f.Content); err != nil {
				return nil, fmt.Errorf("registry: item %s: %w", item.Name, err)
			}
			files[target] = f.Content
		}
		plans = append(plans, planned{item: item, files: files})
	}
	// Earlier installs keep their lock entries.
	lock, err := LoadLock(in.Root)
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		for target, content := range plan.files {
			if err := writeFile(filepath.Join(in.Root, filepath.FromSlash(target)), content); err != nil {
				return nil, err
			}
		}
		if err := in.writeSnapshot(plan.item); err != nil {
			return nil, err
		}
		entry := LockItem{Version: plan.item.Version, Files: map[string]string{}}
		for target, content := range plan.files {
			sum := sha256.Sum256([]byte(content))
			entry.Files[target] = hex.EncodeToString(sum[:])
		}
		lock.Items[plan.item.Name] = entry
	}
	if err := SaveLock(in.Root, lock); err != nil {
		return nil, err
	}
	return order, nil
}

// appDir returns the configured app directory of installed files.
func (in *Installer) appDir() (string, error) {
	dir := in.Dir
	if dir == "" {
		dir = DefaultRoot
	}
	clean := path.Clean(strings.ReplaceAll(dir, `\`, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("registry: install directory %q is not a relative app path", dir)
	}
	return clean, nil
}

// targetPath maps a published target to an app path. A target under the
// published DefaultRoot moves under the configured directory; any other
// target stays as published.
func (in *Installer) targetPath(dir, target string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(target, `\`, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("target %q is not a relative app path", target)
	}
	first, rest, ok := strings.Cut(clean, "/")
	if !ok {
		return path.Join(dir, clean), nil
	}
	if first == DefaultRoot || first == dir {
		return path.Join(dir, rest), nil
	}
	return clean, nil
}

// checkTarget refuses to overwrite a file that differs from the item. The
// first install writes freely; a later version uses gx update (REQ-REG-03).
func (in *Installer) checkTarget(target, content string) error {
	full := filepath.Join(in.Root, filepath.FromSlash(target))
	current, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if string(current) == content {
		return nil
	}
	return fmt.Errorf("%s exists and differs; remove it or use gx update", target)
}

// writeSnapshot stores the published item under .gx/base/<name>@<version>/
// (REQ-REG-02).
func (in *Installer) writeSnapshot(item Item) error {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Join(in.Root, filepath.FromSlash(BaseDir), item.Name+"@"+item.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "item.json"), data, 0o644)
}

// fetch loads one published item and rewrites its cross-item imports to the
// app module path (REQ-REG-05).
func (in *Installer) fetch(name string) (Item, error) {
	data, err := in.read("items/" + name + ".json")
	if err != nil {
		return Item{}, fmt.Errorf("registry: cannot read item %s: %w", name, err)
	}
	var item Item
	if err := json.Unmarshal(data, &item); err != nil {
		return Item{}, fmt.Errorf("registry: item %s: %w", name, err)
	}
	if err := ValidateItem(item); err != nil {
		return Item{}, fmt.Errorf("registry: item %s: %w", name, err)
	}
	if item.Name != name {
		return Item{}, fmt.Errorf("registry: items/%s.json holds item %s", name, item.Name)
	}
	return in.rewriteImports(item)
}

// rewriteImports points the cross-item imports of an item at the app's
// installed copies. Items that import no sibling stay untouched, so an app
// without go.mod can still install them.
func (in *Installer) rewriteImports(item Item) (Item, error) {
	needs := false
	for _, f := range item.Files {
		if strings.Contains(f.Content, RegistryImport) {
			needs = true
			break
		}
	}
	if !needs {
		return item, nil
	}
	module, err := modulePath(in.Root)
	if err != nil {
		return Item{}, err
	}
	dir, err := in.appDir()
	if err != nil {
		return Item{}, err
	}
	prefix := module + "/" + dir + "/"
	out := item
	out.Files = make([]File, len(item.Files))
	for i, f := range item.Files {
		f.Content = strings.ReplaceAll(f.Content, RegistryImport, prefix)
		sum := sha256.Sum256([]byte(f.Content))
		f.SHA256 = hex.EncodeToString(sum[:])
		out.Files[i] = f
	}
	return out, nil
}

// modulePath reads the module path of the app from go.mod.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("registry: the item imports another item, and go.mod cannot be read: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			if path := strings.TrimSpace(rest); path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("registry: go.mod has no module path")
}

// read loads a path from the registry source.
func (in *Installer) read(rel string) ([]byte, error) {
	if strings.HasPrefix(in.Source, "http://") || strings.HasPrefix(in.Source, "https://") {
		url := strings.TrimSuffix(in.Source, "/") + "/" + rel
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		for _, header := range in.Headers {
			name, value, ok := strings.Cut(header, ":")
			if !ok {
				return nil, fmt.Errorf("registry: header %q is not name: value", header)
			}
			req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
		}
		resp, err := in.client().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", url, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
	return os.ReadFile(filepath.Join(in.Source, filepath.FromSlash(rel)))
}

// client returns the HTTP client of the installer.
func (in *Installer) client() *http.Client {
	if in.Client != nil {
		return in.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// writeFile writes one installed file, creating its directory.
func writeFile(full, content string) error {
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

// LoadLock reads the registry section of gx.lock.
func LoadLock(root string) (Lock, error) {
	lock := Lock{Items: map[string]LockItem{}}
	data, err := os.ReadFile(filepath.Join(root, "gx.lock"))
	if err != nil {
		if os.IsNotExist(err) {
			return lock, nil
		}
		return lock, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return lock, fmt.Errorf("registry: gx.lock: %w", err)
	}
	section, ok := raw["registry"]
	if !ok {
		return lock, nil
	}
	if err := json.Unmarshal(section, &lock); err != nil {
		return lock, fmt.Errorf("registry: gx.lock registry: %w", err)
	}
	if lock.Items == nil {
		lock.Items = map[string]LockItem{}
	}
	return lock, nil
}

// SaveLock merges the registry section into gx.lock without touching other
// sections.
func SaveLock(root string, lock Lock) error {
	path := filepath.Join(root, "gx.lock")
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("registry: gx.lock: %w", err)
		}
	}
	if lock.Items == nil {
		lock.Items = map[string]LockItem{}
	}
	section, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	raw["registry"] = section
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// Index reads the index of the registry: one summary per item
// (REQ-REG-01).
func (in *Installer) Index() (Index, error) {
	data, err := in.read("index.json")
	if err != nil {
		return Index{}, fmt.Errorf("registry: cannot read index.json: %w", err)
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return Index{}, fmt.Errorf("registry: index.json: %w", err)
	}
	return index, nil
}
