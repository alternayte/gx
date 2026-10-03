package analyze

import (
	"sort"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// Finding is one analyzer result.
type Finding struct {
	File    string
	Line    int
	Col     int
	Code    string
	Message string
}

// Lint runs every Gx analyzer over the packages of dir and returns the
// findings ordered by position (REQ-TLS-03). Generated files are included,
// so //line directives report .gx positions.
func Lint(dir string) ([]Finding, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports |
			packages.NeedDeps | packages.NeedModule,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, pkg := range pkgs {
		for _, analyzer := range Analyzers() {
			var found []Finding
			pass := &analysis.Pass{
				Analyzer:  analyzer,
				Fset:      pkg.Fset,
				Files:     pkg.Syntax,
				Pkg:       pkg.Types,
				TypesInfo: pkg.TypesInfo,
				Report: func(d analysis.Diagnostic) {
					pos := pkg.Fset.Position(d.Pos)
					end := pkg.Fset.Position(d.End)
					if !end.IsValid() || end.Filename != pos.Filename || end.Line != pos.Line {
						end = pos
					}
					found = append(found, Finding{
						File:    pos.Filename,
						Line:    pos.Line,
						Col:     pos.Column,
						Code:    d.Category,
						Message: d.Message,
					})
				},
			}
			if _, err := analyzer.Run(pass); err != nil {
				return nil, err
			}
			out = append(out, found...)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		if out[i].Col != out[j].Col {
			return out[i].Col < out[j].Col
		}
		return out[i].Code < out[j].Code
	})
	return dedupe(out), nil
}

func dedupe(xs []Finding) []Finding {
	out := xs[:0]
	for i, x := range xs {
		if i > 0 && xs[i-1] == x {
			continue
		}
		out = append(out, x)
	}
	return out
}
