package site

import "github.com/alternayte/gx/docs/registry"

// iconExamples returns the first example of every component of an icon
// item, in page order.
func iconExamples(item string) []registry.Example {
	it, ok := registry.Find(item)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []registry.Example
	for _, ex := range it.Examples {
		if seen[ex.Component] {
			continue
		}
		seen[ex.Component] = true
		out = append(out, ex)
	}
	return out
}
