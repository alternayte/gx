package docs

import "github.com/alternayte/gx"

var FileTreeFixtures = gx.Fixtures[FileTreeProps]{
	"Tree": {Items: []FileTreeItem{
		{Name: "app", Dir: true, Children: []FileTreeItem{
			{Name: "main.go", Comment: "the app entry"},
			{Name: "theme.css"},
		}},
		{Name: "go.mod"},
	}},
}
