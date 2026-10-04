package docs

import "github.com/alternayte/gx"

var FileTreeItemRowFixtures = gx.Fixtures[FileTreeItemRowProps]{
	"File": {Item: FileTreeItem{Name: "main.go", Comment: "entry"}},
	"Dir": {Item: FileTreeItem{Name: "app", Dir: true, Children: []FileTreeItem{
		{Name: "main.go"},
	}}},
}
