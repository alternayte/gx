// Package appmodel builds the full model of an app (REQ-AI-01): the model
// the compiler reads from the code, plus the icon sets and the registry
// items of gx.lock. `gx describe` and the dev MCP server share it.
package appmodel

import (
	"sort"

	"github.com/alternayte/gx/internal/compiler"
	"github.com/alternayte/gx/internal/icons"
	"github.com/alternayte/gx/internal/registry"
)

// Describe returns the model of the app in dir. A module with a diagnostic
// has no model: the diagnostics come back instead.
func Describe(dir string) (*compiler.AppModel, []compiler.Diagnostic, error) {
	model, diags := compiler.Describe(dir)
	if len(diags) > 0 {
		return nil, diags, nil
	}
	pins, err := icons.Pinned(dir)
	if err != nil {
		return nil, nil, err
	}
	for set, entry := range pins {
		model.Icons = append(model.Icons, compiler.IconSetModel{Set: set, Version: entry.Version})
	}
	sort.Slice(model.Icons, func(i, j int) bool { return model.Icons[i].Set < model.Icons[j].Set })
	lock, err := registry.LoadLock(dir)
	if err != nil {
		return nil, nil, err
	}
	for name, item := range lock.Items {
		files := make([]string, 0, len(item.Files))
		for file := range item.Files {
			files = append(files, file)
		}
		sort.Strings(files)
		model.Registry = append(model.Registry, compiler.RegistryModel{Name: name, Version: item.Version, Files: files})
	}
	sort.Slice(model.Registry, func(i, j int) bool { return model.Registry[i].Name < model.Registry[j].Name })
	return model, nil, nil
}
