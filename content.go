package gx

// collection is a typed set of Markdown content files (REQ-CNT-02). The
// compiler reads the declaration for content checks: the directory and the
// components of the Components list (REQ-CNT-03).
type collection[Meta any] struct {
	dir        string
	components []any
}

// Collection declares a content collection rooted at dir, relative to the
// module root.
func Collection[Meta any](dir string) *collection[Meta] {
	return &collection[Meta]{dir: dir}
}

// Components names the components that content files of this collection may
// use (REQ-CNT-03). A component not in this list is a GX8002.
func (c *collection[Meta]) Components(components ...any) *collection[Meta] {
	c.components = append(c.components, components...)
	return c
}

// Dir returns the collection directory as declared.
func (c *collection[Meta]) Dir() string { return c.dir }
