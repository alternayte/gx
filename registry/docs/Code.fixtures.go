package docs

import "github.com/alternayte/gx"

// The fixtures show go.mod, a file every app has: the example code names it
// with gx.CodeFile.
var CodeFixtures = gx.Fixtures[CodeProps]{
	"Go":     {Code: gx.Code{File: "go.mod", Lang: "go", Source: "module app\n\ngo 1.25.0\n"}, Title: "go.mod"},
	"Marked": {Code: gx.Code{File: "go.mod", Lang: "go", Source: "module app\n\ngo 1.25.0\n"}, Marks: []int{1}},
}
