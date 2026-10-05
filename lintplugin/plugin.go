// Package lintplugin registers the Gx analyzers with golangci-lint through
// the module plugin system (REQ-TLS-03). The repo's .custom-gcl.yml builds a
// custom binary with it; the docs guide "The dev loop and editors" holds the linter configuration.
package lintplugin

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/alternayte/gx/internal/analyze"
)

func init() {
	register.Plugin("gx", New)
}

// New returns the Gx linter plugin.
func New(settings any) (register.LinterPlugin, error) {
	return plugin{}, nil
}

type plugin struct{}

// BuildAnalyzers returns every Gx analyzer (REQ-TLS-03).
func (plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return analyze.Analyzers(), nil
}

// GetLoadMode returns the type-info load mode the analyzers need.
func (plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
