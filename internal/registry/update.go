package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Change is the state of one file in gx diff or gx update (REQ-REG-03).
type Change struct {
	// Target is the published target path of the file.
	Target string
	// Status is unchanged, local, upstream, merged, conflict,
	// upstream-added, upstream-removed or local-removed.
	Status string
}

// LoadSnapshot reads the base snapshot of one installed version
// (REQ-REG-02).
func LoadSnapshot(root, name, version string) (Item, error) {
	path := filepath.Join(root, filepath.FromSlash(BaseDir), name+"@"+version, "item.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Item{}, fmt.Errorf("registry: item %s@%s has no base snapshot: %w", name, version, err)
	}
	var item Item
	if err := json.Unmarshal(data, &item); err != nil {
		return Item{}, fmt.Errorf("registry: item %s@%s: %w", name, version, err)
	}
	if err := ValidateItem(item); err != nil {
		return Item{}, err
	}
	return item, nil
}

// Diff reports the local, base and upstream state of one installed item
// (REQ-REG-03).
func (in *Installer) Diff(name string) ([]Change, error) {
	_, base, upstream, err := in.prepare(name, "")
	if err != nil {
		return nil, err
	}
	return in.changes(base, upstream)
}

// Update merges the current registry version into one installed item
// (REQ-REG-03). A clean change takes the upstream text, a local-only change
// stays, and a region both sides changed writes conflict markers and is
// reported. The base snapshot and gx.lock move to the upstream version
// either way, so a second update is a no-op.
func (in *Installer) Update(name, version string) ([]Change, bool, error) {
	entry, base, upstream, err := in.prepare(name, version)
	if err != nil {
		return nil, false, err
	}
	dir, err := in.appDir()
	if err != nil {
		return nil, false, err
	}
	changes, err := in.changes(base, upstream)
	if err != nil {
		return nil, false, err
	}
	conflict := false
	for i := range changes {
		change := &changes[i]
		switch change.Status {
		case "unchanged", "local", "local-removed":
			// Keep what the app has.
		case "upstream", "upstream-added":
			content, _ := fileOf(upstream, change.Target)
			if err := in.writeTarget(dir, change.Target, content.Content); err != nil {
				return nil, false, err
			}
		case "upstream-removed":
			if err := in.removeTarget(dir, change.Target); err != nil {
				return nil, false, err
			}
		case "merged", "conflict":
			up, hasUpstream := fileOf(upstream, change.Target)
			if !hasUpstream {
				// The upstream removed a locally edited file. Keep the
				// local file and report it.
				conflict = true
				continue
			}
			local, _, err := in.readLocal(dir, change.Target)
			if err != nil {
				return nil, false, err
			}
			baseFile, _ := fileOf(base, change.Target)
			merged, conflicted := Merge(baseFile.Content, local, up.Content)
			if err := in.writeTarget(dir, change.Target, merged); err != nil {
				return nil, false, err
			}
			if conflicted {
				conflict = true
				change.Status = "conflict"
			} else {
				change.Status = "merged"
			}
		}
	}
	if err := in.store(name, entry.Version, upstream); err != nil {
		return nil, false, err
	}
	return changes, conflict, nil
}

// prepare loads the lock entry, the base snapshot and the upstream item of
// one installed item.
func (in *Installer) prepare(name, version string) (LockItem, Item, Item, error) {
	lock, err := LoadLock(in.Root)
	if err != nil {
		return LockItem{}, Item{}, Item{}, err
	}
	entry, ok := lock.Items[name]
	if !ok {
		return LockItem{}, Item{}, Item{}, fmt.Errorf("registry: item %s is not installed", name)
	}
	base, err := LoadSnapshot(in.Root, name, entry.Version)
	if err != nil {
		return LockItem{}, Item{}, Item{}, err
	}
	upstream, err := in.fetch(name)
	if err != nil {
		return LockItem{}, Item{}, Item{}, err
	}
	if version != "" && upstream.Version != version {
		return LockItem{}, Item{}, Item{}, fmt.Errorf("registry: item %s is version %s, not %s", name, upstream.Version, version)
	}
	return entry, base, upstream, nil
}

// changes compares each target of base and upstream with the app files.
func (in *Installer) changes(base, upstream Item) ([]Change, error) {
	dir, err := in.appDir()
	if err != nil {
		return nil, err
	}
	targets := map[string]bool{}
	for _, f := range base.Files {
		targets[f.Target] = true
	}
	for _, f := range upstream.Files {
		targets[f.Target] = true
	}
	var out []Change
	for target := range targets {
		local, ok, err := in.readLocal(dir, target)
		if err != nil {
			return nil, err
		}
		out = append(out, Change{Target: target, Status: fileState(target, local, ok, base, upstream)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out, nil
}

// fileState classifies one target (REQ-REG-03).
func fileState(target, local string, localOK bool, base, upstream Item) string {
	b, hasBase := fileOf(base, target)
	u, hasUp := fileOf(upstream, target)
	switch {
	case !hasBase && hasUp:
		return "upstream-added"
	case hasBase && !hasUp:
		if !localOK || local == b.Content {
			return "upstream-removed"
		}
		return "conflict"
	case !localOK:
		if b.Content == u.Content {
			return "local-removed"
		}
		return "conflict"
	case local == b.Content && u.Content == b.Content:
		return "unchanged"
	case local == b.Content:
		return "upstream"
	case u.Content == b.Content:
		return "local"
	case local == u.Content:
		return "merged"
	default:
		if _, conflicted := Merge(b.Content, local, u.Content); !conflicted {
			return "merged"
		}
		return "conflict"
	}
}

// fileOf returns one file by target.
func fileOf(item Item, target string) (File, bool) {
	for _, f := range item.Files {
		if f.Target == target {
			return f, true
		}
	}
	return File{}, false
}

// readLocal reads an installed file. The second result reports whether it
// exists.
func (in *Installer) readLocal(dir, target string) (string, bool, error) {
	app, err := in.targetPath(dir, target)
	if err != nil {
		return "", false, err
	}
	data, err := os.ReadFile(filepath.Join(in.Root, filepath.FromSlash(app)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

// writeTarget writes one installed file.
func (in *Installer) writeTarget(dir, target, content string) error {
	app, err := in.targetPath(dir, target)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(in.Root, filepath.FromSlash(app)), content)
}

// removeTarget removes one installed file.
func (in *Installer) removeTarget(dir, target string) error {
	app, err := in.targetPath(dir, target)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(in.Root, filepath.FromSlash(app))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// store moves the base snapshot and gx.lock to the upstream version.
func (in *Installer) store(name, oldVersion string, upstream Item) error {
	if err := in.writeSnapshot(upstream); err != nil {
		return err
	}
	if oldVersion != upstream.Version {
		old := filepath.Join(in.Root, filepath.FromSlash(BaseDir), name+"@"+oldVersion)
		if err := os.RemoveAll(old); err != nil {
			return err
		}
	}
	dir, err := in.appDir()
	if err != nil {
		return err
	}
	lock, err := LoadLock(in.Root)
	if err != nil {
		return err
	}
	entry := LockItem{Version: upstream.Version, Files: map[string]string{}}
	for _, f := range upstream.Files {
		app, err := in.targetPath(dir, f.Target)
		if err != nil {
			return err
		}
		entry.Files[app] = f.SHA256
	}
	lock.Items[name] = entry
	return SaveLock(in.Root, lock)
}
