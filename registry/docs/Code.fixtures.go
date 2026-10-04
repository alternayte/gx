package docs

import "github.com/alternayte/gx"

var CodeFixtures = gx.Fixtures[CodeProps]{
	"Go":     {Code: gx.Code{File: "main.go", Lang: "go", Source: "package main\n\nfunc main() {}\n"}, Title: "main.go"},
	"Marked": {Code: gx.Code{File: "main.go", Lang: "go", Source: "package main\n\nfunc main() {}\n"}, Marks: []int{1}},
}
