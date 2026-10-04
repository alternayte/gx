package docs

// FileTreeItem is one file or directory of docs.FileTree (REQ-CNT-05).
type FileTreeItem struct {
	// Name is the shown base name.
	Name string
	// Dir marks a directory.
	Dir bool
	// Comment adds a short note after the name.
	Comment string
	// Children holds the directory entries.
	Children []FileTreeItem
}
